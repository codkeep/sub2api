package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const SettingKeyAffiliateWithdrawalMinimumAmount = "affiliate_withdrawal_minimum_amount"
const AffiliateWithdrawalMinimumDefault = 100.0

var (
	ErrAffiliateWithdrawalNotFound     = infraerrors.NotFound("AFFILIATE_WITHDRAWAL_NOT_FOUND", "withdrawal request not found")
	ErrAffiliateWithdrawalState        = infraerrors.Conflict("AFFILIATE_WITHDRAWAL_STATE_CONFLICT", "withdrawal status has changed; refresh and try again")
	ErrAffiliateWithdrawalBelowMinimum = infraerrors.BadRequest("AFFILIATE_WITHDRAWAL_BELOW_MINIMUM", "withdrawal amount is below the minimum")
	ErrAffiliateWithdrawalDetails      = infraerrors.BadRequest("AFFILIATE_WITHDRAWAL_DETAILS_INVALID", "provide a valid payout method, account and recipient name")
)

type AffiliateWithdrawalPolicy struct {
	MinimumAmount float64 `json:"minimum_amount"`
}

type AffiliateWithdrawalInput struct {
	Amount        float64 `json:"amount"`
	PayoutMethod  string  `json:"payout_method"`
	PayoutAccount string  `json:"payout_account"`
	PayoutName    string  `json:"payout_name"`
	UserNote      string  `json:"user_note"`
}

