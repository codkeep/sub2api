package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func newWithdrawalRepositoryTest(t *testing.T) (*affiliateRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	return &affiliateRepository{client: client}, mock
}
func withdrawalRows(status string, admin any, note, reference string) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{"id", "user_id", "email", "username", "amount", "minimum", "method", "account", "name", "note", "status", "admin_note", "reference", "processed_by", "processed_at", "created_at", "updated_at"}).
		AddRow(11, 7, "u@example.com", "user", 150.0, 100.0, "bank", "1234", "name", "note", status, note, reference, admin, nil, now, now)
}
func withdrawalRepoInput() service.AffiliateWithdrawalInput {
	return service.AffiliateWithdrawalInput{Amount: 150, PayoutMethod: "bank", PayoutAccount: "1234", PayoutName: "name", UserNote: "note"}
}

const withdrawalSelect = "(?s)SELECT .* FROM affiliate_withdrawal_requests w JOIN users u ON u.id=w.user_id WHERE "

func expectWithdrawalClaim(mock sqlmock.Sqlmock, claimed bool) {
	rows := sqlmock.NewRows([]string{"id"})
	if claimed {
		rows.AddRow(11)
	}
	mock.ExpectQuery("(?s)INSERT INTO affiliate_withdrawal_requests").
		WithArgs(int64(7), 150.0, 100.0, "bank", "1234", "name", "note", "operation").
		WillReturnRows(rows)
}
func TestAffiliateWithdrawalCreateReservesQuotaAtomically(t *testing.T) {
	for _, sufficient := range []bool{false, true} {
		t.Run(map[bool]string{false: "insufficient_rolls_back", true: "reserved_once"}[sufficient], func(t *testing.T) {
			repo, mock := newWithdrawalRepositoryTest(t)
			mock.ExpectBegin()
			expectWithdrawalClaim(mock, true)
			mock.ExpectQuery("(?s)WITH matured AS").WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"amount"}).AddRow(0))
			affected := int64(0)
			if sufficient {
				affected = 1
			}
			mock.ExpectExec("UPDATE user_affiliates SET aff_quota=aff_quota-").WithArgs(150.0, int64(7)).WillReturnResult(sqlmock.NewResult(0, affected))
			if sufficient {
				mock.ExpectExec("(?s)INSERT INTO user_affiliate_ledger").WithArgs(int64(7), "withdraw_hold", 150.0).WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectQuery(withdrawalSelect).WithArgs(int64(11)).WillReturnRows(withdrawalRows("pending", nil, "", ""))
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}
			result, err := repo.CreateAffiliateWithdrawal(context.Background(), 7, withdrawalRepoInput(), 100, "operation")
			if sufficient {
				require.NoError(t, err)
				require.Equal(t, int64(11), result.ID)
				require.False(t, result.Replayed)
			} else {
				require.ErrorIs(t, err, service.ErrAffiliateQuotaInsufficient)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
func TestAffiliateWithdrawalCreateReplayDoesNotDeductAndChecksPayload(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		repo, mock := newWithdrawalRepositoryTest(t)
		input := withdrawalRepoInput()
		if conflict {
			input.PayoutAccount = "changed"
		}
		mock.ExpectBegin()
		mock.ExpectQuery("(?s)INSERT INTO affiliate_withdrawal_requests").WithArgs(int64(7), 150.0, 100.0, "bank", input.PayoutAccount, "name", "note", "operation").WillReturnRows(sqlmock.NewRows([]string{"id"}))
		mock.ExpectQuery(withdrawalSelect).WithArgs("operation", int64(7)).WillReturnRows(withdrawalRows("pending", nil, "", ""))
		if conflict {
			mock.ExpectRollback()
		} else {
			mock.ExpectCommit()
		}
		result, err := repo.CreateAffiliateWithdrawal(context.Background(), 7, input, 100, "operation")
		if conflict {
			require.ErrorIs(t, err, service.ErrIdempotencyKeyConflict)
		} else {
			require.NoError(t, err)
			require.True(t, result.Replayed)
		}
		require.NoError(t, mock.ExpectationsWereMet())
	}
}
func TestAffiliateWithdrawalBelowMinimumRollsBackClaim(t *testing.T) {
	repo, mock := newWithdrawalRepositoryTest(t)
	mock.ExpectBegin()
	mock.ExpectQuery("(?s)INSERT INTO affiliate_withdrawal_requests").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11))
	mock.ExpectRollback()
	_, err := repo.CreateAffiliateWithdrawal(context.Background(), 7, withdrawalRepoInput(), 200, "operation")
	require.ErrorIs(t, err, service.ErrAffiliateWithdrawalBelowMinimum)
	require.NoError(t, mock.ExpectationsWereMet())
}
func TestAffiliateWithdrawalReviewRefundAndPaidNeverDeductsAgain(t *testing.T) {
	for _, next := range []string{"rejected", "paid", "processing"} {
		t.Run(next, func(t *testing.T) {
			repo, mock := newWithdrawalRepositoryTest(t)
			before := "pending"
			var admin any
			if next == "paid" {
				before = "processing"
				admin = int64(9)
			}
			input := service.AffiliateWithdrawalReview{Status: next, AdminNote: "note", PaymentReference: "reference"}
			mock.ExpectBegin()
			mock.ExpectQuery(withdrawalSelect + ".*FOR UPDATE OF w").WithArgs(int64(11)).WillReturnRows(withdrawalRows(before, admin, "", ""))
			if next == "rejected" {
				mock.ExpectExec("UPDATE user_affiliates SET aff_quota=aff_quota[+]").WithArgs(150.0, int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec("(?s)INSERT INTO user_affiliate_ledger").WithArgs(int64(7), "withdraw_refund", 150.0).WillReturnResult(sqlmock.NewResult(1, 1))
			} else if next == "paid" {
				mock.ExpectExec("(?s)INSERT INTO user_affiliate_ledger").WithArgs(int64(7), "withdraw", 150.0).WillReturnResult(sqlmock.NewResult(1, 1))
			}
			mock.ExpectExec("UPDATE affiliate_withdrawal_requests SET").WithArgs(next, "note", "reference", int64(9), int64(11)).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery(withdrawalSelect).WithArgs(int64(11)).WillReturnRows(withdrawalRows(next, int64(9), "note", "reference"))
			mock.ExpectCommit()
			result, err := repo.ReviewAffiliateWithdrawal(context.Background(), 11, 9, input)
			require.NoError(t, err)
			require.Equal(t, next, result.Status)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
func TestAffiliateWithdrawalReviewReplayRefundsOnlyOnce(t *testing.T) {
	repo, mock := newWithdrawalRepositoryTest(t)
	mock.ExpectBegin()
	mock.ExpectQuery(withdrawalSelect + ".*FOR UPDATE OF w").WithArgs(int64(11)).WillReturnRows(withdrawalRows("rejected", int64(9), "reason", ""))
	mock.ExpectCommit()
	result, err := repo.ReviewAffiliateWithdrawal(context.Background(), 11, 9, service.AffiliateWithdrawalReview{Status: "rejected", AdminNote: "reason"})
	require.NoError(t, err)
	require.True(t, result.Replayed)
	require.NoError(t, mock.ExpectationsWereMet())
}
func TestAffiliateWithdrawalReviewRejectsFinalOrOtherAdminRequests(t *testing.T) {
	for _, status := range []string{"paid", "rejected", "processing", "pending"} {
		repo, mock := newWithdrawalRepositoryTest(t)
		mock.ExpectBegin()
		mock.ExpectQuery(withdrawalSelect + ".*FOR UPDATE OF w").WithArgs(int64(11)).WillReturnRows(withdrawalRows(status, int64(8), "", ""))
		mock.ExpectRollback()
		_, err := repo.ReviewAffiliateWithdrawal(context.Background(), 11, 9, service.AffiliateWithdrawalReview{Status: "paid", PaymentReference: "ref"})
		require.ErrorIs(t, err, service.ErrAffiliateWithdrawalState)
		require.NoError(t, mock.ExpectationsWereMet())
	}
}

func TestAffiliateWithdrawalListUsesUserAndStatusScope(t *testing.T) {
	repo, mock := newWithdrawalRepositoryTest(t)
	mock.ExpectQuery("(?s)SELECT COUNT.*WHERE 1=1 AND w.user_id=\\$1 AND w.status=\\$2").
		WithArgs(int64(7), "pending").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(11))
	mock.ExpectQuery(withdrawalSelect+"1=1 AND w.user_id=\\$1 AND w.status=\\$2.*LIMIT \\$3 OFFSET \\$4").
		WithArgs(int64(7), "pending", 10, 10).WillReturnRows(withdrawalRows("pending", nil, "", ""))
	items, total, err := repo.ListAffiliateWithdrawals(context.Background(), service.AffiliateWithdrawalFilter{UserID: 7, Status: "pending", Page: 2, PageSize: 10})
	require.NoError(t, err)
	require.EqualValues(t, 11, total)
	require.Len(t, items, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAffiliateWithdrawalLedgerFailureRollsBackQuotaChange(t *testing.T) {
	failure := errors.New("ledger unavailable")
	for _, action := range []string{"apply", "reject"} {
		t.Run(action, func(t *testing.T) {
			repo, mock := newWithdrawalRepositoryTest(t)
			mock.ExpectBegin()
			if action == "apply" {
				expectWithdrawalClaim(mock, true)
				mock.ExpectQuery("(?s)WITH matured AS").WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"amount"}).AddRow(0))
				mock.ExpectExec("UPDATE user_affiliates SET aff_quota=aff_quota-").WithArgs(150.0, int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
			} else {
				mock.ExpectQuery(withdrawalSelect + ".*FOR UPDATE OF w").WithArgs(int64(11)).WillReturnRows(withdrawalRows("pending", nil, "", ""))
				mock.ExpectExec("UPDATE user_affiliates SET aff_quota=aff_quota[+]").WithArgs(150.0, int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			mock.ExpectExec("(?s)INSERT INTO user_affiliate_ledger").WillReturnError(failure)
			mock.ExpectRollback()
			var err error
			if action == "apply" {
				_, err = repo.CreateAffiliateWithdrawal(context.Background(), 7, withdrawalRepoInput(), 100, "operation")
			} else {
				_, err = repo.ReviewAffiliateWithdrawal(context.Background(), 11, 9, service.AffiliateWithdrawalReview{Status: "rejected", AdminNote: "reason"})
			}
			require.ErrorIs(t, err, failure)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
