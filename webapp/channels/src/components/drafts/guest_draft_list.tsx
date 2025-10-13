// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, { useCallback, useEffect, useState } from 'react';
import { useSelector, useDispatch } from 'react-redux';
import styled from 'styled-components';

import type { Draft } from '@mattermost/types/drafts';

import { getCurrentUserId, isCurrentUserGuest } from 'mattermost-redux/selectors/entities/users';
import { getChannel } from 'mattermost-redux/selectors/entities/channels';

import { Client4 } from 'mattermost-redux/client';

import GuestDraftIndicator from './guest_draft_indicator';
import { DraftSyncIndicator } from './draft_sync_indicator';

const GuestDraftListContainer = styled.div`
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 16px;
    max-height: 400px;
    overflow-y: auto;
`;

const ListHeader = styled.div`
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
`;

const ListTitle = styled.h4`
    margin: 0;
    font-size: 16px;
    font-weight: 600;
    color: rgba(var(--center-channel-text-rgb), 1);
`;

const DraftCount = styled.span`
    padding: 2px 8px;
    background: rgba(var(--button-bg-rgb), 0.12);
    color: rgba(var(--button-bg-rgb), 1);
    border-radius: 12px;
    font-size: 12px;
    font-weight: 600;
`;

const EmptyState = styled.div`
    text-align: center;
    padding: 32px 16px;
    color: rgba(var(--center-channel-text-rgb), 0.56);
    font-size: 14px;
`;

const EmptyStateIcon = styled.i`
    font-size: 48px;
    margin-bottom: 12px;
    opacity: 0.32;
`;

const DraftItem = styled.div`
    display: flex;
    flex-direction: column;
    padding: 12px;
    background: var(--center-channel-bg);
    border: 1px solid rgba(var(--center-channel-text-rgb), 0.12);
    border-radius: 4px;
    transition: all 0.2s ease;

    &:hover {
        border-color: rgba(var(--button-bg-rgb), 0.32);
        box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
    }
`;

const DraftHeader = styled.div`
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 8px;
`;

const ChannelName = styled.span`
    font-size: 14px;
    font-weight: 600;
    color: rgba(var(--center-channel-text-rgb), 0.88);
`;

const DraftTimestamp = styled.span`
    font-size: 11px;
    color: rgba(var(--center-channel-text-rgb), 0.56);
`;

const DraftContent = styled.div`
    font-size: 13px;
    color: rgba(var(--center-channel-text-rgb), 0.72);
    margin-bottom: 8px;
    white-space: pre-wrap;
    word-break: break-word;
    max-height: 60px;
    overflow: hidden;
    text-overflow: ellipsis;
`;

const DraftMetadata = styled.div`
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 11px;
    color: rgba(var(--center-channel-text-rgb), 0.56);
`;

const MetadataBadge = styled.span`
    padding: 2px 6px;
    background: rgba(var(--center-channel-text-rgb), 0.08);
    border-radius: 4px;
`;

const LoadingSpinner = styled.div`
    display: flex;
    justify-content: center;
    align-items: center;
    padding: 32px;
`;

interface GuestDraftListProps {
  teamId: string;
  onDraftSelect?: (draft: Draft) => void;
}

