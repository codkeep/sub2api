package middleware

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ChannelMonitorProbe keeps a monitor's challenge separate from configured
// conversation Skills. Consume the header so it is not relayed to providers.
func ChannelMonitorProbe() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader(service.ChannelMonitorProbeHeader) == service.ChannelMonitorProbeHeaderValue {
			c.Request = c.Request.WithContext(service.WithChannelMonitorProbe(c.Request.Context()))
		}
		c.Request.Header.Del(service.ChannelMonitorProbeHeader)
		c.Next()
	}
}
