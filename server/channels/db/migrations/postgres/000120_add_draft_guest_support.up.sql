-- Add guest user support to drafts table
-- This enables better draft tracking and analytics for guest user onboarding

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'drafts' AND column_name = 'isguest'
    ) THEN
        ALTER TABLE drafts ADD COLUMN isguest boolean DEFAULT FALSE;
    END IF;
END $$;

-- Add index for guest draft queries (performance optimization)
-- This significantly improves guest draft retrieval performance
CREATE INDEX IF NOT EXISTS idx_drafts_user_guest ON drafts(userid, isguest) WHERE isguest = TRUE;

-- Add index for channel-based draft queries
-- Optimizes draft loading when switching channels
CREATE INDEX IF NOT EXISTS idx_drafts_channel_updateat ON drafts(channelid, updateat DESC);

-- Update statistics for query planner optimization
ANALYZE drafts;

