-- Rollback desktop login support

-- Drop the index
DROP INDEX CONCURRENTLY IF EXISTS idx_sessions_props_device_code;

-- Remove comment
COMMENT ON COLUMN sessions.props IS NULL;

