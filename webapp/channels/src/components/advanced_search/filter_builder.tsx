// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState, useCallback} from 'react';
import {FormattedMessage, useIntl} from 'react-intl';
import styled from 'styled-components';

import type {CustomFieldFilter} from '@mattermost/types/posts';

import Button from 'components/button';
import Input from 'components/input';
import Select from 'components/select';

const FilterContainer = styled.div`
    display: flex;
    flex-direction: column;
    gap: 12px;
`;

const FilterItem = styled.div`
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 12px;
    background: var(--center-channel-bg);
    border: 1px solid rgba(var(--center-channel-color-rgb), 0.16);
    border-radius: 4px;
`;

const FilterField = styled.div`
    flex: 1;
`;

const FilterOperator = styled.div`
    width: 120px;
`;

const FilterValue = styled.div`
    flex: 1;
`;

const FilterCombine = styled.div`
    width: 80px;
`;

const RemoveButton = styled.button`
    padding: 6px;
    background: transparent;
    border: none;
    color: rgba(var(--center-channel-color-rgb), 0.56);
    cursor: pointer;
    border-radius: 4px;
    
    &:hover {
        background: rgba(var(--error-text-rgb), 0.08);
        color: var(--error-text);
    }
`;

const AddButton = styled(Button)`
    margin-top: 8px;
    align-self: flex-start;
`;

const EmptyState = styled.div`
    padding: 24px;
    text-align: center;
    color: rgba(var(--center-channel-color-rgb), 0.64);
    font-size: 14px;
`;

interface Props {
    filters: CustomFieldFilter[];
    onAddFilter: (filter: CustomFieldFilter) => void;
    onRemoveFilter: (index: number) => void;
    onUpdateFilter: (index: number, filter: CustomFieldFilter) => void;
}

const OPERATORS = [
    {value: 'equals', label: 'Equals'},
    {value: 'contains', label: 'Contains'},
    {value: 'gt', label: 'Greater Than'},
    {value: 'lt', label: 'Less Than'},
    {value: 'in', label: 'In List'},
];

const COMBINE_OPERATORS = [
    {value: 'AND', label: 'AND'},
    {value: 'OR', label: 'OR'},
];

const FilterBuilder: React.FC<Props> = ({filters, onAddFilter, onRemoveFilter, onUpdateFilter}) => {
    const intl = useIntl();
    const [newFilter, setNewFilter] = useState<CustomFieldFilter>({
        fieldPath: '',
        operator: 'equals',
        value: '',
        combineOperator: 'AND',
    });

    const handleAddFilter = useCallback(() => {
        if (!newFilter.fieldPath || !newFilter.value) {
            return;
        }

        onAddFilter({...newFilter});
        setNewFilter({
            fieldPath: '',
            operator: 'equals',
            value: '',
            combineOperator: 'AND',
        });
    }, [newFilter, onAddFilter]);

    const handleUpdateFieldPath = useCallback((index: number, fieldPath: string) => {
        const filter = filters[index];
        onUpdateFilter(index, {...filter, fieldPath});
    }, [filters, onUpdateFilter]);

    const handleUpdateOperator = useCallback((index: number, operator: string) => {
        const filter = filters[index];
        onUpdateFilter(index, {...filter, operator});
    }, [filters, onUpdateFilter]);

    const handleUpdateValue = useCallback((index: number, value: string) => {
        const filter = filters[index];
        onUpdateFilter(index, {...filter, value});
    }, [filters, onUpdateFilter]);

    const handleUpdateCombine = useCallback((index: number, combineOperator: string) => {
        const filter = filters[index];
        onUpdateFilter(index, {...filter, combineOperator});
    }, [filters, onUpdateFilter]);

    return (
        <FilterContainer>
            {filters.length === 0 && (
                <EmptyState>
                    <FormattedMessage
                        id='filter_builder.empty'
                        defaultMessage='No custom field filters added yet. Use filters to search by metadata fields like priority, tags, or custom properties.'
                    />
                </EmptyState>
            )}

            {filters.map((filter, index) => (
                <FilterItem key={index}>
                    {index > 0 && (
                        <FilterCombine>
                            <Select
                                value={filter.combineOperator || 'AND'}
                                onChange={(e) => handleUpdateCombine(index, e.target.value)}
                                options={COMBINE_OPERATORS}
                            />
                        </FilterCombine>
                    )}
                    <FilterField>
                        <Input
                            type='text'
                            placeholder={intl.formatMessage({
                                id: 'filter_builder.field.placeholder',
                                defaultMessage: 'Field path (e.g., metadata.priority)',
                            })}
                            value={filter.fieldPath}
                            onChange={(e) => handleUpdateFieldPath(index, e.target.value)}
                        />
                    </FilterField>
                    <FilterOperator>
                        <Select
                            value={filter.operator}
                            onChange={(e) => handleUpdateOperator(index, e.target.value)}
                            options={OPERATORS}
                        />
                    </FilterOperator>
                    <FilterValue>
                        <Input
                            type='text'
                            placeholder={intl.formatMessage({
                                id: 'filter_builder.value.placeholder',
                                defaultMessage: 'Value',
                            })}
                            value={String(filter.value)}
                            onChange={(e) => handleUpdateValue(index, e.target.value)}
                        />
                    </FilterValue>
                    <RemoveButton
                        onClick={() => onRemoveFilter(index)}
                        aria-label={intl.formatMessage({
                            id: 'filter_builder.remove',
                            defaultMessage: 'Remove filter',
                        })}
                    >
                        <i className='icon icon-close'/>
                    </RemoveButton>
                </FilterItem>
            ))}

            <FilterItem>
                <FilterField>
                    <Input
                        type='text'
                        placeholder={intl.formatMessage({
                            id: 'filter_builder.field.placeholder',
                            defaultMessage: 'Field path (e.g., metadata.priority)',
                        })}
                        value={newFilter.fieldPath}
                        onChange={(e) => setNewFilter({...newFilter, fieldPath: e.target.value})}
                    />
                </FilterField>
                <FilterOperator>
                    <Select
                        value={newFilter.operator}
                        onChange={(e) => setNewFilter({...newFilter, operator: e.target.value})}
                        options={OPERATORS}
                    />
                </FilterOperator>
                <FilterValue>
                    <Input
                        type='text'
                        placeholder={intl.formatMessage({
                            id: 'filter_builder.value.placeholder',
                            defaultMessage: 'Value',
                        })}
                        value={String(newFilter.value)}
                        onChange={(e) => setNewFilter({...newFilter, value: e.target.value})}
                    />
                </FilterValue>
                <AddButton
                    onClick={handleAddFilter}
                    disabled={!newFilter.fieldPath || !newFilter.value}
                    size='small'
                >
                    <FormattedMessage
                        id='filter_builder.add'
                        defaultMessage='Add'
                    />
                </AddButton>
            </FilterItem>
        </FilterContainer>
    );
};

export default FilterBuilder;

