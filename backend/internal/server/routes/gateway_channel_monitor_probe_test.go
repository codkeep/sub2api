package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type channelMonitorProbeRouteKeyRepo struct {
	keyBillingRouteAPIKeyRepo
}

func (*channelMonitorProbeRouteKeyRepo) UpdateLastUsed(context.Context, int64, time.Time) error {
	return nil
}

func TestGatewayChannelMonitorProbeKeepsAuthenticationAndModelAdmission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{RunMode: config.RunModeSimple, Gateway: config.GatewayConfig{MaxBodySize: 1 << 20}}
	group := &service.Group{
		ID: 1, Status: service.StatusActive, Hydrated: true, Platform: service.PlatformOpenAI,
		ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"allowed-model"}},
	}
	key := &service.APIKey{
		ID: 1, Key: "probe-route-test-key", Status: service.StatusActive, GroupID: &group.ID, Group: group,
		User: &service.User{ID: 1, Status: service.StatusActive, Balance: 10},
	}
	keys := service.NewAPIKeyService(&channelMonitorProbeRouteKeyRepo{keyBillingRouteAPIKeyRepo{apiKey: key}}, nil, nil, nil, nil, nil, cfg)
	router := gin.New()
	var marked bool
	var remainingHeader string
	router.Use(func(c *gin.Context) {
		c.Next()
		marked = service.IsChannelMonitorProbe(c.Request.Context())
		remainingHeader = c.GetHeader(service.ChannelMonitorProbeHeader)
	})
	RegisterGatewayRoutes(router,
		&handler.Handlers{Gateway: &handler.GatewayHandler{}, OpenAIGateway: &handler.OpenAIGatewayHandler{}},
		middleware.NewAPIKeyAuthMiddleware(keys, nil, cfg), keys, nil, nil, nil, nil, cfg)
	for _, path := range []string{
		"/v1/messages", "/v1/messages/count_tokens", "/v1/responses", "/v1/responses/compact", "/v1/chat/completions",
		"/responses", "/responses/compact", "/chat/completions", "/messages/count_tokens", "/backend-api/codex/responses",
		"/antigravity/v1/messages", "/antigravity/v1/messages/count_tokens",
		"/v1beta/models/blocked-model:generateContent", "/antigravity/v1beta/models/blocked-model:generateContent",
	} {
		t.Run(path, func(t *testing.T) {
			for _, tc := range []struct {
				name        string
				probe, auth bool
			}{
				{"probe", true, true}, {"normal after probe", false, true}, {"unauthenticated probe", true, false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"model":"blocked-model","messages":[{"role":"user","content":"hi"}]}`))
					req.Header.Set("Content-Type", "application/json")
					if tc.auth {
						req.Header.Set("Authorization", "Bearer "+key.Key)
					}
					if tc.probe {
						req.Header.Set(service.ChannelMonitorProbeHeader, service.ChannelMonitorProbeHeaderValue)
					}
					w := httptest.NewRecorder()
					router.ServeHTTP(w, req)
					if !tc.auth {
						require.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
						require.False(t, marked)
						return
					}
					require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
					require.Contains(t, w.Body.String(), "blocked-model", "probes must still obey the model allowlist")
					require.Equal(t, tc.probe, marked, "all gateway aliases must carry the request-local probe context")
					require.Empty(t, remainingHeader)
				})
			}
		})
	}
}
