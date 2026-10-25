ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS group_ids JSONB NOT NULL DEFAULT '[]'::jsonb;
UPDATE api_keys SET group_ids = jsonb_build_array(group_id) WHERE group_id IS NOT NULL AND (group_ids = '[]'::jsonb OR group_ids IS NULL);
