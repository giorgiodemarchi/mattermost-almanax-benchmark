// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useState, useMemo, useEffect} from 'react';
import {useDispatch, useSelector} from 'react-redux';
import {FormattedMessage, useIntl} from 'react-intl';
import styled from 'styled-components';

import type {CustomFieldFilter, SearchParams, SearchRankingOptions} from '@mattermost/types/posts';

import {executeAdvancedSearch, updateSearchParams, clearSearchResults} from 'actions/views/search';
import {getCurrentTeamId} from 'selectors/teams';
import {getSearchParams, getSearchResults} from 'selectors/search';

import Input from 'components/input';
import Button from 'components/button';
import FilterBuilder from './filter_builder';
import RankingOptions from './ranking_options';
import SearchResults from './search_results';

const AdvancedSearchContainer = styled.div`
    display: flex;
    flex-direction: column;
    height: 100%;
    background: var(--center-channel-bg);
    padding: 20px;
`;

const SearchHeader = styled.div`
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 24px;
    padding-bottom: 16px;
    border-bottom: 1px solid rgba(var(--center-channel-color-rgb), 0.12);
`;

const SearchTitle = styled.h2`
    font-size: 24px;
    font-weight: 600;
    color: var(--center-channel-color);
    margin: 0;
`;

const SearchForm = styled.div`
    display: flex;
    flex-direction: column;
    gap: 16px;
    margin-bottom: 24px;
`;

const SearchRow = styled.div`
    display: flex;
    gap: 12px;
    align-items: flex-end;
`;

const SearchInputWrapper = styled.div`
    flex: 1;
`;

const ButtonGroup = styled.div`
    display: flex;
    gap: 8px;
`;

const ExpandableSection = styled.div<{expanded: boolean}>`
    background: rgba(var(--center-channel-color-rgb), 0.04);
    border-radius: 8px;
    padding: 16px;
    margin-bottom: 16px;
    transition: all 0.2s ease;
    
    ${(props) => !props.expanded && `
        padding: 12px 16px;
    `}
`;

const SectionHeader = styled.div`
    display: flex;
    align-items: center;
    justify-content: space-between;
    cursor: pointer;
    user-select: none;
`;

const SectionTitle = styled.h3`
    font-size: 16px;
    font-weight: 600;
    color: var(--center-channel-color);
    margin: 0;
`;

const SectionContent = styled.div<{expanded: boolean}>`
    margin-top: ${(props) => (props.expanded ? '16px' : '0')};
    display: ${(props) => (props.expanded ? 'block' : 'none')};
`;

const StatsBar = styled.div`
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 16px;
    background: rgba(var(--button-bg-rgb), 0.08);
    border-radius: 6px;
    margin-bottom: 16px;
`;

const StatItem = styled.div`
    display: flex;
    flex-direction: column;
    gap: 4px;
`;

const StatLabel = styled.span`
    font-size: 12px;
    color: rgba(var(--center-channel-color-rgb), 0.64);
    text-transform: uppercase;
    font-weight: 500;
`;

const StatValue = styled.span`
    font-size: 18px;
    font-weight: 600;
    color: var(--center-channel-color);
`;