type AffiliateWithdrawal struct {
	ID               int64      `json:"id"`
	UserID           int64      `json:"user_id"`
	UserEmail        string     `json:"user_email,omitempty"`
	Username         string     `json:"username,omitempty"`
	Amount           float64    `json:"amount"`
	MinimumAmount    float64    `json:"minimum_amount"`
	PayoutMethod     string     `json:"payout_method"`
	PayoutAccount    string     `json:"payout_account"`
	PayoutName       string     `json:"payout_name"`
	UserNote         string     `json:"user_note"`
	Status           string     `json:"status"`
	AdminNote        string     `json:"admin_note"`
	PaymentReference string     `json:"payment_reference"`
	ProcessedBy      *int64     `json:"processed_by,omitempty"`
	ProcessedAt      *time.Time `json:"processed_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	Replayed         bool       `json:"-"`
}

type AffiliateWithdrawalFilter struct {
	UserID   int64
	Status   string
	Search   string
	Page     int
	PageSize int
}

type AffiliateWithdrawalReview struct {
	Status           string `json:"status"`
	AdminNote        string `json:"admin_note"`
	PaymentReference string `json:"payment_reference"`
}

// 单独接口让原有返利入账、转余额仓储及其调用方保持独立。
type AffiliateWithdrawalRepository interface {
	CreateAffiliateWithdrawal(context.Context, int64, AffiliateWithdrawalInput, float64, string) (*AffiliateWithdrawal, error)
	ListAffiliateWithdrawals(context.Context, AffiliateWithdrawalFilter) ([]AffiliateWithdrawal, int64, error)
	ReviewAffiliateWithdrawal(context.Context, int64, int64, AffiliateWithdrawalReview) (*AffiliateWithdrawal, error)
}

func (s *AffiliateService) withdrawalRepository() (AffiliateWithdrawalRepository, error) {
	if s != nil {
		if repo, ok := s.repo.(AffiliateWithdrawalRepository); ok {
			return repo, nil
		}
	}
	return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate withdrawal service unavailable")
}

func validAffiliateWithdrawalAmount(amount float64) bool {
	return !math.IsNaN(amount) && !math.IsInf(amount, 0) && amount >= 0 && amount < 1e12
}

func (s *AffiliateService) GetWithdrawalPolicy(ctx context.Context) (*AffiliateWithdrawalPolicy, error) {
	policy := &AffiliateWithdrawalPolicy{MinimumAmount: AffiliateWithdrawalMinimumDefault}
	if s == nil || s.settingService == nil || s.settingService.settingRepo == nil {
		return policy, nil
	}
	raw, err := s.settingService.settingRepo.GetValue(ctx, SettingKeyAffiliateWithdrawalMinimumAmount)
	if errors.Is(err, ErrSettingNotFound) {
		return policy, nil
	}
	if err != nil {
		return nil, err
	}
	amount, err := strconv.ParseFloat(raw, 64)
	if err != nil || !validAffiliateWithdrawalAmount(amount) {
		return nil, infraerrors.ServiceUnavailable("AFFILIATE_WITHDRAWAL_POLICY_INVALID", "withdrawal settings are invalid")
	}
	policy.MinimumAmount = roundTo(amount, 8)
	if !validAffiliateWithdrawalAmount(policy.MinimumAmount) {
		return nil, infraerrors.ServiceUnavailable("AFFILIATE_WITHDRAWAL_POLICY_INVALID", "withdrawal settings are invalid")
	}
	return policy, nil
}

func (s *AffiliateService) SetWithdrawalPolicy(ctx context.Context, amount float64) (*AffiliateWithdrawalPolicy, error) {
	if !validAffiliateWithdrawalAmount(amount) {
		return nil, ErrAffiliateWithdrawAmountInvalid
	}
	if s == nil || s.settingService == nil || s.settingService.settingRepo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "settings service unavailable")
	}
	amount = roundTo(amount, 8)
	if !validAffiliateWithdrawalAmount(amount) {
		return nil, ErrAffiliateWithdrawAmountInvalid
	}
	if err := s.settingService.settingRepo.Set(ctx, SettingKeyAffiliateWithdrawalMinimumAmount, strconv.FormatFloat(amount, 'f', 8, 64)); err != nil {
		return nil, err
	}
	return &AffiliateWithdrawalPolicy{MinimumAmount: amount}, nil
}

func (s *AffiliateService) ApplyWithdrawal(ctx context.Context, userID int64, input AffiliateWithdrawalInput, key string) (*AffiliateWithdrawal, error) {
	repo, err := s.withdrawalRepository()
	if err != nil {
		return nil, err
	}
	if userID <= 0 {
		return nil, ErrUserNotFound
	}
	if !s.IsEnabled(ctx) {
		return nil, infraerrors.Forbidden("AFFILIATE_DISABLED", "affiliate program is disabled")
	}
	key, err = NormalizeIdempotencyKey(key)
	if err != nil {
		return nil, err
	}
	if key == "" {
		return nil, ErrIdempotencyKeyRequired
	}
	if !validAffiliateWithdrawalAmount(input.Amount) {
		return nil, ErrAffiliateWithdrawAmountInvalid
	}
	input.Amount = roundTo(input.Amount, 8)
	if input.Amount <= 0 || !validAffiliateWithdrawalAmount(input.Amount) {
		return nil, ErrAffiliateWithdrawAmountInvalid
	}
	input.PayoutMethod = strings.TrimSpace(input.PayoutMethod)
	input.PayoutAccount = strings.TrimSpace(input.PayoutAccount)
	input.PayoutName = strings.TrimSpace(input.PayoutName)
	input.UserNote = strings.TrimSpace(input.UserNote)
	if input.PayoutMethod == "" || len([]rune(input.PayoutMethod)) > 40 || input.PayoutAccount == "" || len([]rune(input.PayoutAccount)) > 200 || input.PayoutName == "" || len([]rune(input.PayoutName)) > 100 || len([]rune(input.UserNote)) > 500 {
		return nil, ErrAffiliateWithdrawalDetails
	}
	policy, err := s.GetWithdrawalPolicy(ctx)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte("user.affiliates.withdrawal\x00" + strconv.FormatInt(userID, 10) + "\x00" + key))
	return repo.CreateAffiliateWithdrawal(ctx, userID, input, policy.MinimumAmount, hex.EncodeToString(sum[:]))
}

func (s *AffiliateService) ListWithdrawals(ctx context.Context, filter AffiliateWithdrawalFilter) ([]AffiliateWithdrawal, int64, error) {
	repo, err := s.withdrawalRepository()
	if err != nil {
		return nil, 0, err
	}
	switch filter.Status {
	case "", "pending", "processing", "paid", "rejected":
	default:
		return nil, 0, ErrAffiliateWithdrawalState
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	if filter.Page > math.MaxInt/filter.PageSize {
		filter.Page = math.MaxInt / filter.PageSize
	}
	filter.Search = strings.TrimSpace(filter.Search)
	return repo.ListAffiliateWithdrawals(ctx, filter)
}

func (s *AffiliateService) ReviewWithdrawal(ctx context.Context, id, adminID int64, input AffiliateWithdrawalReview) (*AffiliateWithdrawal, error) {
	repo, err := s.withdrawalRepository()
	if err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, ErrAffiliateWithdrawalNotFound
	}
	if adminID <= 0 {
		return nil, infraerrors.Forbidden("ADMIN_REQUIRED", "admin identity required")
	}
	input.AdminNote = strings.TrimSpace(input.AdminNote)
	input.PaymentReference = strings.TrimSpace(input.PaymentReference)
	if len([]rune(input.AdminNote)) > 500 || len([]rune(input.PaymentReference)) > 200 {
		return nil, ErrAffiliateWithdrawalDetails
	}
	switch input.Status {
	case "processing":
	case "paid":
		if input.PaymentReference == "" {
			return nil, infraerrors.BadRequest("AFFILIATE_WITHDRAWAL_REFERENCE_REQUIRED", "payment reference is required")
		}
	case "rejected":
		if input.AdminNote == "" {
			return nil, infraerrors.BadRequest("AFFILIATE_WITHDRAWAL_REJECTION_REQUIRED", "rejection reason is required")
		}
	default:
		return nil, ErrAffiliateWithdrawalState
	}
	return repo.ReviewAffiliateWithdrawal(ctx, id, adminID, input)
}
