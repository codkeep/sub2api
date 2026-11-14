-- Preserve existing billing behavior until an administrator selects requested effort.
ALTER TABLE channels
    ADD COLUMN IF NOT EXISTS reasoning_effort_source VARCHAR(20) NOT NULL DEFAULT 'upstream'
    CHECK (reasoning_effort_source IN ('requested', 'upstream'));
