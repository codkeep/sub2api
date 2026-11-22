package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type adminWithdrawalRepositoryStub struct {
	service.AffiliateRepository
	id, adminID int64
	input       service.AffiliateWithdrawalReview
	calls       int
}

func (r *adminWithdrawalRepositoryStub) CreateAffiliateWithdrawal(context.Context, int64, service.AffiliateWithdrawalInput, float64, string) (*service.AffiliateWithdrawal, error) {
	panic("unexpected apply")
}
func (r *adminWithdrawalRepositoryStub) ListAffiliateWithdrawals(context.Context, service.AffiliateWithdrawalFilter) ([]service.AffiliateWithdrawal, int64, error) {
	return nil, 0, nil
}
func (r *adminWithdrawalRepositoryStub) ReviewAffiliateWithdrawal(_ context.Context, id, adminID int64, input service.AffiliateWithdrawalReview) (*service.AffiliateWithdrawal, error) {
	r.calls++
	r.id = id
	r.adminID = adminID
	r.input = input
	return &service.AffiliateWithdrawal{ID: id, Status: input.Status, Replayed: true}, nil
}
func TestAffiliateWithdrawalAdminUsesAuthenticatedIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &adminWithdrawalRepositoryStub{}
	h := NewAffiliateHandler(service.NewAffiliateService(repo, nil, nil, nil), nil)
	router := gin.New()
	router.POST("/withdrawals/:id/review", func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 9})
		h.ReviewWithdrawalRequest(c)
	})
	for _, test := range []struct {
		body   string
		status int
	}{
		{`{"status":"paid","processed_by":99}`, http.StatusBadRequest},
		{`{"status":"rejected","admin_note":" "}`, http.StatusBadRequest},
		{`{"status":"paid","payment_reference":"ref","processed_by":99}`, http.StatusOK},
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/withdrawals/11/review", strings.NewReader(test.body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(rec, req)
		require.Equal(t, test.status, rec.Code, rec.Body.String())
		if test.status == http.StatusOK {
			require.Equal(t, "true", rec.Header().Get("X-Idempotency-Replayed"))
		}
	}
	require.Equal(t, 1, repo.calls)
	require.Equal(t, int64(9), repo.adminID)
	require.Equal(t, int64(11), repo.id)
}
