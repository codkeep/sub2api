-- 用户申请提现时预留可用返利，审核拒绝时原路退回；打款完成不再二次扣款。
CREATE TABLE IF NOT EXISTS affiliate_withdrawal_requests (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    amount DECIMAL(20,8) NOT NULL CHECK (amount > 0),
    minimum_amount DECIMAL(20,8) NOT NULL CHECK (minimum_amount >= 0),
    payout_method VARCHAR(40) NOT NULL,
    payout_account VARCHAR(200) NOT NULL,
    payout_name VARCHAR(100) NOT NULL,
    user_note VARCHAR(500) NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'paid', 'rejected')),
    admin_note VARCHAR(500) NOT NULL DEFAULT '',
    payment_reference VARCHAR(200) NOT NULL DEFAULT '',
    processed_by BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    processed_at TIMESTAMPTZ NULL,
    operation_id VARCHAR(64) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_affiliate_withdrawals_user_created ON affiliate_withdrawal_requests(user_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_affiliate_withdrawals_status_created ON affiliate_withdrawal_requests(status, created_at DESC, id DESC);
COMMENT ON TABLE affiliate_withdrawal_requests IS '邀请返利提现申请及线下打款处理记录';
INSERT INTO settings (key, value, updated_at) VALUES ('affiliate_withdrawal_minimum_amount', '100', NOW()) ON CONFLICT (key) DO NOTHING;
