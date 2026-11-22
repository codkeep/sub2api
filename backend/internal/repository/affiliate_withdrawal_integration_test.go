//go:build integration

package repository

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func committedWithdrawalFixture(t *testing.T, label string, quota, frozen float64) (*affiliateRepository, *service.User) {
	t.Helper()
	user := mustCreateCommittedAffiliateWithdrawUser(t, label, quota, frozen, quota+frozen)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM affiliate_withdrawal_requests WHERE user_id=$1", user.ID)
	})
	return &affiliateRepository{client: testEntClient(t)}, user
}

func TestAffiliateWithdrawalIntegrationConcurrentApplyAndRefundOnce(t *testing.T) {
	ctx := context.Background()
	repo, user := committedWithdrawalFixture(t, "request-concurrent", 200, 0)
	input := withdrawalRepoInput()
	operation := newAffiliateWithdrawOperationID("request")
	const workers = 8
	results := make([]*service.AffiliateWithdrawal, workers)
	errs := make([]error, workers)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range workers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i], errs[i] = repo.CreateAffiliateWithdrawal(ctx, user.ID, input, 100, operation)
		}(i)
	}
	close(start)
	wg.Wait()
	applied := 0
	for i := range workers {
		require.NoError(t, errs[i])
		require.Equal(t, results[0].ID, results[i].ID)
		if !results[i].Replayed {
			applied++
		}
	}
	require.Equal(t, 1, applied)
	require.Equal(t, 50.0, querySingleFloat(t, ctx, repo.client, "SELECT aff_quota::double precision FROM user_affiliates WHERE user_id=$1", user.ID))
	require.Equal(t, 1, querySingleInt(t, ctx, repo.client, "SELECT COUNT(*) FROM user_affiliate_ledger WHERE user_id=$1 AND action='withdraw_hold'", user.ID))
	replay, err := repo.CreateAffiliateWithdrawal(ctx, user.ID, input, 300, operation)
	require.NoError(t, err)
	require.True(t, replay.Replayed)
	input.PayoutAccount = "changed"
	_, err = repo.CreateAffiliateWithdrawal(ctx, user.ID, input, 100, operation)
	require.ErrorIs(t, err, service.ErrIdempotencyKeyConflict)

	_, err = repo.ReviewAffiliateWithdrawal(ctx, results[0].ID, user.ID, service.AffiliateWithdrawalReview{Status: "processing"})
	require.NoError(t, err)
	review := service.AffiliateWithdrawalReview{Status: "rejected", AdminNote: "invalid account"}
	start = make(chan struct{})
	for i := range workers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i], errs[i] = repo.ReviewAffiliateWithdrawal(ctx, replay.ID, user.ID, review)
		}(i)
	}
	close(start)
	wg.Wait()
	applied = 0
	for i := range workers {
		require.NoError(t, errs[i])
		if !results[i].Replayed {
			applied++
		}
	}
	require.Equal(t, 1, applied)
	require.Equal(t, 200.0, querySingleFloat(t, ctx, repo.client, "SELECT aff_quota::double precision FROM user_affiliates WHERE user_id=$1", user.ID))
	require.Equal(t, 1, querySingleInt(t, ctx, repo.client, "SELECT COUNT(*) FROM user_affiliate_ledger WHERE user_id=$1 AND action='withdraw_refund'", user.ID))
}

func TestAffiliateWithdrawalIntegrationDifferentRequestsCannotOverspend(t *testing.T) {
	ctx := context.Background()
	repo, user := committedWithdrawalFixture(t, "request-overspend", 200, 0)
	input := withdrawalRepoInput()
	const workers = 4
	var wg sync.WaitGroup
	errs := make([]error, workers)
	start := make(chan struct{})
	for i := range workers {
		operation := newAffiliateWithdrawOperationID("different")
		wg.Add(1)
		go func(i int, operation string) {
			defer wg.Done()
			<-start
			_, errs[i] = repo.CreateAffiliateWithdrawal(ctx, user.ID, input, 100, operation)
		}(i, operation)
	}
	close(start)
	wg.Wait()
	success := 0
	for _, err := range errs {
		if err == nil {
			success++
		} else {
			require.ErrorIs(t, err, service.ErrAffiliateQuotaInsufficient)
		}
	}
	require.Equal(t, 1, success)
	require.Equal(t, 50.0, querySingleFloat(t, ctx, repo.client, "SELECT aff_quota::double precision FROM user_affiliates WHERE user_id=$1", user.ID))
	require.Equal(t, 1, querySingleInt(t, ctx, repo.client, "SELECT COUNT(*) FROM affiliate_withdrawal_requests WHERE user_id=$1", user.ID))
}

