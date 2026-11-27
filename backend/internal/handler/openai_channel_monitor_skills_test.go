package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type monitorResponsesSkillSettings struct {
	service.SettingRepository
	skillReads int
}

func (s *monitorResponsesSkillSettings) GetValue(_ context.Context, key string) (string, error) {
	switch key {
	case service.SettingKeyGlobalModelSkillRules:
		s.skillReads++
		return `{"default_skill_ids":["global"]}`, nil
	case service.SettingKeyModelSkills:
		s.skillReads++
		return `[{"id":"global","name":"global","content":"GLOBAL_SKILL_CONTENT"},{"id":"group","name":"group","content":"GROUP_SKILL_CONTENT"}]`, nil
	default:
		return "", service.ErrSettingNotFound
	}
}

type monitorResponsesUpstream struct {
	service.HTTPUpstream
	body   []byte
	header http.Header
}

func (u *monitorResponsesUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	var err error
	u.body, err = io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	u.header = req.Header.Clone()
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"resp_probe","object":"response","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"8"}]}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`)),
	}, nil
}

func TestOpenAIResponsesChannelMonitorSkillsAtUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, passthrough := range []bool{false, true} {
		name := "transformed"
		if passthrough {
			name = "passthrough"
		}
		t.Run(name, func(t *testing.T) {
			cfg := &config.Config{RunMode: config.RunModeSimple}
			cfg.Default.RateMultiplier = 1
			accountRepo := &openAIWSFailoverHandlerAccountRepoStub{accounts: []service.Account{{
				ID: 9911, Name: "probe-account", Platform: service.PlatformOpenAI,
				Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true,
				Credentials: map[string]any{"api_key": "sk-upstream", "base_url": "https://api.example.test"},
				Extra:       map[string]any{"openai_passthrough": passthrough},
			}}}
			settingsRepo := &monitorResponsesSkillSettings{}
			settings := service.NewSettingService(settingsRepo, cfg)
			upstream := &monitorResponsesUpstream{}
			billingCache := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
			t.Cleanup(billingCache.Stop)
			gateway := service.NewOpenAIGatewayService(
				accountRepo, nil, nil, nil, nil, nil, nil, cfg, nil, nil,
				service.NewBillingService(cfg, nil), service.NewRateLimitService(accountRepo, nil, cfg, nil, nil),
				billingCache, upstream, &service.DeferredService{}, nil, nil, nil, nil, nil, settings, nil,
			)
			h := NewOpenAIGatewayHandler(gateway, service.NewConcurrencyService(nil), billingCache,
				service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, cfg), nil, nil, nil, nil, cfg)
			group := &service.Group{ID: 42, Platform: service.PlatformOpenAI, Status: service.StatusActive, DefaultSkillIDs: []string{"group"}}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{
					ID: 1, GroupID: &group.ID, Group: group, User: &service.User{ID: 2, Status: service.StatusActive},
				})
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 2})
				c.Next()
			})
			router.Use(middleware.ChannelMonitorProbe())
			router.POST("/v1/responses", h.Responses)

			// Alternate probe and normal traffic on the same router to exercise
			// pooled contexts as well as the final transformed upstream payload.
			for _, probe := range []bool{true, false, true} {
				body := `{"model":"gpt-6.1-sol","instructions":"Answer the arithmetic challenge briefly.","input":"3 + 5 = ?","stream":false,"max_output_tokens":50}`
				req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				if probe {
					req.Header.Set(service.ChannelMonitorProbeHeader, service.ChannelMonitorProbeHeaderValue)
				}
				upstream.body = nil
				readsBefore := settingsRepo.skillReads
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)
				require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
				require.NotEmpty(t, upstream.body)
				require.False(t, gjson.GetBytes(upstream.body, "reasoning").Exists(), "Skill isolation must not introduce reasoning parameters")
				require.Equal(t, "3 + 5 = ?", gjson.GetBytes(upstream.body, "input").String())
				require.Empty(t, upstream.header.Get(service.ChannelMonitorProbeHeader))
				if probe {
					require.Equal(t, readsBefore, settingsRepo.skillReads, "probes must not query Skill settings")
					require.Equal(t, "Answer the arithmetic challenge briefly.", gjson.GetBytes(upstream.body, "instructions").String())
					require.NotContains(t, string(upstream.body), "SKILL_CONTENT")
				} else {
					require.Greater(t, settingsRepo.skillReads, readsBefore)
					require.Contains(t, string(upstream.body), "GLOBAL_SKILL_CONTENT")
					require.Contains(t, string(upstream.body), "GROUP_SKILL_CONTENT")
				}
			}
		})
	}
}
