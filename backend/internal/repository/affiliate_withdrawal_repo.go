package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

var _ service.AffiliateWithdrawalRepository = (*affiliateRepository)(nil)

const affiliateWithdrawalColumns = `w.id, w.user_id, COALESCE(u.email, ''), COALESCE(u.username, ''),
    w.amount::double precision, w.minimum_amount::double precision,
    w.payout_method, w.payout_account, w.payout_name, w.user_note, w.status,
    w.admin_note, w.payment_reference, w.processed_by, w.processed_at, w.created_at, w.updated_at`

func scanAffiliateWithdrawal(rows *sql.Rows) (*service.AffiliateWithdrawal, error) {
	var item service.AffiliateWithdrawal
	err := rows.Scan(&item.ID, &item.UserID, &item.UserEmail, &item.Username, &item.Amount, &item.MinimumAmount,
		&item.PayoutMethod, &item.PayoutAccount, &item.PayoutName, &item.UserNote, &item.Status,
		&item.AdminNote, &item.PaymentReference, &item.ProcessedBy, &item.ProcessedAt, &item.CreatedAt, &item.UpdatedAt)
	return &item, err
}

func queryAffiliateWithdrawal(ctx context.Context, client affiliateQueryExecer, where string, args []any, lock bool) (*service.AffiliateWithdrawal, error) {
	query := "SELECT " + affiliateWithdrawalColumns + " FROM affiliate_withdrawal_requests w JOIN users u ON u.id=w.user_id WHERE " + where
	if lock {
		query += " FOR UPDATE OF w"
	}
	rows, err := client.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, service.ErrAffiliateWithdrawalNotFound
	}
	return scanAffiliateWithdrawal(rows)
}

func (r *affiliateRepository) CreateAffiliateWithdrawal(ctx context.Context, userID int64, input service.AffiliateWithdrawalInput, minimum float64, operationID string) (*service.AffiliateWithdrawal, error) {
	var result *service.AffiliateWithdrawal
	err := r.withTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		// 先占用幂等标识。并发重试在唯一索引上等待，随后返回同一笔申请。
		rows, err := client.QueryContext(txCtx, `INSERT INTO affiliate_withdrawal_requests
            (user_id, amount, minimum_amount, payout_method, payout_account, payout_name, user_note, operation_id)
            VALUES ($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT (operation_id) DO NOTHING RETURNING id`,
			userID, input.Amount, minimum, input.PayoutMethod, input.PayoutAccount, input.PayoutName, input.UserNote, operationID)
		if err != nil {
			return err
		}
		claimed := rows.Next()
		var id int64
		if claimed {
			err = rows.Scan(&id)
		} else {
			err = rows.Err()
		}
		closeErr := rows.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		if !claimed {
			result, err = queryAffiliateWithdrawal(txCtx, client, "w.operation_id=$1 AND w.user_id=$2", []any{operationID, userID}, false)
			if err != nil {
				return err
			}
			if result.Amount != input.Amount || result.PayoutMethod != input.PayoutMethod || result.PayoutAccount != input.PayoutAccount || result.PayoutName != input.PayoutName || result.UserNote != input.UserNote {
				return service.ErrIdempotencyKeyConflict
			}
			result.Replayed = true
			return nil
		}
		if input.Amount < minimum {
			return service.ErrAffiliateWithdrawalBelowMinimum.WithMetadata(map[string]string{"minimum": strconv.FormatFloat(minimum, 'f', -1, 64)})
		}
		if _, err := thawFrozenQuotaTx(txCtx, client, userID); err != nil {
			return err
		}
		res, err := client.ExecContext(txCtx, `UPDATE user_affiliates SET aff_quota=aff_quota-$1, updated_at=NOW() WHERE user_id=$2 AND aff_quota >= $1`, input.Amount, userID)
		if err != nil {
			return err
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if affected != 1 {
			return service.ErrAffiliateQuotaInsufficient
		}
		if err := recordAffiliateWithdrawalLedger(txCtx, client, userID, input.Amount, "withdraw_hold"); err != nil {
			return err
		}
		result, err = queryAffiliateWithdrawal(txCtx, client, "w.id=$1", []any{id}, false)
		return err
	})
	return result, err
}

