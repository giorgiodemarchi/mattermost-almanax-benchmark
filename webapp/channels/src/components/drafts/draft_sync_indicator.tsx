// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, { useCallback, useEffect, useState } from 'react';
import { useSelector } from 'react-redux';
import styled, { keyframes } from 'styled-components';

import type { Draft } from '@mattermost/types/drafts';

import { getCurrentUserId } from 'mattermost-redux/selectors/entities/users';

const spin = keyframes`
    from {
        transform: rotate(0deg);
    }
    to {
        transform: rotate(360deg);
    }
`;

const SyncContainer = styled.div<{ isConflict: boolean }>`
    display: flex;
    align-items: center;
    padding: 4px 8px;
    background: ${props => props.isConflict ? 'rgba(var(--error-text-color-rgb), 0.08)' : 'rgba(var(--button-bg-rgb), 0.08)'};
    border-radius: 4px;
    font-size: 11px;
    color: ${props => props.isConflict ? 'rgba(var(--error-text-color-rgb), 1)' : 'rgba(var(--center-channel-text-rgb), 0.64)'};
    margin-left: 8px;
`;

const SyncIcon = styled.i<{ isSyncing: boolean }>`
    margin-right: 4px;
    font-size: 12px;
    animation: ${props => props.isSyncing ? spin : 'none'} 1s linear infinite;
`;

const ConflictBadge = styled.span`
    padding: 2px 6px;
    background: rgba(var(--error-text-color-rgb), 1);
    color: white;
    border-radius: 10px;
    font-size: 10px;
    font-weight: 600;
    margin-left: 4px;
`;

interface DraftSyncIndicatorProps {
  draft: Draft;
  isSyncing?: boolean;
}

export const DraftSyncIndicator: React.FC<DraftSyncIndicatorProps> = ({
  draft,
  isSyncing = false,
}) => {
  const currentUserId = useSelector(getCurrentUserId);
  const [syncStatus, setSyncStatus] = useState<'idle' | 'syncing' | 'synced' | 'conflict'>('idle');
  const [deviceId, setDeviceId] = useState<string>('');
  const [conflictId, setConflictId] = useState<string>('');

  useEffect(() => {
    // Extract sync metadata from draft props
    const syncMetadata = draft.props;
    if (syncMetadata) {
      setDeviceId(syncMetadata.sync_device_id || '');
      setConflictId(syncMetadata.conflict_id || '');
    }
  }, [draft]);

  useEffect(() => {
    if (isSyncing) {
      setSyncStatus('syncing');
    } else if (conflictId) {
      setSyncStatus('conflict');
    } else if (deviceId) {
      setSyncStatus('synced');
    } else {
      setSyncStatus('idle');
    }
  }, [isSyncing, conflictId, deviceId]);

  const getSyncText = useCallback(() => {
    switch (syncStatus) {
      case 'syncing':
        return 'Syncing...';
      case 'synced':
        return 'Synced';
      case 'conflict':
        return 'Conflict detected';
      default:
        return '';
    }
  }, [syncStatus]);

  const getSyncIconClass = useCallback(() => {
    switch (syncStatus) {
      case 'syncing':
        return 'icon-loading';
      case 'synced':
        return 'icon-check';
      case 'conflict':
        return 'icon-alert-outline';
      default:
        return '';
    }
  }, [syncStatus]);

  if (syncStatus === 'idle') {
    return null;
  }

  return (
    <SyncContainer isConflict={syncStatus === 'conflict'} data-testid="draft-sync-indicator">
      <SyncIcon
        className={`icon ${getSyncIconClass()}`}
        isSyncing={syncStatus === 'syncing'}
      />
      <span>{getSyncText()}</span>
      {syncStatus === 'conflict' && (
        <ConflictBadge data-testid="conflict-badge">!</ConflictBadge>
      )}
    </SyncContainer>
  );
};

interface DraftConflictResolverProps {
  localDraft: Draft;
  remoteDraft: Draft;
  onResolve: (resolvedDraft: Draft) => void;
  onCancel: () => void;
}

const ConflictResolverContainer = styled.div`
    position: fixed;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    background: var(--center-channel-bg);
    border: 1px solid rgba(var(--center-channel-text-rgb), 0.16);
    border-radius: 8px;
    padding: 24px;
    max-width: 600px;
    width: 90%;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
    z-index: 1000;
`;

const ConflictTitle = styled.h3`
    margin: 0 0 16px 0;
    font-size: 18px;
    font-weight: 600;
    color: rgba(var(--center-channel-text-rgb), 1);
`;

const ConflictDescription = styled.p`
    margin: 0 0 24px 0;
    font-size: 14px;
    color: rgba(var(--center-channel-text-rgb), 0.72);
`;

const DraftPreviewContainer = styled.div`
    display: flex;
    gap: 16px;
    margin-bottom: 24px;
`;

const DraftPreview = styled.div<{ isSelected: boolean }>`
    flex: 1;
    padding: 16px;
    border: 2px solid ${props => props.isSelected ? 'rgba(var(--button-bg-rgb), 1)' : 'rgba(var(--center-channel-text-rgb), 0.16)'};
    border-radius: 4px;
    cursor: pointer;
    background: ${props => props.isSelected ? 'rgba(var(--button-bg-rgb), 0.08)' : 'transparent'};
    transition: all 0.2s ease;

    &:hover {
        border-color: rgba(var(--button-bg-rgb), 0.64);
    }
`;