func TestAffiliateWithdrawalIntegrationFrozenQuotaAndPayment(t *testing.T) {
	ctx := context.Background()
	repo, user := committedWithdrawalFixture(t, "request-frozen", 50, 200)
	_, err := integrationDB.ExecContext(ctx, "INSERT INTO user_affiliate_ledger (user_id,action,amount,frozen_until) VALUES ($1,'accrue',200,NOW()+INTERVAL '1 hour')", user.ID)
	require.NoError(t, err)
	input := withdrawalRepoInput()
	operation := newAffiliateWithdrawOperationID("frozen")
	_, err = repo.CreateAffiliateWithdrawal(ctx, user.ID, input, 0, operation)
	require.ErrorIs(t, err, service.ErrAffiliateQuotaInsufficient)
	require.Equal(t, 0, querySingleInt(t, ctx, repo.client, "SELECT COUNT(*) FROM affiliate_withdrawal_requests WHERE user_id=$1", user.ID))
	input.Amount = 50
	request, err := repo.CreateAffiliateWithdrawal(ctx, user.ID, input, 0, operation)
	require.NoError(t, err)
	_, err = repo.ReviewAffiliateWithdrawal(ctx, request.ID, user.ID, service.AffiliateWithdrawalReview{Status: "processing"})
	require.NoError(t, err)
	paid, err := repo.ReviewAffiliateWithdrawal(ctx, request.ID, user.ID, service.AffiliateWithdrawalReview{Status: "paid", PaymentReference: "test-ref"})
	require.NoError(t, err)
	require.Equal(t, "paid", paid.Status)
	_, err = repo.ReviewAffiliateWithdrawal(ctx, request.ID, user.ID, service.AffiliateWithdrawalReview{Status: "rejected", AdminNote: "too late"})
	require.ErrorIs(t, err, service.ErrAffiliateWithdrawalState)
	require.Equal(t, 0.0, querySingleFloat(t, ctx, repo.client, "SELECT aff_quota::double precision FROM user_affiliates WHERE user_id=$1", user.ID))
	require.Equal(t, 200.0, querySingleFloat(t, ctx, repo.client, "SELECT aff_frozen_quota::double precision FROM user_affiliates WHERE user_id=$1", user.ID))
	records, total, err := repo.ListAffiliateWithdrawals(ctx, service.AffiliateWithdrawalFilter{UserID: user.ID, Status: "paid", Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, records, 1)
	require.Equal(t, "test-ref", records[0].PaymentReference)
	records, total, err = repo.ListAffiliateWithdrawals(ctx, service.AffiliateWithdrawalFilter{UserID: user.ID + 100000, Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, records)
}

func TestAffiliateWithdrawalIntegrationConcurrentTransferConservesQuota(t *testing.T) {
	ctx := context.Background()
	repo, user := committedWithdrawalFixture(t, "request-transfer", 200, 0)
	start := make(chan struct{})
	var request *service.AffiliateWithdrawal
	var applyErr, transferErr error
	var transferred float64
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		request, applyErr = repo.CreateAffiliateWithdrawal(ctx, user.ID, withdrawalRepoInput(), 100, newAffiliateWithdrawOperationID("transfer"))
	}()
	go func() {
		defer wg.Done()
		<-start
		transferred, _, transferErr = repo.TransferQuotaToBalance(ctx, user.ID)
	}()
	close(start)
	wg.Wait()
	require.NoError(t, transferErr)
	held := 0.0
	if applyErr == nil {
		held = request.Amount
	} else {
		require.True(t, errors.Is(applyErr, service.ErrAffiliateQuotaInsufficient), "%v", applyErr)
	}
	require.Equal(t, 200.0, held+transferred)
	require.Equal(t, 0.0, querySingleFloat(t, ctx, repo.client, "SELECT aff_quota::double precision FROM user_affiliates WHERE user_id=$1", user.ID))
}
