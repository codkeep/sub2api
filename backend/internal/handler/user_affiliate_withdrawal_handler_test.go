package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type userWithdrawalRepositoryStub struct {
	service.AffiliateRepository
	filter service.AffiliateWithdrawalFilter
	calls  int
}

func (r *userWithdrawalRepositoryStub) CreateAffiliateWithdrawal(context.Context, int64, service.AffiliateWithdrawalInput, float64, string) (*service.AffiliateWithdrawal, error) {
	panic("unexpected apply")
}
func (r *userWithdrawalRepositoryStub) ListAffiliateWithdrawals(_ context.Context, filter service.AffiliateWithdrawalFilter) ([]service.AffiliateWithdrawal, int64, error) {
	r.filter = filter
	r.calls++
	return []service.AffiliateWithdrawal{}, 0, nil
}
func (r *userWithdrawalRepositoryStub) ReviewAffiliateWithdrawal(context.Context, int64, int64, service.AffiliateWithdrawalReview) (*service.AffiliateWithdrawal, error) {
	panic("unexpected review")
}

func TestAffiliateWithdrawalUserListCannotSelectAnotherUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &userWithdrawalRepositoryStub{}
	h := &UserHandler{affiliateService: service.NewAffiliateService(repo, nil, nil, nil)}
	router := gin.New()
	router.GET("/withdrawals", func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
		h.ListAffiliateWithdrawals(c)
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/withdrawals?user_id=99&search=other&page=2&page_size=10&status=pending", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(7), repo.filter.UserID)
	require.Empty(t, repo.filter.Search)
	require.Equal(t, 2, repo.filter.Page)
	require.Equal(t, "pending", repo.filter.Status)
}
func TestAffiliateWithdrawalUserRequiresIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &userWithdrawalRepositoryStub{}
	h := &UserHandler{affiliateService: service.NewAffiliateService(repo, nil, nil, nil)}
	for _, handler := range []gin.HandlerFunc{h.ListAffiliateWithdrawals, h.ApplyAffiliateWithdrawal} {
		router := gin.New()
		router.POST("/withdrawals", handler)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/withdrawals", nil))
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	}
	require.Zero(t, repo.calls)
}
