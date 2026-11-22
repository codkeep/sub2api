package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/model"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type gatewayErrorMappingRuleRepo struct {
	service.ErrorPassthroughRepository
	rules []*model.ErrorPassthroughRule
}

func (r *gatewayErrorMappingRuleRepo) List(context.Context) ([]*model.ErrorPassthroughRule, error) {
	return r.rules, nil
}

func newGatewayErrorMappingRouter(rule *model.ErrorPassthroughRule, key *service.APIKey, subscriptions *service.SubscriptionService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{Gateway: config.GatewayConfig{MaxBodySize: 1 << 20}}
	apiKeys := service.NewAPIKeyService(&keyBillingRouteAPIKeyRepo{apiKey: key}, nil, nil, nil, nil, nil, cfg)
	rules := service.NewErrorPassthroughService(&gatewayErrorMappingRuleRepo{rules: []*model.ErrorPassthroughRule{rule}}, nil)
	gateway := handler.NewGatewayHandler(nil, nil, nil, nil, nil, nil, nil, nil, apiKeys, nil, rules, nil, nil, cfg, nil)
	router := gin.New()
	RegisterGatewayRoutes(router, &handler.Handlers{Gateway: gateway, OpenAIGateway: &handler.OpenAIGatewayHandler{}},
		servermiddleware.NewAPIKeyAuthMiddleware(apiKeys, subscriptions, cfg), apiKeys, subscriptions, nil, nil, nil, cfg)
	return router
}

func TestGatewayErrorMappingBeforeAuthentication(t *testing.T) {
	message := "无效的API密钥"
	rule := &model.ErrorPassthroughRule{
		ID: 1, Name: "invalid-key", Enabled: true,
		ErrorCodes: []int{401}, Keywords: []string{"INVALID_API_KEY"}, MatchMode: model.MatchModeAll,
		PassthroughCode: true, CustomMessage: &message, SkipMonitoring: true,
	}
	router := newGatewayErrorMappingRouter(rule, nil, nil)
	for _, tc := range []struct {
		method, path string
		google       bool
	}{
		{http.MethodPost, "/v1/responses", false},
		{http.MethodPost, "/responses", false},
		{http.MethodPost, "/backend-api/codex/responses", false},
		{http.MethodPost, "/v1/messages", false},
		{http.MethodPost, "/antigravity/v1/messages", false},
		{http.MethodGet, "/antigravity/models", false},
		{http.MethodPost, "/v1beta/models/gemini-2.5-flash:generateContent", true},
		{http.MethodPost, "/antigravity/v1beta/models/gemini-2.5-flash:generateContent", true},
	} {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("Authorization", "Bearer invalid-key")
			req.Header.Set("Accept", "text/event-stream")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			require.Equal(t, http.StatusUnauthorized, recorder.Code)
			var payload map[string]any
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
			if tc.google {
				errBody := payload["error"].(map[string]any)
				require.Equal(t, message, errBody["message"])
				require.Equal(t, float64(401), errBody["code"])
				require.Equal(t, "UNAUTHENTICATED", errBody["status"])
			} else {
				require.Equal(t, message, payload["message"])
				require.Equal(t, "INVALID_API_KEY", payload["code"])
			}
		})
	}
}

func TestGatewayErrorMappingUsesGroupBeforeAuthenticationCompletes(t *testing.T) {
	message := "密钥已过期，请更换密钥"
	group := &service.Group{ID: 1, Status: service.StatusActive, Hydrated: true, Platform: service.PlatformOpenAI}
	key := &service.APIKey{
		ID: 1, Key: "expired-key", Status: service.StatusAPIKeyExpired, GroupID: &group.ID, Group: group,
		User: &service.User{ID: 1, Status: service.StatusActive, Balance: 10},
	}
	for _, tc := range []struct {
		name, path, platform string
		matched              bool
	}{
		{"group platform", "/v1/responses", service.PlatformOpenAI, true},
		{"other platform excluded", "/v1/responses", service.PlatformAnthropic, false},
		{"forced platform", "/antigravity/v1/messages", service.PlatformAntigravity, true},
		{"Google auth code", "/v1beta/models/gemini-2.5-flash:generateContent", service.PlatformOpenAI, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rule := &model.ErrorPassthroughRule{
				ID: 1, Name: "expired-key", Enabled: true,
				ErrorCodes: []int{403}, Keywords: []string{"API_KEY_EXPIRED"}, MatchMode: model.MatchModeAll,
				Platforms: []string{tc.platform}, PassthroughCode: true, CustomMessage: &message,
			}
			router := newGatewayErrorMappingRouter(rule, key, nil)
			req := httptest.NewRequest(http.MethodPost, tc.path, nil)
			req.Header.Set("Authorization", "Bearer expired-key")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			require.Equal(t, http.StatusForbidden, recorder.Code)
			if tc.matched {
				require.Contains(t, recorder.Body.String(), message)
			} else {
				require.NotContains(t, recorder.Body.String(), message)
				require.Contains(t, recorder.Body.String(), "API_KEY_EXPIRED")
			}
		})
	}
}

