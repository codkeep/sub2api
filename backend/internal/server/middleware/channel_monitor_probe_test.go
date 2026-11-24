package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestChannelMonitorProbeRequestIsolation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ChannelMonitorProbe())
	body := `{"messages":[{"role":"user","content":"channel health-check: Calculate and respond with ONLY the number"}]}`
	var gotProbe bool
	router.POST("/test", func(c *gin.Context) {
		gotProbe = service.IsChannelMonitorProbe(c.Request.Context())
		require.Empty(t, c.GetHeader(service.ChannelMonitorProbeHeader), "internal header must be consumed before forwarding")
		actual, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		require.Equal(t, body, string(actual))
		c.Status(http.StatusOK)
	})
	// Reuse the same router so a probe followed by normal traffic catches any
	// state accidentally kept on a pooled Gin context or shared service.
	for _, tc := range []struct {
		name, value string
		probe       bool
	}{
		{"probe", service.ChannelMonitorProbeHeaderValue, true},
		{"normal prompt mentioning monitoring", "", false},
		{"unrecognized value", "false", false},
		{"probe again", service.ChannelMonitorProbeHeaderValue, true},
		{"normal again", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
			if tc.value != "" {
				req.Header.Set(service.ChannelMonitorProbeHeader, tc.value)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, tc.probe, gotProbe)
		})
	}
}
