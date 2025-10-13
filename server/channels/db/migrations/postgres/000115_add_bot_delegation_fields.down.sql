-- Rollback bot delegation feature

-- Drop indexes first
DROP INDEX IF EXISTS idx_bots_is_system_managed;
DROP INDEX IF EXISTS idx_bots_delegation_type;
DROP INDEX IF EXISTS idx_bots_parent_bot_id;

-- Drop columns
ALTER TABLE Bots DROP COLUMN IF EXISTS ServiceAccountId;
ALTER TABLE Bots DROP COLUMN IF EXISTS DelegationScopes;
ALTER TABLE Bots DROP COLUMN IF EXISTS IsSystemManaged;
ALTER TABLE Bots DROP COLUMN IF EXISTS DelegationType;
ALTER TABLE Bots DROP COLUMN IF EXISTS ParentBotId;