type gatewayErrorMappingSubscriptionRepo struct {
	service.UserSubscriptionRepository
	subscription *service.UserSubscription
}

func (r *gatewayErrorMappingSubscriptionRepo) GetActiveByUserIDAndGroupID(context.Context, int64, int64) (*service.UserSubscription, error) {
	clone := *r.subscription
	return &clone, nil
}

func TestGatewayErrorMappingSubscriptionQuotaTemplates(t *testing.T) {
	for _, period := range []string{"daily", "weekly", "monthly"} {
		t.Run(period, func(t *testing.T) {
			limit := 1.0
			group := &service.Group{
				ID: 1, Status: service.StatusActive, Hydrated: true, Platform: service.PlatformOpenAI,
				SubscriptionType: service.SubscriptionTypeSubscription,
			}
			now := time.Now()
			subscription := &service.UserSubscription{
				ID: 1, UserID: 1, GroupID: group.ID, Status: service.SubscriptionStatusActive,
				ExpiresAt: now.Add(24 * time.Hour), DailyWindowStart: &now, WeeklyWindowStart: &now, MonthlyWindowStart: &now,
			}
			switch period {
			case "daily":
				group.DailyLimitUSD, subscription.DailyUsageUSD = &limit, 10
			case "weekly":
				group.WeeklyLimitUSD, subscription.WeeklyUsageUSD = &limit, 10
			case "monthly":
				group.MonthlyLimitUSD, subscription.MonthlyUsageUSD = &limit, 10
			}
			key := &service.APIKey{
				ID: 1, UserID: 1, Key: "quota-key", Status: service.StatusActive, GroupID: &group.ID, Group: group,
				User: &service.User{ID: 1, Status: service.StatusActive, Balance: 10},
			}
			message := period + " 自定义限额提示"
			rule := &model.ErrorPassthroughRule{
				ID: 1, Name: period, Enabled: true, ErrorCodes: []int{429}, MatchMode: model.MatchModeAll,
				Keywords:        []string{strings.ToUpper(period) + "_LIMIT_EXCEEDED", period + " usage limit exceeded"},
				PassthroughCode: true, CustomMessage: &message,
			}
			subscriptions := service.NewSubscriptionService(nil, &gatewayErrorMappingSubscriptionRepo{subscription: subscription}, nil, nil, nil)
			t.Cleanup(subscriptions.Stop)
			router := newGatewayErrorMappingRouter(rule, key, subscriptions)
			for _, path := range []string{"/v1/responses", "/v1beta/models/gemini-2.5-flash:generateContent"} {
				t.Run(path, func(t *testing.T) {
					req := httptest.NewRequest(http.MethodPost, path, nil)
					req.Header.Set("Authorization", "Bearer quota-key")
					recorder := httptest.NewRecorder()
					router.ServeHTTP(recorder, req)
					require.Equal(t, http.StatusTooManyRequests, recorder.Code)
					var payload map[string]any
					require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
					if path == "/v1/responses" {
						require.Equal(t, message, payload["message"])
						require.Equal(t, "USAGE_LIMIT_EXCEEDED", payload["code"])
					} else {
						errBody := payload["error"].(map[string]any)
						require.Equal(t, message, errBody["message"])
						require.Equal(t, float64(429), errBody["code"])
						require.Equal(t, "RESOURCE_EXHAUSTED", errBody["status"])
					}
					require.NotContains(t, recorder.Body.String(), "usage limit exceeded")
				})
			}
		})
	}
}
