-- Add index on Sessions.Props for device_code lookup
-- This improves performance when querying sessions by device code
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sessions_props_device_code ON sessions USING gin (props);

-- Note: Props is already a JSONB column in PostgreSQL, so no schema change needed
-- The JSON_EXTRACT queries in session_store.go will work with existing schema

-- Add comment documenting the desktop login feature
COMMENT ON COLUMN sessions.props IS 'Session properties including device_code, session_state, and device info for desktop login flow';

