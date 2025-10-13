-- Rollback MFA recovery support

-- Drop indexes
DROP INDEX idx_users_mfarecoverystate ON Users;
DROP INDEX idx_users_mfarecoveryexpiry ON Users;

-- Drop columns
ALTER TABLE Users DROP COLUMN MfaBackupCodesUsed;
ALTER TABLE Users DROP COLUMN MfaBackupCodes;
ALTER TABLE Users DROP COLUMN MfaRecoveryExpiry;
ALTER TABLE Users DROP COLUMN MfaRecoveryState;

