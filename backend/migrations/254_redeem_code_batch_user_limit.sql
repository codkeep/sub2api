-- 兑换码批次级用户使用上限
ALTER TABLE redeem_codes
    ADD COLUMN IF NOT EXISTS batch_id VARCHAR(64),
    ADD COLUMN IF NOT EXISTS per_user_limit INTEGER NOT NULL DEFAULT 1;

ALTER TABLE redeem_codes
    DROP CONSTRAINT IF EXISTS redeem_codes_per_user_limit_positive;
ALTER TABLE redeem_codes
    ADD CONSTRAINT redeem_codes_per_user_limit_positive CHECK (per_user_limit > 0);

CREATE INDEX IF NOT EXISTS idx_redeem_codes_batch_id ON redeem_codes(batch_id);

COMMENT ON COLUMN redeem_codes.batch_id IS '批量生成时共享的批次标识';
COMMENT ON COLUMN redeem_codes.per_user_limit IS '同一用户在该批次最多可成功兑换的兑换码数量';
