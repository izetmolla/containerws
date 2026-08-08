"use client"

import * as React from "react"
import RawAsyncSelect, { type AsyncProps } from "react-select/async"
import { mergeStyles, type Props as ReactSelectProps } from "react-select"
import {
  composeClassNames,
  composeComponents,
  extractValueFromChange,
  reactSelectClassNames,
  reactSelectStyles,
  resolveReactSelectMenuPortalTarget,
  resolveValue,
  type GroupBase,
  type ReactSelectKeyApi,
  type ReactSelectShadcnProps,
  type SelectInstance,
} from "./reactselect"

export type ReactSelectAsyncComponentProps<
  Option = unknown,
  IsMulti extends boolean = false,
  Group extends GroupBase<Option> = GroupBase<Option>,
> = Omit<AsyncProps<Option, IsMulti, Group>, "value"> &
  ReactSelectShadcnProps &
  ReactSelectKeyApi<Option, IsMulti> & {
    ref?: React.Ref<SelectInstance<Option, IsMulti, Group>>
  }

/**
 * shadcn-styled wrapper around `react-select/async`.
 *
 * All native props are forwarded (loadOptions, defaultOptions, cacheOptions,
 * onChange, isMulti, isClearable, isSearchable, isLoading, isDisabled,
 * components, classNames, styles, theme, menuPortalTarget, etc.).
 *
 * `value` accepts either an `Option` object or a primitive (`"web"`);
 * primitives are resolved against `defaultOptions` when it is a static
 * array. For fully-dynamic option sets, keep passing the `Option` object
 * directly.
 */
export function ReactSelectAsync<
  Option = unknown,
  IsMulti extends boolean = false,
  Group extends GroupBase<Option> = GroupBase<Option>,
>(props: ReactSelectAsyncComponentProps<Option, IsMulti, Group>) {
  const {
    size = "default",
    invalid,
    wrapOptionText = false,
    classNames,
    components,
    styles,
    unstyled = true,
    value: providedValue,
    onValueChange,
    onChange: providedOnChange,
    defaultOptions,
    isMulti,
    getOptionValue,
    menuPortalTarget,
    menuPosition,
    menuShouldBlockScroll,
    captureMenuScroll,
    ...rest
  } = props as ReactSelectAsyncComponentProps<Option, IsMulti, Group> & {
    onValueChange?: (value: unknown) => void
    onChange?: ReactSelectProps<Option, IsMulti, Group>["onChange"]
  }

  const lookupOptions = Array.isArray(defaultOptions)
    ? (defaultOptions as ReadonlyArray<Option | Group>)
    : undefined

  const resolvedValue = resolveValue<Option, Group>(
    providedValue,
    lookupOptions,
    isMulti as boolean | undefined,
    getOptionValue
  ) as ReactSelectProps<Option, IsMulti, Group>["value"]

  const resolvedOnChange: ReactSelectProps<Option, IsMulti, Group>["onChange"] =
    onValueChange
      ? (newValue, meta) => {
          onValueChange(
            extractValueFromChange<Option>(newValue, getOptionValue)
          )
          providedOnChange?.(newValue, meta)
        }
      : providedOnChange

  const resolvedMenuPortalTarget =
    resolveReactSelectMenuPortalTarget(menuPortalTarget)
  const resolvedMenuPosition =
    menuPosition ?? (resolvedMenuPortalTarget ? "fixed" : undefined)

  return (
    <RawAsyncSelect<Option, IsMulti, Group>
      unstyled={unstyled}
      classNames={composeClassNames<Option, IsMulti, Group>(
        reactSelectClassNames<Option, IsMulti, Group>({
          size,
          invalid,
          wrapOptionText,
          isMulti: Boolean(isMulti),
        }),
        classNames
      )}
      components={composeComponents<Option, IsMulti, Group>(components, {
        wrapOptionText,
      })}
      styles={mergeStyles(
        reactSelectStyles<Option, IsMulti, Group>({
          size,
          invalid,
          wrapOptionText,
          isMulti: Boolean(isMulti),
        }),
        styles
      )}
      defaultOptions={defaultOptions}
      isMulti={isMulti}
      getOptionValue={getOptionValue}
      value={resolvedValue}
      onChange={resolvedOnChange}
      {...rest}
      menuPortalTarget={resolvedMenuPortalTarget}
      menuPosition={resolvedMenuPosition}
      menuShouldBlockScroll={menuShouldBlockScroll ?? false}
      captureMenuScroll={captureMenuScroll ?? true}
    />
  )
}

export type { AsyncProps }
