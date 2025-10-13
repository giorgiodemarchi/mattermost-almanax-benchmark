-- Rollback desktop login support

SET @exist := (SELECT COUNT(*) FROM information_schema.statistics 
    WHERE table_schema = DATABASE() AND table_name = 'Sessions' 
    AND index_name = 'idx_sessions_props_device_code');

SET @sqlstmt := IF(@exist > 0, 
    'ALTER TABLE Sessions DROP INDEX idx_sessions_props_device_code',
    'SELECT ''Index does not exist''');

PREPARE stmt FROM @sqlstmt;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

