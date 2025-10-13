-- Add MFA recovery fields to Users table
-- These fields support the MFA recovery and backup codes feature

-- Add recovery state field
ALTER TABLE Users ADD COLUMN MfaRecoveryState VARCHAR(64) DEFAULT '' AFTER MfaUsedTimestamps;

-- Add recovery expiry timestamp
ALTER TABLE Users ADD COLUMN MfaRecoveryExpiry BIGINT DEFAULT 0 AFTER MfaRecoveryState;

-- Add backup codes (stored as TEXT to hold JSON array of hashed codes)
ALTER TABLE Users ADD COLUMN MfaBackupCodes TEXT AFTER MfaRecoveryExpiry;

-- Add used backup codes tracking
ALTER TABLE Users ADD COLUMN MfaBackupCodesUsed TEXT AFTER MfaBackupCodes;

-- Add index on recovery state for efficient querying of users in recovery
CREATE INDEX idx_users_mfarecoverystate ON Users(MfaRecoveryState);

-- Add index on recovery expiry for cleanup of expired recovery states
CREATE INDEX idx_users_mfarecoveryexpiry ON Users(MfaRecoveryExpiry);

