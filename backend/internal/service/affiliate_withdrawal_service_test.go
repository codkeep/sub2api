package service

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type withdrawalSettingsStub struct {
	SettingRepository
	values map[string]string
	err    error
}

func (s *withdrawalSettingsStub) GetValue(_ context.Context, key string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	if value, ok := s.values[key]; ok {
		return value, nil
	}
	return "", ErrSettingNotFound
}
func (s *withdrawalSettingsStub) Set(_ context.Context, key, value string) error {
	if s.err != nil {
		return s.err
	}
	s.values[key] = value
	return nil
}

type withdrawalRequestRepoStub struct {
	AffiliateRepository
	input       AffiliateWithdrawalInput
	minimum     float64
	operationID string
	userID      int64
	calls       int
	filter      AffiliateWithdrawalFilter
	review      AffiliateWithdrawalReview
	adminID     int64
}

func (r *withdrawalRequestRepoStub) CreateAffiliateWithdrawal(_ context.Context, userID int64, input AffiliateWithdrawalInput, minimum float64, operationID string) (*AffiliateWithdrawal, error) {
	r.calls++
	r.userID = userID
	r.input = input
	r.minimum = minimum
	r.operationID = operationID
	return &AffiliateWithdrawal{ID: 1, UserID: userID, Amount: input.Amount}, nil
}
func (r *withdrawalRequestRepoStub) ListAffiliateWithdrawals(_ context.Context, filter AffiliateWithdrawalFilter) ([]AffiliateWithdrawal, int64, error) {
	r.filter = filter
	return []AffiliateWithdrawal{}, 0, nil
}
func (r *withdrawalRequestRepoStub) ReviewAffiliateWithdrawal(_ context.Context, id, adminID int64, input AffiliateWithdrawalReview) (*AffiliateWithdrawal, error) {
	r.calls++
	r.review = input
	r.adminID = adminID
	return &AffiliateWithdrawal{ID: id, Status: input.Status}, nil
}
func newWithdrawalTestService() (*AffiliateService, *withdrawalRequestRepoStub, *withdrawalSettingsStub) {
	settings := &withdrawalSettingsStub{values: map[string]string{SettingKeyAffiliateEnabled: "true"}}
	repo := &withdrawalRequestRepoStub{}
	return NewAffiliateService(repo, NewSettingService(settings, nil), nil, nil), repo, settings
}
func withdrawalTestInput() AffiliateWithdrawalInput {
	return AffiliateWithdrawalInput{Amount: 150, PayoutMethod: " Bank ", PayoutAccount: " 1234 ", PayoutName: " 张三 ", UserNote: " note "}
}

func TestAffiliateWithdrawalPolicy(t *testing.T) {
	svc, _, settings := newWithdrawalTestService()
	ctx := context.Background()
	policy, err := svc.GetWithdrawalPolicy(ctx)
	require.NoError(t, err)
	require.Equal(t, 100.0, policy.MinimumAmount)
	for _, amount := range []float64{0, 12.345678914, 1000} {
		policy, err = svc.SetWithdrawalPolicy(ctx, amount)
		require.NoError(t, err)
		require.Equal(t, roundTo(amount, 8), policy.MinimumAmount)
		loaded, loadErr := svc.GetWithdrawalPolicy(ctx)
		require.NoError(t, loadErr)
		require.Equal(t, policy, loaded)
	}
	for _, amount := range []float64{-1, math.NaN(), math.Inf(1), 1e12} {
		_, err = svc.SetWithdrawalPolicy(ctx, amount)
		require.ErrorIs(t, err, ErrAffiliateWithdrawAmountInvalid)
	}
	for _, value := range []string{"broken", "NaN", "-1", "1000000000000"} {
		settings.values[SettingKeyAffiliateWithdrawalMinimumAmount] = value
		policy, err = svc.GetWithdrawalPolicy(ctx)
		require.Error(t, err)
		require.Nil(t, policy)
	}
	settings.err = errors.New("database unavailable")
	_, err = svc.GetWithdrawalPolicy(ctx)
	require.ErrorIs(t, err, settings.err)
	_, err = svc.SetWithdrawalPolicy(ctx, 0)
	require.ErrorIs(t, err, settings.err)
}