const PreviewLabel = styled.div`
    font-size: 12px;
    font-weight: 600;
    color: rgba(var(--center-channel-text-rgb), 0.64);
    margin-bottom: 8px;
    text-transform: uppercase;
`;

const PreviewContent = styled.div`
    font-size: 14px;
    color: rgba(var(--center-channel-text-rgb), 0.88);
    white-space: pre-wrap;
    word-break: break-word;
    max-height: 200px;
    overflow-y: auto;
`;

const PreviewTimestamp = styled.div`
    font-size: 11px;
    color: rgba(var(--center-channel-text-rgb), 0.56);
    margin-top: 8px;
`;

const ConflictActions = styled.div`
    display: flex;
    justify-content: flex-end;
    gap: 12px;
`;

const ConflictButton = styled.button<{ variant: 'primary' | 'secondary' }>`
    padding: 8px 16px;
    border-radius: 4px;
    font-size: 14px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s ease;
    
    ${props => props.variant === 'primary' ? `
        background: rgba(var(--button-bg-rgb), 1);
        color: var(--button-color);
        border: none;

        &:hover {
            background: rgba(var(--button-bg-rgb), 0.88);
        }
    ` : `
        background: transparent;
        color: rgba(var(--button-bg-rgb), 1);
        border: 1px solid rgba(var(--button-bg-rgb), 1);

        &:hover {
            background: rgba(var(--button-bg-rgb), 0.08);
        }
    `}

    &:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }
`;

export const DraftConflictResolver: React.FC<DraftConflictResolverProps> = ({
  localDraft,
  remoteDraft,
  onResolve,
  onCancel,
}) => {
  const [selectedDraft, setSelectedDraft] = useState<'local' | 'remote' | 'merge'>('local');

  const handleResolve = useCallback(() => {
    let resolvedDraft: Draft;

    if (selectedDraft === 'local') {
      resolvedDraft = { ...localDraft };
    } else if (selectedDraft === 'remote') {
      resolvedDraft = { ...remoteDraft };
    } else {
      // Merge strategy: combine both messages
      resolvedDraft = {
        ...localDraft,
        message: `${localDraft.message}\n\n--- Remote changes ---\n${remoteDraft.message}`,
        props: {
          ...localDraft.props,
          ...remoteDraft.props,
          conflict_resolved: true,
          conflict_resolution_strategy: 'merge',
        },
      };
    }

    // Clear conflict metadata
    if (resolvedDraft.props) {
      delete resolvedDraft.props.conflict_id;
    }

    onResolve(resolvedDraft);
  }, [selectedDraft, localDraft, remoteDraft, onResolve]);

  const formatTimestamp = (timestamp: number) => {
    const date = new Date(timestamp);
    return date.toLocaleString();
  };

  return (
    <ConflictResolverContainer data-testid="draft-conflict-resolver">
      <ConflictTitle>Draft Sync Conflict</ConflictTitle>
      <ConflictDescription>
        This draft was edited on multiple devices. Choose which version to keep or merge both.
      </ConflictDescription>

      <DraftPreviewContainer>
        <DraftPreview
          isSelected={selectedDraft === 'local'}
          onClick={() => setSelectedDraft('local')}
          data-testid="local-draft-preview"
        >
          <PreviewLabel>This Device</PreviewLabel>
          <PreviewContent>{localDraft.message || '(empty)'}</PreviewContent>
          <PreviewTimestamp>
            Last updated: {formatTimestamp(localDraft.update_at)}
          </PreviewTimestamp>
        </DraftPreview>

        <DraftPreview
          isSelected={selectedDraft === 'remote'}
          onClick={() => setSelectedDraft('remote')}
          data-testid="remote-draft-preview"
        >
          <PreviewLabel>Other Device</PreviewLabel>
          <PreviewContent>{remoteDraft.message || '(empty)'}</PreviewContent>
          <PreviewTimestamp>
            Last updated: {formatTimestamp(remoteDraft.update_at)}
          </PreviewTimestamp>
        </DraftPreview>
      </DraftPreviewContainer>

      <DraftPreview
        isSelected={selectedDraft === 'merge'}
        onClick={() => setSelectedDraft('merge')}
        data-testid="merge-draft-preview"
      >
        <PreviewLabel>Merge Both (Advanced)</PreviewLabel>
        <PreviewContent>
          {localDraft.message || '(empty)'}
          {'\n\n--- Remote changes ---\n'}
          {remoteDraft.message || '(empty)'}
        </PreviewContent>
      </DraftPreview>

      <ConflictActions>
        <ConflictButton
          variant="secondary"
          onClick={onCancel}
          data-testid="cancel-conflict-resolution"
        >
          Cancel
        </ConflictButton>
        <ConflictButton
          variant="primary"
          onClick={handleResolve}
          data-testid="resolve-conflict"
        >
          Use Selected Version
        </ConflictButton>
      </ConflictActions>
    </ConflictResolverContainer>
  );
};

export default DraftSyncIndicator;

