package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// 使用字段白名单，避免将渠道映射、账号统计定价及其他管理配置公开。
type userModelPricing struct {
	*userSupportedModelPricing
	Platform       string                      `json:"platform"`
	Models         []string                    `json:"models"`
	FastMultiplier *float64                    `json:"fast_multiplier"`
	FlexMultiplier *float64                    `json:"flex_multiplier"`
	TimePricing    *service.ChannelTimePricing `json:"time_pricing"`
}

type userChannelPricing struct {
	Name         string             `json:"name"`
	ModelPricing []userModelPricing `json:"model_pricing"`
}

// ListPricing 允许所有登录账号查看基础价目，不依赖可用渠道开关、分组授权或订阅。
// GET /api/v1/channels/pricing
func (h *AvailableChannelHandler) ListPricing(c *gin.Context) {
	if _, ok := middleware.GetAuthSubjectFromContext(c); !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	channels, err := h.channelService.ListConfiguredPricing(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]userChannelPricing, 0, len(channels))
	for _, ch := range channels {
		pricing := make([]userModelPricing, 0, len(ch.ModelPricing))
		for i := range ch.ModelPricing {
			p := &ch.ModelPricing[i]
			if len(p.Models) == 0 {
				continue
			}
			pricing = append(pricing, userModelPricing{
				userSupportedModelPricing: toUserPricing(p),
				Platform:                  p.Platform,
				Models:                    p.Models,
				FastMultiplier:            p.FastMultiplier,
				FlexMultiplier:            p.FlexMultiplier,
				TimePricing:               p.TimePricing,
			})
		}
		if len(pricing) > 0 {
			out = append(out, userChannelPricing{Name: ch.Name, ModelPricing: pricing})
		}
	}
	response.Success(c, out)
}
