import { useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { Link } from "react-router"
import { ArrowLeft } from "lucide-react"
import { z } from "zod"

import i18n from "@/components/providers/i18n/lib"
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form"
import { FormPasswordField } from "@/components/password"
import { cn } from "@/lib/utils"
import {
  authCompactButtonClass,
  authCompactInputClass,
  authFieldClassName,
  authFieldItemClass,
  authFieldLabelClass,
  authFooterLinkClass,
  authLabelClassName,
} from "../../lib/auth-form-styles"
import { AuthHeader } from "../../components/layout"
import { AuthButton } from "../../components/AuthInput"

const emailSchema = z.object({
  email: z.string().min(1, {
    message: i18n.t("Please enter your email address", { ns: "authorization" }),
  }),
})

const resetSchema = z
  .object({
    code: z.string().length(6, {
      message: i18n.t("Enter the 6-digit code", { ns: "authorization" }),
    }),
    password: z.string().min(1, {
      message: i18n.t("Please enter your new password", {
        ns: "authorization",
      }),
    }),
    confirm_password: z.string().min(1, {
      message: i18n.t("Please confirm your password", { ns: "authorization" }),
    }),
  })
  .refine((data) => data.password === data.confirm_password, {
    message: i18n.t("Passwords do not match", { ns: "authorization" }),
    path: ["confirm_password"],
  })

type EmailSchema = z.infer<typeof emailSchema>
type ResetSchema = z.infer<typeof resetSchema>

const ForgotPasswordPage = () => {
  const { t } = useTranslation("authorization")
  const [step, setStep] = useState<1 | 2>(1)
  const [email, setEmail] = useState("")

  const emailForm = useForm<EmailSchema>({
    resolver: zodResolver(emailSchema),
    defaultValues: { email: "" },
  })

  const resetForm = useForm<ResetSchema>({
    resolver: zodResolver(resetSchema),
    defaultValues: { code: "", password: "", confirm_password: "" },
  })

  if (step === 1) {
    return (
      <>
        <AuthHeader
          title={t("Reset your password")}
          subtitle={t("We'll send a verification code to your email")}
        />
        <Form {...emailForm}>
          <form
            className="space-y-3"
            onSubmit={emailForm.handleSubmit(({ email: value }) => {
              setEmail(value)
              setStep(2)
            })}
          >
            <FormField
              control={emailForm.control}
              name="email"
              render={({ field }) => (
                <FormItem className={authFieldItemClass}>
                  <FormLabel
                    className={cn(authLabelClassName, authFieldLabelClass)}
                  >
                    {t("Email address")}
                  </FormLabel>
                  <FormControl>
                    <input
                      type="email"
                      placeholder={t("you@company.com")}
                      autoComplete="email"
                      className={cn(authFieldClassName, authCompactInputClass)}
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <AuthButton type="submit" className={authCompactButtonClass}>
              {t("Send reset code")}
            </AuthButton>
          </form>
        </Form>
        <div className={authFooterLinkClass}>
          <Link
            to="/sign-in"
            className="inline-flex items-center gap-1.5 text-muted-foreground transition-colors hover:text-foreground"
          >
            <ArrowLeft size={14} />
            {t("Back to sign in")}
          </Link>
        </div>
      </>
    )
  }

  const maskedEmail = email.replace(/^(.{2}).*(@.*)$/, "$1***$2")

  return (
    <>
      <AuthHeader
        title={t("Check your email")}
        subtitle={t("Enter the code sent to {{email}}", { email: maskedEmail })}
      />
      <Form {...resetForm}>
        <form className="space-y-3" onSubmit={(e) => e.preventDefault()}>
          <FormPasswordField
            control={resetForm.control}
            name="password"
            label={t("New password")}
            placeholder={t("Create a new password")}
            autoComplete="new-password"
            itemClassName={authFieldItemClass}
            labelClassName={authFieldLabelClass}
            className={authCompactInputClass}
          />
          <FormPasswordField
            control={resetForm.control}
            name="confirm_password"
            label={t("Confirm password")}
            placeholder={t("Repeat your new password")}
            autoComplete="new-password"
            itemClassName={authFieldItemClass}
            labelClassName={authFieldLabelClass}
            className={authCompactInputClass}
          />
          <AuthButton type="submit" className={authCompactButtonClass}>
            {t("Reset password")}
          </AuthButton>
        </form>
      </Form>
      <div className={authFooterLinkClass}>
        <button
          type="button"
          onClick={() => setStep(1)}
          className="inline-flex items-center gap-1.5 text-muted-foreground transition-colors hover:text-foreground"
        >
          <ArrowLeft size={14} />
          {t("Use a different email")}
        </button>
      </div>
    </>
  )
}

export default ForgotPasswordPage