func (r *affiliateRepository) ListAffiliateWithdrawals(ctx context.Context, filter service.AffiliateWithdrawalFilter) ([]service.AffiliateWithdrawal, int64, error) {
	client := clientFromContext(ctx, r.client)
	conditions := []string{"1=1"}
	args := []any{}
	add := func(condition string, value any) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf(condition, len(args)))
	}
	if filter.UserID > 0 {
		add("w.user_id=$%d", filter.UserID)
	}
	if filter.Status != "" {
		add("w.status=$%d", filter.Status)
	}
	if filter.Search != "" {
		add("(u.email ILIKE $%[1]d OR u.username ILIKE $%[1]d OR w.user_id::text ILIKE $%[1]d OR w.id::text ILIKE $%[1]d)", "%"+filter.Search+"%")
	}
	where := " FROM affiliate_withdrawal_requests w JOIN users u ON u.id=w.user_id WHERE " + strings.Join(conditions, " AND ")
	total, err := queryAffiliateRecordCount(ctx, client, "SELECT COUNT(*)"+where, args...)
	if err != nil {
		return nil, 0, err
	}
	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	query := "SELECT " + affiliateWithdrawalColumns + where + fmt.Sprintf(" ORDER BY w.created_at DESC, w.id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	rows, err := client.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]service.AffiliateWithdrawal, 0)
	for rows.Next() {
		item, err := scanAffiliateWithdrawal(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *item)
	}
	return items, total, rows.Err()
}

func (r *affiliateRepository) ReviewAffiliateWithdrawal(ctx context.Context, id, adminID int64, input service.AffiliateWithdrawalReview) (*service.AffiliateWithdrawal, error) {
	var result *service.AffiliateWithdrawal
	err := r.withTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		current, err := queryAffiliateWithdrawal(txCtx, client, "w.id=$1", []any{id}, true)
		if err != nil {
			return err
		}
		if current.Status == input.Status && current.AdminNote == input.AdminNote && current.PaymentReference == input.PaymentReference && current.ProcessedBy != nil && *current.ProcessedBy == adminID {
			result = current
			result.Replayed = true
			return nil
		}
		// 接单后由同一管理员完成或拒绝，避免多名管理员同时线下打款。
		if current.Status == "processing" && (current.ProcessedBy == nil || *current.ProcessedBy != adminID) {
			return service.ErrAffiliateWithdrawalState
		}
		switch input.Status {
		case "processing":
			if current.Status != "pending" {
				return service.ErrAffiliateWithdrawalState
			}
		case "paid":
			if current.Status != "processing" {
				return service.ErrAffiliateWithdrawalState
			}
		case "rejected":
			if current.Status != "pending" && current.Status != "processing" {
				return service.ErrAffiliateWithdrawalState
			}
		default:
			return service.ErrAffiliateWithdrawalState
		}
		if input.Status == "rejected" {
			res, err := client.ExecContext(txCtx, `UPDATE user_affiliates SET aff_quota=aff_quota+$1, updated_at=NOW() WHERE user_id=$2`, current.Amount, current.UserID)
			if err != nil {
				return err
			}
			affected, err := res.RowsAffected()
			if err != nil {
				return err
			}
			if affected != 1 {
				return service.ErrAffiliateProfileNotFound
			}
			if err := recordAffiliateWithdrawalLedger(txCtx, client, current.UserID, current.Amount, "withdraw_refund"); err != nil {
				return err
			}
		} else if input.Status == "paid" {
			// 申请时已经预留；这里只记实际提现流水，不能再次扣减可用额度。
			if err := recordAffiliateWithdrawalLedger(txCtx, client, current.UserID, current.Amount, "withdraw"); err != nil {
				return err
			}
		}
		_, err = client.ExecContext(txCtx, `UPDATE affiliate_withdrawal_requests SET status=$1, admin_note=$2, payment_reference=$3, processed_by=$4, processed_at=NOW(), updated_at=NOW() WHERE id=$5`, input.Status, input.AdminNote, input.PaymentReference, adminID, id)
		if err != nil {
			return err
		}
		result, err = queryAffiliateWithdrawal(txCtx, client, "w.id=$1", []any{id}, false)
		return err
	})
	return result, err
}

func recordAffiliateWithdrawalLedger(ctx context.Context, client affiliateQueryExecer, userID int64, amount float64, action string) error {
	_, err := client.ExecContext(ctx, `INSERT INTO user_affiliate_ledger
        (user_id, action, amount, balance_after, aff_quota_after, aff_frozen_quota_after, aff_history_quota_after)
        SELECT ua.user_id, $2, $3, u.balance, ua.aff_quota, ua.aff_frozen_quota, ua.aff_history_quota
        FROM user_affiliates ua JOIN users u ON u.id=ua.user_id WHERE ua.user_id=$1`, userID, action, amount)
	return err
}
