import {
  forwardRef,
  useState,
  type ComponentPropsWithoutRef,
  type ReactNode,
} from "react"
import { Eye, EyeOff } from "lucide-react"
import type { Control, FieldPath, FieldValues } from "react-hook-form"

import {
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form"
import { cn } from "@/lib/utils"

const fieldClassName = cn(
  "w-full rounded-lg border border-input bg-background px-3 text-foreground",
  "transition-all duration-150 ease-out outline-none placeholder:text-muted-foreground",
  "focus:border-ring focus:ring-2 focus:ring-ring/20"
)

const fieldErrorClassName =
  "border-destructive focus:border-destructive focus:ring-destructive/20"

const defaultLabelClassName = "block text-sm font-medium text-foreground"

const inputClass = "h-11 text-sm sm:h-[42px]"

export const PasswordInput = forwardRef<
  HTMLInputElement,
  Omit<ComponentPropsWithoutRef<"input">, "type">
>(function PasswordInput(
  { className, style, "aria-invalid": ariaInvalid, ...props },
  ref
) {
  const [show, setShow] = useState(false)
  const hasError = ariaInvalid === true || ariaInvalid === "true"

  return (
    <div className="relative">
      <input
        ref={ref}
        type={show ? "text" : "password"}
        className={cn(
          fieldClassName,
          inputClass,
          "pr-10",
          hasError && fieldErrorClassName,
          className
        )}
        style={style}
        aria-invalid={ariaInvalid}
        {...props}
      />
      <button
        type="button"
        tabIndex={-1}
        onClick={() => setShow((visible) => !visible)}
        className="absolute top-1/2 right-3 -translate-y-1/2 text-muted-foreground transition-colors duration-150 hover:text-foreground"
        aria-label={show ? "Hide password" : "Show password"}
      >
        {show ? <EyeOff size={16} /> : <Eye size={16} />}
      </button>
    </div>
  )
})

type FormPasswordFieldProps<
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
> = {
  control: Control<TFieldValues>
  name: TName
  label: string
  description?: ReactNode
  footer?: ReactNode
  itemClassName?: string
  labelClassName?: string
} & Omit<
  ComponentPropsWithoutRef<typeof PasswordInput>,
  "value" | "onChange" | "onBlur" | "name" | "ref"
>

export function FormPasswordField<
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
>({
  control,
  name,
  label,
  description,
  footer,
  itemClassName,
  labelClassName,
  className,
  ...inputProps
}: FormPasswordFieldProps<TFieldValues, TName>) {
  return (
    <FormField
      control={control}
      name={name}
      render={({ field, fieldState }) => (
        <FormItem className={cn("space-y-1.5", itemClassName)}>
          <FormLabel className={cn(defaultLabelClassName, labelClassName)}>
            {label}
          </FormLabel>
          {description ? (
            <FormDescription>{description}</FormDescription>
          ) : null}
          <FormControl>
            <PasswordInput
              className={cn(className, fieldState.error && fieldErrorClassName)}
              aria-invalid={fieldState.error ? true : undefined}
              {...field}
              {...inputProps}
            />
          </FormControl>
          <FormMessage />
          {footer}
        </FormItem>
      )}
    />
  )
}
