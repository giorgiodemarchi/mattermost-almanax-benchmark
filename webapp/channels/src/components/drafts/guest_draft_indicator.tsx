// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, { useCallback, useEffect, useState } from 'react';
import { useSelector, useDispatch } from 'react-redux';
import styled from 'styled-components';

import type { Channel } from '@mattermost/types/channels';
import type { Draft } from '@mattermost/types/drafts';

import { getCurrentUserId, isCurrentUserGuest } from 'mattermost-redux/selectors/entities/users';
import { getChannel } from 'mattermost-redux/selectors/entities/channels';

import { Client4 } from 'mattermost-redux/client';

const GuestDraftContainer = styled.div`
    display: flex;
    align-items: center;
    padding: 8px 12px;
    background: rgba(var(--dnd-indicator-rgb), 0.08);
    border-radius: 4px;
    margin: 8px 0;
    font-size: 12px;
    color: rgba(var(--center-channel-text-rgb), 0.64);
`;

const GuestDraftIcon = styled.i`
    margin-right: 8px;
    font-size: 14px;
    color: rgba(var(--dnd-indicator-rgb), 1);
`;

const GuestDraftText = styled.span`
    flex: 1;
`;

const GuestDraftActions = styled.div`
    display: flex;
    gap: 8px;
`;

const GuestDraftButton = styled.button`
    padding: 4px 8px;
    background: transparent;
    border: 1px solid rgba(var(--center-channel-text-rgb), 0.16);
    border-radius: 4px;
    color: rgba(var(--center-channel-text-rgb), 0.72);
    cursor: pointer;
    font-size: 11px;

    &:hover {
        background: rgba(var(--center-channel-text-rgb), 0.08);
        border-color: rgba(var(--center-channel-text-rgb), 0.32);
    }
`;

interface GuestDraftIndicatorProps {
  channelId: string;
  draft?: Draft;
  onDraftLoad?: (draft: Draft) => void;
}

export const GuestDraftIndicator: React.FC<GuestDraftIndicatorProps> = ({
  channelId,
  draft: propDraft,
  onDraftLoad,
}) => {
  const dispatch = useDispatch();
  const currentUserId = useSelector(getCurrentUserId);
  const isGuest = useSelector(isCurrentUserGuest);
  const channel = useSelector((state) => getChannel(state, channelId));

  const [draft, setDraft] = useState<Draft | undefined>(propDraft);
  const [isLoading, setIsLoading] = useState(false);

  useEffect(() => {
    if (propDraft) {
      setDraft(propDraft);
    }
  }, [propDraft]);

  const handleLoadDraft = useCallback(async () => {
    if (!draft) {
      return;
    }

    setIsLoading(true);
    try {
      if (onDraftLoad) {
        onDraftLoad(draft);
      }
    } finally {
      setIsLoading(false);
    }
  }, [draft, onDraftLoad]);

  const handleDeleteDraft = useCallback(async () => {
    if (!draft) {
      return;
    }

    setIsLoading(true);
    try {
      await Client4.deleteDraft(draft.channel_id, draft.root_id || '');
      setDraft(undefined);
    } catch (error) {
      console.error('Failed to delete guest draft:', error);
    } finally {
      setIsLoading(false);
    }
  }, [draft]);

  // Only show for guest users with drafts
  if (!isGuest || !draft || !draft.is_guest) {
    return null;
  }

  const channelName = draft.props?.channel_name || channel?.display_name || 'Unknown Channel';
  const draftPreview = draft.message.length > 50
    ? draft.message.substring(0, 50) + '...'
    : draft.message;

  return (
    <GuestDraftContainer data-testid="guest-draft-indicator">
      <GuestDraftIcon className="icon icon-pencil-outline" />
      <GuestDraftText>
        {`Draft in ${channelName}: "${draftPreview}"`}
      </GuestDraftText>
      <GuestDraftActions>
        <GuestDraftButton
          onClick={handleLoadDraft}
          disabled={isLoading}
          data-testid="load-guest-draft"
        >
          Load
        </GuestDraftButton>
        <GuestDraftButton
          onClick={handleDeleteDraft}
          disabled={isLoading}
          data-testid="delete-guest-draft"
        >
          Delete
        </GuestDraftButton>
      </GuestDraftActions>
    </GuestDraftContainer>
  );
};

export default GuestDraftIndicator;