func TestAffiliateWithdrawalApplyValidationAndUserScopedIdempotency(t *testing.T) {
	svc, repo, settings := newWithdrawalTestService()
	settings.values[SettingKeyAffiliateWithdrawalMinimumAmount] = "25"
	input := withdrawalTestInput()
	_, err := svc.ApplyWithdrawal(context.Background(), 7, input, "same-key")
	require.NoError(t, err)
	require.Equal(t, 25.0, repo.minimum)
	require.Equal(t, "张三", repo.input.PayoutName)
	require.Equal(t, "Bank", repo.input.PayoutMethod)
	require.Equal(t, "1234", repo.input.PayoutAccount)
	require.Equal(t, "note", repo.input.UserNote)
	firstID := repo.operationID
	_, err = svc.ApplyWithdrawal(context.Background(), 7, input, "same-key")
	require.NoError(t, err)
	require.Equal(t, firstID, repo.operationID)
	_, err = svc.ApplyWithdrawal(context.Background(), 8, input, "same-key")
	require.NoError(t, err)
	require.NotEqual(t, firstID, repo.operationID)

	for _, amount := range []float64{0, -1, math.NaN(), math.Inf(1), 1e-10, 1e12} {
		bad := input
		bad.Amount = amount
		before := repo.calls
		_, err = svc.ApplyWithdrawal(context.Background(), 7, bad, "new-key")
		require.ErrorIs(t, err, ErrAffiliateWithdrawAmountInvalid)
		require.Equal(t, before, repo.calls)
	}
	for _, change := range []func(*AffiliateWithdrawalInput){
		func(v *AffiliateWithdrawalInput) { v.PayoutMethod = " " },
		func(v *AffiliateWithdrawalInput) { v.PayoutAccount = "" },
		func(v *AffiliateWithdrawalInput) { v.PayoutName = "" },
		func(v *AffiliateWithdrawalInput) { v.UserNote = strings.Repeat("中", 501) },
	} {
		bad := input
		change(&bad)
		_, err = svc.ApplyWithdrawal(context.Background(), 7, bad, "new-key")
		require.ErrorIs(t, err, ErrAffiliateWithdrawalDetails)
	}
	_, err = svc.ApplyWithdrawal(context.Background(), 7, input, "")
	require.ErrorIs(t, err, ErrIdempotencyKeyRequired)
	_, err = svc.ApplyWithdrawal(context.Background(), 0, input, "key")
	require.ErrorIs(t, err, ErrUserNotFound)
	settings.values[SettingKeyAffiliateEnabled] = "false"
	_, err = svc.ApplyWithdrawal(context.Background(), 7, input, "key")
	require.Error(t, err)
}

func TestAffiliateWithdrawalReviewAndListValidation(t *testing.T) {
	svc, repo, _ := newWithdrawalTestService()
	ctx := context.Background()
	for _, input := range []AffiliateWithdrawalReview{
		{Status: "pending"}, {Status: "paid"}, {Status: "rejected"}, {Status: "processing", AdminNote: strings.Repeat("x", 501)},
	} {
		_, err := svc.ReviewWithdrawal(ctx, 1, 9, input)
		require.Error(t, err)
	}
	_, err := svc.ReviewWithdrawal(ctx, 1, 0, AffiliateWithdrawalReview{Status: "processing"})
	require.Error(t, err)
	require.Zero(t, repo.calls)
	for _, input := range []AffiliateWithdrawalReview{
		{Status: "processing"}, {Status: "paid", PaymentReference: " reference "}, {Status: "rejected", AdminNote: " reason "},
	} {
		_, err = svc.ReviewWithdrawal(ctx, 1, 9, input)
		require.NoError(t, err)
		require.Equal(t, int64(9), repo.adminID)
		require.Equal(t, strings.TrimSpace(input.AdminNote), repo.review.AdminNote)
		require.Equal(t, strings.TrimSpace(input.PaymentReference), repo.review.PaymentReference)
	}
	_, _, err = svc.ListWithdrawals(ctx, AffiliateWithdrawalFilter{UserID: 7, Page: -1, PageSize: 999, Search: " hello "})
	require.NoError(t, err)
	require.Equal(t, int64(7), repo.filter.UserID)
	require.Equal(t, 1, repo.filter.Page)
	require.Equal(t, 20, repo.filter.PageSize)
	require.Equal(t, "hello", repo.filter.Search)
	_, _, err = svc.ListWithdrawals(ctx, AffiliateWithdrawalFilter{Status: "other"})
	require.ErrorIs(t, err, ErrAffiliateWithdrawalState)
}
