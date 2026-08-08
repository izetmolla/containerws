import { forwardRef, useState, type InputHTMLAttributes } from "react"
import { useTranslation } from "react-i18next"
import { Eye, EyeOff } from "lucide-react"
import { cn } from "@/lib/utils"
import {
  authFieldClassName,
  authInputClass,
  authLabelClassName,
} from "../lib/auth-form-styles"

interface AuthInputProps extends InputHTMLAttributes<HTMLInputElement> {
  label: string
}

export const AuthInput = forwardRef<HTMLInputElement, AuthInputProps>(
  ({ label, id, className = "", ...props }, ref) => {
    return (
      <div className="space-y-1.5">
        <label htmlFor={id} className={authLabelClassName}>
          {label}
        </label>
        <input
          ref={ref}
          id={id}
          {...props}
          className={cn(authFieldClassName, authInputClass, className)}
        />
      </div>
    )
  }
)
AuthInput.displayName = "AuthInput"

interface PasswordInputProps extends Omit<
  InputHTMLAttributes<HTMLInputElement>,
  "type"
> {
  label: string
}

export function PasswordInput({
  label,
  id,
  className,
  ...props
}: PasswordInputProps) {
  const { t } = useTranslation("authorization")
  const [show, setShow] = useState(false)
  return (
    <div className="space-y-1.5">
      <label htmlFor={id} className={authLabelClassName}>
        {label}
      </label>
      <div className="relative">
        <input
          id={id}
          type={show ? "text" : "password"}
          {...props}
          className={cn(authFieldClassName, authInputClass, "pr-10", className)}
        />
        <button
          type="button"
          onClick={() => setShow((s) => !s)}
          className="absolute top-1/2 right-3 -translate-y-1/2 text-muted-foreground transition-colors duration-150 hover:text-foreground"
          aria-label={show ? t("Hide password") : t("Show password")}
        >
          {show ? <EyeOff size={16} /> : <Eye size={16} />}
        </button>
      </div>
    </div>
  )
}

export function AuthButton({
  children,
  className,
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      {...props}
      className={cn(
        "h-11 min-h-11 w-full rounded-lg bg-primary text-sm font-semibold text-primary-foreground sm:h-[42px] sm:min-h-0",
        "transition-all duration-150 ease-out hover:bg-primary/90 active:scale-[0.99] disabled:opacity-50",
        className
      )}
    >
      {children}
    </button>
  )
}
