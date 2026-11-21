-- Keep existing group Skills as the all-model defaults.
ALTER TABLE group_default_skills
    ADD COLUMN IF NOT EXISTS model_skill_ids JSONB NOT NULL DEFAULT '{}'::jsonb;
