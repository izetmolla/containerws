"use client"

import type { Column } from "@tanstack/react-table"
import * as React from "react"

import { ReactSelect } from "@/components/ui/reactselect"

import type { Option } from "../types/data-table"

type SelectOption = { label: string; value: string }

interface DataTableReactSelectFilterProps<TData, TValue> {
  column?: Column<TData, TValue>
  title?: string
  options: Option[]
  multiple?: boolean
  disabled?: boolean
}

function toSelectOptions(options: Option[]): SelectOption[] {
  return options.map((o) => ({
    label: o.label,
    value: String(o.value),
  }))
}

export function DataTableReactSelectFilter<TData, TValue>({
  column,
  title,
  options,
  multiple,
  disabled,
}: DataTableReactSelectFilterProps<TData, TValue>) {
  const selectOptions = React.useMemo(
    () => toSelectOptions(options),
    [options],
  )

  const columnFilterValue = column?.getFilterValue()
  const selected = React.useMemo(() => {
    if (Array.isArray(columnFilterValue)) {
      return columnFilterValue.map(String)
    }
    if (columnFilterValue == null || columnFilterValue === "") return []
    return [String(columnFilterValue)]
  }, [columnFilterValue])

  const placeholder = title ? `Filter ${title.toLowerCase()}…` : "Filter…"

  if (multiple) {
    return (
      <div className="min-w-[12rem] max-w-[18rem] flex-1">
        <ReactSelect<SelectOption, true>
          size="sm"
          isMulti
          isSearchable
          isClearable
          isDisabled={disabled}
          options={selectOptions}
          value={selected}
          placeholder={placeholder}
          closeMenuOnSelect={false}
          hideSelectedOptions={false}
          noOptionsMessage={() => "No options"}
          onValueChange={(values) => {
            column?.setFilterValue(values.length ? values : undefined)
          }}
        />
      </div>
    )
  }

  return (
    <div className="w-[12rem]">
      <ReactSelect<SelectOption, false>
        size="sm"
        isSearchable
        isClearable
        isDisabled={disabled}
        options={selectOptions}
        value={selected[0] ?? null}
        placeholder={placeholder}
        noOptionsMessage={() => "No options"}
        onValueChange={(value) => {
          column?.setFilterValue(value ? [value] : undefined)
        }}
      />
    </div>
  )
}
