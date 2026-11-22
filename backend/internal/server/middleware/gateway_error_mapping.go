package middleware

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func mapGatewayLocalError(c *gin.Context, status int, code, message string) (int, string) {
	platform, forced := GetForcePlatformFromContext(c)
	if !forced && c.Request != nil {
		platform, _ = service.ResolvedTargetPlatformFromContext(c.Request.Context())
	}
	if platform == "" {
		apiKey, ok := GetAPIKeyFromContext(c)
		if !ok || apiKey == nil {
			apiKey, _ = GetOpsFallbackAPIKey(c)
		}
		if apiKey != nil && apiKey.Group != nil {
			platform = apiKey.Group.Platform
		}
	}
	// 无效 Key 无法确定分组平台，只匹配适用于所有平台的规则。
	return service.ApplyLocalErrorPassthroughRule(c, platform, status, code, message)
}
