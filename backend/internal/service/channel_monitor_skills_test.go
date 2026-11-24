//go:build unit

package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChannelMonitorProbeHeadersAcrossProvidersAndBodyModes(t *testing.T) {
	swapMonitorHTTPClient(t)
	for _, provider := range []string{MonitorProviderOpenAI, MonitorProviderAnthropic, MonitorProviderGemini,
		MonitorProviderGrok, MonitorProviderKimi, MonitorProviderZhipu, MonitorProviderDeepseek, MonitorProviderMiniMax} {
		modes := []string{MonitorAPIModeChatCompletions}
		if provider == MonitorProviderOpenAI {
			modes = append(modes, MonitorAPIModeResponses)
		}
		for _, apiMode := range modes {
			for _, bodyMode := range []string{MonitorBodyOverrideModeOff, MonitorBodyOverrideModeMerge, MonitorBodyOverrideModeReplace} {
				t.Run(provider+"/"+apiMode+"/"+bodyMode, func(t *testing.T) {
					adapter, _, ok := providerAdapterFor(provider, apiMode)
					require.True(t, ok)
					challenge := generateChallenge()
					defaultBody, err := adapter.buildBody("test-model", challenge.Prompt)
					require.NoError(t, err)
					var override map[string]any
					require.NoError(t, json.Unmarshal(defaultBody, &override))
					if bodyMode == MonitorBodyOverrideModeMerge {
						override = map[string]any{"temperature": 0}
					}
					opts := &CheckOptions{
						APIMode: apiMode, BodyOverrideMode: bodyMode, BodyOverride: override,
						ExtraHeaders: map[string]string{
							strings.ToLower(ChannelMonitorProbeHeader): "false", "X-Custom": "retained",
						},
					}
					expectedBody, err := buildRequestBody(adapter, provider, apiMode, "test-model", challenge.Prompt, opts)
					require.NoError(t, err)
					var receivedHeader http.Header
					var receivedBody []byte
					srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						receivedHeader = r.Header.Clone()
						receivedBody, _ = io.ReadAll(r.Body)
						w.Header().Set("Content-Type", "application/json")
						_, _ = w.Write([]byte(`{}`))
					}))
					defer srv.Close()
					_, _, status, err := callProvider(context.Background(), provider, srv.URL, "test-key", "test-model", challenge.Prompt, opts)
					require.NoError(t, err)
					require.Equal(t, http.StatusOK, status)
					require.Equal(t, []string{ChannelMonitorProbeHeaderValue}, receivedHeader.Values(ChannelMonitorProbeHeader))
					require.Equal(t, "retained", receivedHeader.Get("X-Custom"))
					require.Equal(t, expectedBody, receivedBody, "the marker must not rewrite the challenge or override body")
					require.Equal(t, "false", opts.ExtraHeaders[strings.ToLower(ChannelMonitorProbeHeader)], "shared template headers must not be mutated")
				})
			}
		}
	}
}
