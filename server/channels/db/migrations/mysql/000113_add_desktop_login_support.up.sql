-- Add index on Sessions.Props for device_code lookup (MySQL 5.7+)
-- This improves performance when querying sessions by device code using JSON functions
SET @exist := (SELECT COUNT(*) FROM information_schema.statistics 
    WHERE table_schema = DATABASE() AND table_name = 'Sessions' 
    AND index_name = 'idx_sessions_props_device_code');

SET @sqlstmt := IF(@exist = 0, 
    'ALTER TABLE Sessions ADD INDEX idx_sessions_props_device_code ((CAST(JSON_EXTRACT(Props, ''$.device_code'') AS CHAR(255))))',
    'SELECT ''Index already exists''');

PREPARE stmt FROM @sqlstmt;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- Note: Props is already a JSON column in MySQL 5.7+, so no schema change needed
-- The JSON_EXTRACT queries in session_store.go will work with existing schema

