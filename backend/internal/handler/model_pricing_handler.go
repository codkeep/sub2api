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
	GroupID      int64              `json:"group_id"`
	GroupName    string             `json:"group_name"`
	PricingUnit  string             `json:"pricing_unit"`
	ModelPricing []userModelPricing `json:"model_pricing"`
}

// ListPricing 只返回当前用户有效订阅分组关联渠道中已配置的模型定价。
// GET /api/v1/channels/pricing
func (h *AvailableChannelHandler) ListPricing(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	groups, err := h.apiKeyService.GetAvailableGroups(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	subscriptionGroups := make(map[int64]service.Group)
	for i := range groups {
		if groups[i].IsSubscriptionType() {
			subscriptionGroups[groups[i].ID] = groups[i]
		}
	}
	if len(subscriptionGroups) == 0 {
		response.Success(c, []userChannelPricing{})
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
		if len(pricing) == 0 {
			continue
		}
		for _, groupID := range ch.GroupIDs {
			group, ok := subscriptionGroups[groupID]
			if !ok {
				continue
			}
			out = append(out, userChannelPricing{
				Name:         ch.Name,
				GroupID:      group.ID,
				GroupName:    group.Name,
				PricingUnit:  group.PricingUnit,
				ModelPricing: pricing,
			})
		}
	}
	response.Success(c, out)
}
