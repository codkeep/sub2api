package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *UserHandler) GetAffiliateWithdrawalPolicy(c *gin.Context) {
	policy, err := h.affiliateService.GetWithdrawalPolicy(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, policy)
}

func (h *UserHandler) ApplyAffiliateWithdrawal(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var input service.AffiliateWithdrawalInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "Invalid withdrawal request")
		return
	}
	result, err := h.affiliateService.ApplyWithdrawal(c.Request.Context(), subject.UserID, input, c.GetHeader("Idempotency-Key"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if result.Replayed {
		c.Header("X-Idempotency-Replayed", "true")
	}
	response.Success(c, result)
}

func (h *UserHandler) ListAffiliateWithdrawals(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	page, size := response.ParsePagination(c)
	if size > 100 {
		size = 100
	}
	items, total, err := h.affiliateService.ListWithdrawals(c.Request.Context(), service.AffiliateWithdrawalFilter{
		UserID: subject.UserID, Status: c.Query("status"), Page: page, PageSize: size,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, size)
}
