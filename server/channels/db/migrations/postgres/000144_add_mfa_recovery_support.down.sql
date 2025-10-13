-- Rollback MFA recovery support

-- Drop indexes
DROP INDEX IF EXISTS idx_users_mfarecoverystate;
DROP INDEX IF EXISTS idx_users_mfarecoveryexpiry;

-- Drop columns
ALTER TABLE users DROP COLUMN IF EXISTS mfabackupcodesused;
ALTER TABLE users DROP COLUMN IF EXISTS mfabackupcodes;
ALTER TABLE users DROP COLUMN IF EXISTS mfarecoveryexpiry;
ALTER TABLE users DROP COLUMN IF EXISTS mfarecoverystate;

