-- Add MFA recovery fields to Users table
-- These fields support the MFA recovery and backup codes feature

-- Add recovery state field
ALTER TABLE users ADD COLUMN IF NOT EXISTS mfarecoverystate VARCHAR(64) DEFAULT '';

-- Add recovery expiry timestamp
ALTER TABLE users ADD COLUMN IF NOT EXISTS mfarecoveryexpiry BIGINT DEFAULT 0;

-- Add backup codes (stored as TEXT to hold JSON array of hashed codes)
ALTER TABLE users ADD COLUMN IF NOT EXISTS mfabackupcodes TEXT DEFAULT '';

-- Add used backup codes tracking
ALTER TABLE users ADD COLUMN IF NOT EXISTS mfabackupcodesused TEXT DEFAULT '';

-- Add index on recovery state for efficient querying of users in recovery
CREATE INDEX IF NOT EXISTS idx_users_mfarecoverystate ON users(mfarecoverystate) WHERE mfarecoverystate != '';

-- Add index on recovery expiry for cleanup of expired recovery states
CREATE INDEX IF NOT EXISTS idx_users_mfarecoveryexpiry ON users(mfarecoveryexpiry) WHERE mfarecoveryexpiry > 0;

-- Add comments documenting the MFA recovery feature
COMMENT ON COLUMN users.mfarecoverystate IS 'MFA recovery state: pending, email_sent, code_verified, backup_code, or admin_assisted';
COMMENT ON COLUMN users.mfarecoveryexpiry IS 'Unix timestamp (milliseconds) when the MFA recovery state expires';
COMMENT ON COLUMN users.mfabackupcodes IS 'JSON array of hashed backup codes for MFA recovery';
COMMENT ON COLUMN users.mfabackupcodesused IS 'JSON array of used backup codes to prevent reuse';

