// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback} from 'react';
import {FormattedMessage} from 'react-intl';
import styled from 'styled-components';

import type {SearchRankingOptions} from '@mattermost/types/posts';

import Checkbox from 'components/checkbox';
import Slider from 'components/slider';

const RankingContainer = styled.div`
    display: flex;
    flex-direction: column;
    gap: 16px;
`;

const OptionRow = styled.div`
    display: flex;
    align-items: center;
    gap: 12px;
`;

const OptionLabel = styled.label`
    flex: 1;
    font-size: 14px;
    color: var(--center-channel-color);
    cursor: pointer;
`;

const OptionDescription = styled.div`
    font-size: 12px;
    color: rgba(var(--center-channel-color-rgb), 0.64);
    margin-top: 4px;
`;

const WeightAdjuster = styled.div`
    margin-top: 16px;
    padding: 16px;
    background: rgba(var(--center-channel-color-rgb), 0.04);
    border-radius: 4px;
`;

const WeightLabel = styled.div`
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 8px;
    font-size: 13px;
    color: var(--center-channel-color);
`;

const WeightValue = styled.span`
    font-weight: 600;
    color: var(--button-bg);
`;

interface Props {
    options: SearchRankingOptions;
    onChange: (options: SearchRankingOptions) => void;
}

const RankingOptions: React.FC<Props> = ({options, onChange}) => {
    const handleToggleRelevance = useCallback(() => {
        onChange({
            ...options,
            useRelevanceScoring: !options.useRelevanceScoring,
        });
    }, [options, onChange]);

    const handleToggleRecent = useCallback(() => {
        onChange({
            ...options,
            boostRecent: !options.boostRecent,
        });
    }, [options, onChange]);

    const handleToggleReactions = useCallback(() => {
        onChange({
            ...options,
            boostReactions: !options.boostReactions,
        });
    }, [options, onChange]);

    const handleWeightChange = useCallback((factor: string, weight: number) => {
        const customWeights = {...(options.customWeights || {})};
        customWeights[factor] = weight;
        onChange({
            ...options,
            customWeights,
        });
    }, [options, onChange]);

    return (
        <RankingContainer>
            <OptionRow>
                <Checkbox
                    id='relevance-scoring'
                    checked={options.useRelevanceScoring}
                    onChange={handleToggleRelevance}
                />
                <OptionLabel htmlFor='relevance-scoring'>
                    <FormattedMessage
                        id='ranking_options.relevance.label'
                        defaultMessage='Enable Relevance Scoring'
                    />
                    <OptionDescription>
                        <FormattedMessage
                            id='ranking_options.relevance.description'
                            defaultMessage='Use machine learning to rank results by relevance instead of chronological order'
                        />
                    </OptionDescription>
                </OptionLabel>
            </OptionRow>

            {options.useRelevanceScoring && (
                <>
                    <OptionRow>
                        <Checkbox
                            id='boost-recent'
                            checked={options.boostRecent}
                            onChange={handleToggleRecent}
                        />
                        <OptionLabel htmlFor='boost-recent'>
                            <FormattedMessage
                                id='ranking_options.recent.label'
                                defaultMessage='Boost Recent Posts'
                            />
                            <OptionDescription>
                                <FormattedMessage
                                    id='ranking_options.recent.description'
                                    defaultMessage='Give higher scores to recently created posts'
                                />
                            </OptionDescription>
                        </OptionLabel>
                    </OptionRow>

                    <OptionRow>
                        <Checkbox
                            id='boost-reactions'
                            checked={options.boostReactions}
                            onChange={handleToggleReactions}
                        />
                        <OptionLabel htmlFor='boost-reactions'>
                            <FormattedMessage
                                id='ranking_options.reactions.label'
                                defaultMessage='Boost Posts with Reactions'
                            />
                            <OptionDescription>
                                <FormattedMessage
                                    id='ranking_options.reactions.description'
                                    defaultMessage='Prioritize posts that have received emoji reactions'
                                />
                            </OptionDescription>
                        </OptionLabel>
                    </OptionRow>

                    <WeightAdjuster>
                        <WeightLabel>
                            <span>
                                <FormattedMessage
                                    id='ranking_options.weight.text_match'
                                    defaultMessage='Text Match Weight'
                                />
                            </span>
                            <WeightValue>
                                {(options.customWeights?.text_match || 3.0).toFixed(1)}
                            </WeightValue>
                        </WeightLabel>
                        <Slider
                            min={0}
                            max={5}
                            step={0.1}
                            value={options.customWeights?.text_match || 3.0}
                            onChange={(value) => handleWeightChange('text_match', value)}
                        />
                    </WeightAdjuster>

                    <WeightAdjuster>
                        <WeightLabel>
                            <span>
                                <FormattedMessage
                                    id='ranking_options.weight.recency'
                                    defaultMessage='Recency Weight'
                                />
                            </span>
                            <WeightValue>
                                {(options.customWeights?.recency || 1.5).toFixed(1)}
                            </WeightValue>
                        </WeightLabel>
                        <Slider
                            min={0}
                            max={3}
                            step={0.1}
                            value={options.customWeights?.recency || 1.5}
                            onChange={(value) => handleWeightChange('recency', value)}
                        />
                    </WeightAdjuster>

                    <WeightAdjuster>
                        <WeightLabel>
                            <span>
                                <FormattedMessage
                                    id='ranking_options.weight.engagement'
                                    defaultMessage='Engagement Weight'
                                />
                            </span>
                            <WeightValue>
                                {(options.customWeights?.engagement || 1.0).toFixed(1)}
                            </WeightValue>
                        </WeightLabel>
                        <Slider
                            min={0}
                            max={3}
                            step={0.1}
                            value={options.customWeights?.engagement || 1.0}
                            onChange={(value) => handleWeightChange('engagement', value)}
                        />
                    </WeightAdjuster>
                </>
            )}
        </RankingContainer>
    );
};

export default RankingOptions;

