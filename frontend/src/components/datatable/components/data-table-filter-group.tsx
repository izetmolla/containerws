"use client";

import type { Column } from "@tanstack/react-table";
import { FolderTree, Trash2 } from "lucide-react";
import * as React from "react";

import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { SortableItem, SortableItemHandle } from "./ui/sortable";
import { dataTableConfig } from "../config/data-table";
import type { AdvancedFilterGroup } from "../lib/advanced-filter-types";
import { normalizeAdvancedFilterJoinOperators } from "../lib/advanced-filters";
import { getDefaultFilterOperator } from "../lib/data-table";
import { generateId } from "../lib/id";
import { cn } from "@/lib/utils";
import type {
  ExtendedColumnFilter,
  FilterOperator,
  JoinOperator,
} from "../types/data-table";

export interface DataTableFilterGroupProps<TData> {
  group: AdvancedFilterGroup;
  index: number;
  groupItemId: string;
  defaultJoinOperator: JoinOperator;
  columns: Column<TData>[];
  onGroupUpdate: (
    groupId: string,
    updates: Partial<Omit<AdvancedFilterGroup, "groupId" | "type">>,
  ) => void;
  onGroupRemove: (groupId: string) => void;
  onFilterAddToGroup: (groupId: string, filter: ExtendedColumnFilter<TData>) => void;
  renderFilterItem: (props: {
    filter: ExtendedColumnFilter<TData>;
    index: number;
    filterItemId: string;
    nested?: boolean;
  }) => React.ReactNode;
}

export function DataTableFilterGroup<TData>({
  group,
  index,
  groupItemId,
  defaultJoinOperator,
  columns,
  onGroupUpdate,
  onGroupRemove,
  onFilterAddToGroup,
  renderFilterItem,
}: DataTableFilterGroupProps<TData>) {
  const joinOperatorListboxId = `${groupItemId}-join-operator-listbox`;
  const normalizedFilters = React.useMemo(
    () => normalizeAdvancedFilterJoinOperators(group.filters, defaultJoinOperator),
    [group.filters, defaultJoinOperator],
  );

  const onAddFilter = React.useCallback(() => {
    const column = columns[0];
    if (!column) return;

    const variant = column.columnDef.meta?.variant ?? "text";
    onFilterAddToGroup(group.groupId, {
      id: column.id as Extract<keyof TData, string>,
      value: "",
      variant,
      operator: getDefaultFilterOperator(variant) as FilterOperator,
      filterId: generateId({ length: 8 }),
      ...(group.filters.length > 0 && { joinOperator: "and" as JoinOperator }),
    });
  }, [columns, group.filters.length, group.groupId, onFilterAddToGroup]);

  return (
    <SortableItem value={group.groupId} asChild>
      <div role="listitem" id={groupItemId} className="flex items-start gap-2">
        <div className="min-w-[72px] pt-1 text-center">
          {index === 0 ? (
            <span className="text-muted-foreground text-sm">Where</span>
          ) : (
            <Select
              value={group.joinOperator ?? defaultJoinOperator}
              onValueChange={(value: JoinOperator) =>
                onGroupUpdate(group.groupId, { joinOperator: value })
              }
            >
              <SelectTrigger
                aria-label="Select join operator for group"
                aria-controls={joinOperatorListboxId}
                className="h-8 rounded lowercase [&[data-size]]:h-8"
              >
                <SelectValue
                  placeholder={group.joinOperator ?? defaultJoinOperator}
                />
              </SelectTrigger>
              <SelectContent
                id={joinOperatorListboxId}
                position="popper"
                className="min-w-(--radix-select-trigger-width) lowercase"
              >
                {dataTableConfig.joinOperators.map((operator) => (
                  <SelectItem key={operator} value={operator}>
                    {operator}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        </div>

        <div
          className={cn(
            "flex min-w-0 flex-1 flex-col gap-2 rounded-lg border border-dashed border-border bg-muted/20 p-2",
          )}
        >
          <div className="flex items-center justify-between gap-2">
            <div className="flex items-center gap-1.5 text-muted-foreground text-xs font-medium uppercase tracking-wide">
              <FolderTree className="size-3.5" />
              Group
            </div>
            <div className="flex items-center gap-1">
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="h-7 rounded px-2 text-xs"
                onClick={onAddFilter}
              >
                Add filter
              </Button>
              <Button
                type="button"
                aria-label="Remove filter group"
                variant="outline"
                size="icon"
                className="size-7 rounded"
                onClick={() => onGroupRemove(group.groupId)}
              >
                <Trash2 className="size-3.5" />
              </Button>
              <SortableItemHandle asChild>
                <Button variant="outline" size="icon" className="size-7 rounded">
                  <span className="sr-only">Reorder group</span>
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    className="size-3.5"
                    aria-hidden
                  >
                    <circle cx="9" cy="5" r="1" />
                    <circle cx="9" cy="12" r="1" />
                    <circle cx="9" cy="19" r="1" />
                    <circle cx="15" cy="5" r="1" />
                    <circle cx="15" cy="12" r="1" />
                    <circle cx="15" cy="19" r="1" />
                  </svg>
                </Button>
              </SortableItemHandle>
            </div>
          </div>

          <div className="flex flex-col gap-2">
            {normalizedFilters.map((filter, filterIndex) =>
              renderFilterItem({
                filter: filter as ExtendedColumnFilter<TData>,
                index: filterIndex,
                filterItemId: `${groupItemId}-condition-${filter.filterId}`,
                nested: true,
              }),
            )}
          </div>
        </div>
      </div>
    </SortableItem>
  );
}
