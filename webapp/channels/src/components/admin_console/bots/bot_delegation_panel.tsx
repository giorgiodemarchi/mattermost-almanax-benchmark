// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState, useEffect, useCallback} from 'react';
import {FormattedMessage} from 'react-intl';

import {Bot, BotDelegationRequest} from '@mattermost/types/bots';

import {Client4} from 'mattermost-redux/client';

import LoadingScreen from 'components/loading_screen';
import FormattedMarkdownMessage from 'components/formatted_markdown_message';

interface Props {
    bot: Bot;
    onClose: () => void;
}

const BotDelegationPanel: React.FC<Props> = ({bot, onClose}) => {
    const [delegatedBots, setDelegatedBots] = useState<Bot[]>([]);
    const [loading, setLoading] = useState(true);
    const [creating, setCreating] = useState(false);
    const [showCreateForm, setShowCreateForm] = useState(false);

    // Form state for creating delegated bots
    const [username, setUsername] = useState('');
    const [displayName, setDisplayName] = useState('');
    const [description, setDescription] = useState('');
    const [delegationType, setDelegationType] = useState('subbot');
    const [scopes, setScopes] = useState<string[]>([]);

    const loadDelegatedBots = useCallback(async () => {
        try {
            setLoading(true);
            const bots = await Client4.getDelegatedBots(bot.user_id);
            setDelegatedBots(bots);
        } catch (error) {
            // Error loading bots
            console.error('Failed to load delegated bots:', error);
        } finally {
            setLoading(false);
        }
    }, [bot.user_id]);

    useEffect(() => {
        loadDelegatedBots();
    }, [loadDelegatedBots]);

    const handleCreateDelegatedBot = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!username) {
            return;
        }

        setCreating(true);
        try {
            const request: BotDelegationRequest = {
                parent_bot_id: bot.user_id,
                username,
                display_name: displayName,
                description,
                delegation_type: delegationType,
                scopes,
            };

            await Client4.createDelegatedBot(bot.user_id, request);

            // Reset form and reload bots
            setUsername('');
            setDisplayName('');
            setDescription('');
            setScopes([]);
            setShowCreateForm(false);
            await loadDelegatedBots();
        } catch (error) {
            console.error('Failed to create delegated bot:', error);
        } finally {
            setCreating(false);
        }
    };

    const handleScopeChange = (scope: string, checked: boolean) => {
        if (checked) {
            setScopes([...scopes, scope]);
        } else {
            setScopes(scopes.filter((s) => s !== scope));
        }
    };

    if (loading) {
        return <LoadingScreen/>;
    }

    return (
        <div className='bot-delegation-panel'>
            <div className='panel-header'>
                <h3>
                    <FormattedMessage
                        id='admin.bots.delegation.title'
                        defaultMessage='Manage Delegated Bots'
                    />
                </h3>
                <p>
                    <FormattedMarkdownMessage
                        id='admin.bots.delegation.description'
                        defaultMessage='Create and manage sub-bots for **{botName}**. Delegated bots inherit permissions from their parent and can be used for microservice architectures.'
                        values={{botName: bot.display_name || bot.username}}
                    />
                </p>
            </div>

            <div className='delegated-bots-list'>
                <h4>
                    <FormattedMessage
                        id='admin.bots.delegation.existing'
                        defaultMessage='Existing Delegated Bots ({count})'
                        values={{count: delegatedBots.length}}
                    />
                </h4>

                {delegatedBots.length === 0 ? (
                    <p className='empty-state'>
                        <FormattedMessage
                            id='admin.bots.delegation.no_bots'
                            defaultMessage='No delegated bots created yet.'
                        />
                    </p>
                ) : (
                    <ul className='bot-list'>
                        {delegatedBots.map((delegatedBot) => (
                            <li
                                key={delegatedBot.user_id}
                                className='bot-item'
                            >
                                <div className='bot-info'>
                                    <strong>{delegatedBot.display_name || delegatedBot.username}</strong>
                                    <span className='bot-type-badge'>
                                        {delegatedBot.delegation_type}
                                    </span>
                                    {delegatedBot.is_system_managed && (
                                        <span className='system-managed-badge'>
                                            <FormattedMessage
                                                id='admin.bots.system_managed'
                                                defaultMessage='System Managed'
                                            />
                                        </span>
                                    )}
                                </div>
                                <div className='bot-description'>
                                    {delegatedBot.description}
                                </div>
                            </li>
                        ))}
                    </ul>
                )}
            </div>

            {!showCreateForm && (
                <button
                    className='btn btn-primary'
                    onClick={() => setShowCreateForm(true)}
                >
                    <FormattedMessage
                        id='admin.bots.delegation.create'
                        defaultMessage='Create Delegated Bot'
                    />
                </button>
            )}

            {showCreateForm && (
                <form
                    className='create-delegated-bot-form'
                    onSubmit={handleCreateDelegatedBot}
                >
                    <h4>
                        <FormattedMessage
                            id='admin.bots.delegation.create_new'
                            defaultMessage='Create New Delegated Bot'
                        />
                    </h4>

                    <div className='form-group'>
                        <label htmlFor='username'>
                            <FormattedMessage
                                id='admin.bots.username'
                                defaultMessage='Username'
                            />
                        </label>
                        <input
                            id='username'
                            type='text'
                            className='form-control'
                            value={username}
                            onChange={(e) => setUsername(e.target.value)}
                            required={true}
                        />
                    </div>

                    <div className='form-group'>
                        <label htmlFor='displayName'>
                            <FormattedMessage
                                id='admin.bots.display_name'
                                defaultMessage='Display Name'
                            />
                        </label>
                        <input
                            id='displayName'
                            type='text'
                            className='form-control'
                            value={displayName}
                            onChange={(e) => setDisplayName(e.target.value)}
                        />
                    </div>

                    <div className='form-group'>
                        <label htmlFor='description'>
                            <FormattedMessage
                                id='admin.bots.description'
                                defaultMessage='Description'
                            />
                        </label>
                        <textarea
                            id='description'
                            className='form-control'
                            value={description}
                            onChange={(e) => setDescription(e.target.value)}
                            rows={3}
                        />
                    </div>

                    <div className='form-group'>
                        <label htmlFor='delegationType'>
                            <FormattedMessage
                                id='admin.bots.delegation_type'
                                defaultMessage='Delegation Type'
                            />
                        </label>
                        <select
                            id='delegationType'
                            className='form-control'
                            value={delegationType}
                            onChange={(e) => setDelegationType(e.target.value)}
                        >
                            <option value='subbot'>Sub-Bot</option>
                            <option value='service'>Service Account</option>
                            <option value='api_client'>API Client</option>
                        </select>
                    </div>

                    <div className='form-group'>
                        <label>
                            <FormattedMessage
                                id='admin.bots.scopes'
                                defaultMessage='Scopes'
                            />
                        </label>
                        <div className='scopes-checkboxes'>
                            {['posts:read', 'posts:write', 'channels:read', 'channels:manage', 'users:read', 'teams:read'].map((scope) => (
                                <label
                                    key={scope}
                                    className='checkbox-inline'
                                >
                                    <input
                                        type='checkbox'
                                        checked={scopes.includes(scope)}
                                        onChange={(e) => handleScopeChange(scope, e.target.checked)}
                                    />
                                    {scope}
                                </label>
                            ))}
                        </div>
                    </div>

                    <div className='form-actions'>
                        <button
                            type='submit'
                            className='btn btn-primary'
                            disabled={creating || !username}
                        >
                            {creating ? (
                                <FormattedMessage
                                    id='admin.bots.creating'
                                    defaultMessage='Creating...'
                                />
                            ) : (
                                <FormattedMessage
                                    id='admin.bots.create'
                                    defaultMessage='Create'
                                />
                            )}
                        </button>
                        <button
                            type='button'
                            className='btn btn-link'
                            onClick={() => setShowCreateForm(false)}
                        >
                            <FormattedMessage
                                id='admin.bots.cancel'
                                defaultMessage='Cancel'
                            />
                        </button>
                    </div>
                </form>
            )}

            <button
                className='btn btn-link close-button'
                onClick={onClose}
            >
                <FormattedMessage
                    id='admin.bots.close'
                    defaultMessage='Close'
                />
            </button>
        </div>
    );
};

export default BotDelegationPanel;

