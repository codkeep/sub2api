package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
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
	GroupID      int64              `json:"group_id"`
	GroupName    string             `json:"group_name"`
	PricingUnit  string             `json:"pricing_unit"`
	ModelPricing []userModelPricing `json:"model_pricing"`
}

// ListPricing 返回全部启用渠道关联分组中的模型定价，供所有登录用户查看。
// GET /api/v1/channels/pricing
func (h *AvailableChannelHandler) ListPricing(c *gin.Context) {
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
		if len(pricing) == 0 {
			continue
		}
		for _, group := range ch.Groups {
			out = append(out, userChannelPricing{
				Name:         ch.Name,
				GroupID:      group.ID,
				GroupName:    group.I18n.LocalizeName(c.GetHeader("Accept-Language"), group.Name),
				PricingUnit:  group.PricingUnit,
				ModelPricing: pricing,
			})
		}
	}
	response.Success(c, out)
}
