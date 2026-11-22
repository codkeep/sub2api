package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *AffiliateHandler) GetWithdrawalPolicy(c *gin.Context) {
	policy, err := h.affiliateService.GetWithdrawalPolicy(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, policy)
}

func (h *AffiliateHandler) SetWithdrawalPolicy(c *gin.Context) {
	var req struct {
		MinimumAmount *float64 `json:"minimum_amount"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.MinimumAmount == nil {
		response.BadRequest(c, "minimum_amount is required")
		return
	}
	policy, err := h.affiliateService.SetWithdrawalPolicy(c.Request.Context(), *req.MinimumAmount)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, policy)
}

func (h *AffiliateHandler) ListWithdrawalRequests(c *gin.Context) {
	page, size := response.ParsePagination(c)
	if size > 100 {
		size = 100
	}
	items, total, err := h.affiliateService.ListWithdrawals(c.Request.Context(), service.AffiliateWithdrawalFilter{
		Status: c.Query("status"), Search: c.Query("search"), Page: page, PageSize: size,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, size)
}

func (h *AffiliateHandler) ReviewWithdrawalRequest(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Admin not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid withdrawal id")
		return
	}
	var input service.AffiliateWithdrawalReview
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "Invalid withdrawal review")
		return
	}
	result, err := h.affiliateService.ReviewWithdrawal(c.Request.Context(), id, subject.UserID, input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if result.Replayed {
		c.Header("X-Idempotency-Replayed", "true")
	}
	response.Success(c, result)
}