const AdvancedSearchPanel: React.FC = () => {
    const intl = useIntl();
    const dispatch = useDispatch();
    
    const teamId = useSelector(getCurrentTeamId);
    const searchParams = useSelector(getSearchParams);
    const searchResults = useSelector(getSearchResults);

    const [searchTerm, setSearchTerm] = useState('');
    const [customFilters, setCustomFilters] = useState<CustomFieldFilter[]>([]);
    const [rankingOptions, setRankingOptions] = useState<SearchRankingOptions>({
        useRelevanceScoring: true,
        boostRecent: true,
        boostReactions: true,
        customWeights: {},
    });
    
    const [filtersExpanded, setFiltersExpanded] = useState(false);
    const [rankingExpanded, setRankingExpanded] = useState(false);
    const [isSearching, setIsSearching] = useState(false);

    // Auto-expand sections if they contain values
    useEffect(() => {
        if (customFilters.length > 0) {
            setFiltersExpanded(true);
        }
    }, [customFilters.length]);

    const handleSearch = useCallback(async () => {
        if (!searchTerm.trim() && customFilters.length === 0) {
            return;
        }

        setIsSearching(true);

        const params: SearchParams = {
            terms: searchTerm,
            customFieldFilters: customFilters,
            rankingOptions: rankingOptions,
            inChannels: searchParams?.inChannels || [],
            fromUsers: searchParams?.fromUsers || [],
        };

        try {
            await dispatch(executeAdvancedSearch(teamId, params));
        } catch (error) {
            console.error('Advanced search failed:', error);
        } finally {
            setIsSearching(false);
        }
    }, [dispatch, teamId, searchTerm, customFilters, rankingOptions, searchParams]);

    const handleClear = useCallback(() => {
        setSearchTerm('');
        setCustomFilters([]);
        setRankingOptions({
            useRelevanceScoring: true,
            boostRecent: true,
            boostReactions: true,
            customWeights: {},
        });
        dispatch(clearSearchResults());
    }, [dispatch]);

    const handleAddFilter = useCallback((filter: CustomFieldFilter) => {
        setCustomFilters((prev) => [...prev, filter]);
    }, []);

    const handleRemoveFilter = useCallback((index: number) => {
        setCustomFilters((prev) => prev.filter((_, i) => i !== index));
    }, []);

    const handleUpdateFilter = useCallback((index: number, filter: CustomFieldFilter) => {
        setCustomFilters((prev) => prev.map((f, i) => (i === index ? filter : f)));
    }, []);

    const handleKeyPress = useCallback((e: React.KeyboardEvent) => {
        if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
            handleSearch();
        }
    }, [handleSearch]);

    // Calculate search statistics
    const searchStats = useMemo(() => {
        if (!searchResults?.posts) {
            return {totalResults: 0, channels: 0, timeSpan: '0h'};
        }

        const posts = Object.values(searchResults.posts);
        const uniqueChannels = new Set(posts.map((p) => p.channel_id)).size;
        
        // Calculate time span
        if (posts.length > 0) {
            const timestamps = posts.map((p) => p.create_at);
            const oldest = Math.min(...timestamps);
            const newest = Math.max(...timestamps);
            const hours = Math.floor((newest - oldest) / (1000 * 3600));
            const timeSpan = hours > 24 ? `${Math.floor(hours / 24)}d` : `${hours}h`;
            
            return {
                totalResults: posts.length,
                channels: uniqueChannels,
                timeSpan,
            };
        }

        return {totalResults: 0, channels: 0, timeSpan: '0h'};
    }, [searchResults]);

    return (
        <AdvancedSearchContainer>
            <SearchHeader>
                <SearchTitle>
                    <FormattedMessage
                        id='advanced_search.title'
                        defaultMessage='Advanced Search'
                    />
                </SearchTitle>
                <ButtonGroup>
                    <Button
                        onClick={handleClear}
                        variant='tertiary'
                        size='small'
                    >
                        <FormattedMessage
                            id='advanced_search.clear'
                            defaultMessage='Clear'
                        />
                    </Button>
                </ButtonGroup>
            </SearchHeader>

            <SearchForm>
                <SearchRow>
                    <SearchInputWrapper>
                        <Input
                            type='text'
                            placeholder={intl.formatMessage({
                                id: 'advanced_search.placeholder',
                                defaultMessage: 'Search messages...',
                            })}
                            value={searchTerm}
                            onChange={(e) => setSearchTerm(e.target.value)}
                            onKeyPress={handleKeyPress}
                            autoFocus={true}
                        />
                    </SearchInputWrapper>
                    <Button
                        onClick={handleSearch}
                        disabled={isSearching || (!searchTerm.trim() && customFilters.length === 0)}
                        loading={isSearching}
                    >
                        <FormattedMessage
                            id='advanced_search.search'
                            defaultMessage='Search'
                        />
                    </Button>
                </SearchRow>

                <ExpandableSection expanded={filtersExpanded}>
                    <SectionHeader onClick={() => setFiltersExpanded(!filtersExpanded)}>
                        <SectionTitle>
                            <FormattedMessage
                                id='advanced_search.filters'
                                defaultMessage='Custom Field Filters'
                            />
                            {customFilters.length > 0 && ` (${customFilters.length})`}
                        </SectionTitle>
                        <i className={`icon icon-chevron-${filtersExpanded ? 'up' : 'down'}`}/>
                    </SectionHeader>
                    <SectionContent expanded={filtersExpanded}>
                        <FilterBuilder
                            filters={customFilters}
                            onAddFilter={handleAddFilter}
                            onRemoveFilter={handleRemoveFilter}
                            onUpdateFilter={handleUpdateFilter}
                        />
                    </SectionContent>
                </ExpandableSection>

                <ExpandableSection expanded={rankingExpanded}>
                    <SectionHeader onClick={() => setRankingExpanded(!rankingExpanded)}>
                        <SectionTitle>
                            <FormattedMessage
                                id='advanced_search.ranking'
                                defaultMessage='Ranking & Relevance'
                            />
                        </SectionTitle>
                        <i className={`icon icon-chevron-${rankingExpanded ? 'up' : 'down'}`}/>
                    </SectionHeader>
                    <SectionContent expanded={rankingExpanded}>
                        <RankingOptions
                            options={rankingOptions}
                            onChange={setRankingOptions}
                        />
                    </SectionContent>
                </ExpandableSection>
            </SearchForm>

            {searchResults && searchStats.totalResults > 0 && (
                <StatsBar>
                    <StatItem>
                        <StatLabel>
                            <FormattedMessage
                                id='advanced_search.stats.results'
                                defaultMessage='Results'
                            />
                        </StatLabel>
                        <StatValue>{searchStats.totalResults}</StatValue>
                    </StatItem>
                    <StatItem>
                        <StatLabel>
                            <FormattedMessage
                                id='advanced_search.stats.channels'
                                defaultMessage='Channels'
                            />
                        </StatLabel>
                        <StatValue>{searchStats.channels}</StatValue>
                    </StatItem>
                    <StatItem>
                        <StatLabel>
                            <FormattedMessage
                                id='advanced_search.stats.timespan'
                                defaultMessage='Time Span'
                            />
                        </StatLabel>
                        <StatValue>{searchStats.timeSpan}</StatValue>
                    </StatItem>
                </StatsBar>
            )}

            <SearchResults
                results={searchResults}
                searchTerm={searchTerm}
                isLoading={isSearching}
            />
        </AdvancedSearchContainer>
    );
};

export default AdvancedSearchPanel;

