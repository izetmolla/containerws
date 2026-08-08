"use client"

import * as React from "react"
import RawCreatableSelect, { type CreatableProps } from "react-select/creatable"
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

export type ReactSelectCreatableComponentProps<
  Option = unknown,
  IsMulti extends boolean = false,
  Group extends GroupBase<Option> = GroupBase<Option>,
> = Omit<CreatableProps<Option, IsMulti, Group>, "value"> &
  ReactSelectShadcnProps &
  ReactSelectKeyApi<Option, IsMulti> & {
    ref?: React.Ref<SelectInstance<Option, IsMulti, Group>>
  }

/**
 * shadcn-styled wrapper around `react-select/creatable`.
 *
 * All native props are forwarded (allowCreateWhileLoading, createOptionPosition,
 * formatCreateLabel, getNewOptionData, isValidNewOption, onCreateOption,
 * onChange, isMulti, isClearable, isSearchable, isLoading, isDisabled,
 * components, classNames, styles, theme, menuPortalTarget, etc.).
 *
 * `value` accepts either an `Option` object or a primitive (`"web"`); the
 * wrapper resolves primitives against `options` via `getOptionValue`.
 */
export function ReactSelectCreatable<
  Option = unknown,
  IsMulti extends boolean = false,
  Group extends GroupBase<Option> = GroupBase<Option>,
>(props: ReactSelectCreatableComponentProps<Option, IsMulti, Group>) {
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
    options,
    isMulti,
    getOptionValue,
    menuPortalTarget,
    menuPosition,
    menuShouldBlockScroll,
    captureMenuScroll,
    ...rest
  } = props as ReactSelectCreatableComponentProps<Option, IsMulti, Group> & {
    onValueChange?: (value: unknown) => void
    onChange?: ReactSelectProps<Option, IsMulti, Group>["onChange"]
  }

  const resolvedValue = resolveValue<Option, Group>(
    providedValue,
    options,
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
    <RawCreatableSelect<Option, IsMulti, Group>
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
      options={options}
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

export type { CreatableProps }