export const GuestDraftList: React.FC<GuestDraftListProps> = ({
  teamId,
  onDraftSelect,
}) => {
  const dispatch = useDispatch();
  const currentUserId = useSelector(getCurrentUserId);
  const isGuest = useSelector(isCurrentUserGuest);

  const [drafts, setDrafts] = useState<Draft[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadDrafts = useCallback(async () => {
    if (!isGuest) {
      return;
    }

    setIsLoading(true);
    setError(null);

    try {
      const response = await Client4.getDrafts(teamId);
      // Filter to only show guest drafts
      const guestDrafts = response.filter((draft: Draft) => draft.is_guest);
      setDrafts(guestDrafts);
    } catch (err) {
      setError('Failed to load drafts');
      console.error('Error loading guest drafts:', err);
    } finally {
      setIsLoading(false);
    }
  }, [teamId, isGuest]);

  useEffect(() => {
    loadDrafts();
  }, [loadDrafts]);

  const formatTimestamp = useCallback((timestamp: number) => {
    const now = Date.now();
    const diff = now - timestamp;

    if (diff < 60000) {
      return 'Just now';
    } else if (diff < 3600000) {
      const minutes = Math.floor(diff / 60000);
      return `${minutes} minute${minutes > 1 ? 's' : ''} ago`;
    } else if (diff < 86400000) {
      const hours = Math.floor(diff / 3600000);
      return `${hours} hour${hours > 1 ? 's' : ''} ago`;
    } else {
      const date = new Date(timestamp);
      return date.toLocaleDateString();
    }
  }, []);

  const handleDraftClick = useCallback((draft: Draft) => {
    if (onDraftSelect) {
      onDraftSelect(draft);
    }
  }, [onDraftSelect]);

  if (!isGuest) {
    return null;
  }

  if (isLoading) {
    return (
      <GuestDraftListContainer>
        <LoadingSpinner>
          <i className="icon icon-loading" />
        </LoadingSpinner>
      </GuestDraftListContainer>
    );
  }

  if (error) {
    return (
      <GuestDraftListContainer>
        <EmptyState>
          <EmptyStateIcon className="icon icon-alert-outline" />
          <div>{error}</div>
        </EmptyState>
      </GuestDraftListContainer>
    );
  }

  if (drafts.length === 0) {
    return (
      <GuestDraftListContainer>
        <EmptyState>
          <EmptyStateIcon className="icon icon-pencil-outline" />
          <div>No drafts yet</div>
          <div style={{ fontSize: '12px', marginTop: '8px' }}>
            Start typing in a channel to create a draft
          </div>
        </EmptyState>
      </GuestDraftListContainer>
    );
  }

  return (
    <GuestDraftListContainer data-testid="guest-draft-list">
      <ListHeader>
        <ListTitle>Your Drafts</ListTitle>
        <DraftCount>{drafts.length}</DraftCount>
      </ListHeader>

      {drafts.map((draft) => {
        const channel = useSelector((state) => getChannel(state, draft.channel_id));
        const channelDisplayName = channel?.display_name || draft.props?.channel_name || 'Unknown Channel';
        const hasFiles = draft.file_ids && draft.file_ids.length > 0;
        const hasPriority = draft.priority && Object.keys(draft.priority).length > 0;

        return (
          <DraftItem
            key={`${draft.channel_id}-${draft.root_id}`}
            onClick={() => handleDraftClick(draft)}
            data-testid={`guest-draft-item-${draft.channel_id}`}
          >
            <DraftHeader>
              <ChannelName>
                <i className="icon icon-pound" style={{ marginRight: '4px' }} />
                {channelDisplayName}
              </ChannelName>
              <DraftTimestamp>
                {formatTimestamp(draft.update_at)}
              </DraftTimestamp>
            </DraftHeader>

            <DraftContent>
              {draft.message || '(empty draft)'}
            </DraftContent>

            <DraftMetadata>
              {hasFiles && (
                <MetadataBadge>
                  <i className="icon icon-paperclip" style={{ marginRight: '4px' }} />
                  {draft.file_ids.length} file{draft.file_ids.length > 1 ? 's' : ''}
                </MetadataBadge>
              )}
              {hasPriority && (
                <MetadataBadge>
                  <i className="icon icon-alert-circle-outline" style={{ marginRight: '4px' }} />
                  Priority
                </MetadataBadge>
              )}
              {draft.root_id && (
                <MetadataBadge>
                  <i className="icon icon-reply" style={{ marginRight: '4px' }} />
                  Thread reply
                </MetadataBadge>
              )}
              <DraftSyncIndicator draft={draft} />
            </DraftMetadata>
          </DraftItem>
        );
      })}
    </GuestDraftListContainer>
  );
};

export default GuestDraftList;

