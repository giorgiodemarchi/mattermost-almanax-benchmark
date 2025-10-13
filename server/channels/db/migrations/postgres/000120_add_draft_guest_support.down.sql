-- Rollback guest user support from drafts table

DROP INDEX IF EXISTS idx_drafts_user_guest;
DROP INDEX IF EXISTS idx_drafts_channel_updateat;

ALTER TABLE drafts DROP COLUMN IF EXISTS isguest;

