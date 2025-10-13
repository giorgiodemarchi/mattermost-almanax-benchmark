-- Add bot delegation fields to support service account architectures
-- This migration adds support for bot-to-bot delegation, allowing plugins
-- to create hierarchies of bots for microservice patterns

-- Add parent_bot_id to track delegation relationships
ALTER TABLE Bots ADD COLUMN IF NOT EXISTS ParentBotId varchar(26) DEFAULT '';

-- Add delegation_type to categorize different types of delegated bots
-- Types: 'service' (service accounts), 'subbot' (child bots), 'api_client' (API clients)
ALTER TABLE Bots ADD COLUMN IF NOT EXISTS DelegationType varchar(32) DEFAULT '';

-- Add is_system_managed flag for system-level bot operations
-- This is used during migrations, bulk imports, and system maintenance
ALTER TABLE Bots ADD COLUMN IF NOT EXISTS IsSystemManaged boolean DEFAULT false;

-- Add delegation_scopes to store bot permissions as JSON
-- This allows fine-grained control over what delegated bots can access
ALTER TABLE Bots ADD COLUMN IF NOT EXISTS DelegationScopes text DEFAULT '';

-- Add service_account_id for mapping to external systems
-- Used when integrating with external authentication providers or services
ALTER TABLE Bots ADD COLUMN IF NOT EXISTS ServiceAccountId varchar(128) DEFAULT '';

-- Create index on parent_bot_id for efficient delegation chain queries
CREATE INDEX IF NOT EXISTS idx_bots_parent_bot_id ON Bots(ParentBotId);

-- Create index on delegation_type for filtering bot types
CREATE INDEX IF NOT EXISTS idx_bots_delegation_type ON Bots(DelegationType);

-- Create index on is_system_managed for system bot queries
CREATE INDEX IF NOT EXISTS idx_bots_is_system_managed ON Bots(IsSystemManaged);

