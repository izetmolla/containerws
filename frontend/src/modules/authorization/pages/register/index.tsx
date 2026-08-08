import { zodResolver } from "@hookform/resolvers/zod"
import { useMutation } from "@tanstack/react-query"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { Link, useSearchParams } from "react-router"

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
import { AuthHeader } from "../../components/layout"
import {
  authCompactButtonClass,
  authCompactInputClass,
  authFieldClassName,
  authOAuthStackClass,
  authFieldItemClass,
  authFieldLabelClass,
  authFooterLinkClass,
  authLabelClassName,
} from "../../lib/auth-form-styles"
import { AuthButton } from "../../components/AuthInput"
import { AuthOAuthDivider } from "../../components/AuthOAuthDivider"
import { GoogleSignInButton } from "../../components/GoogleSignInButton"
import { register, registerSchema, type RegisterSchema } from "./api"
import {
  getFieldsErrorsFromData,
  getRequestErrorMessage,
} from "@/lib/network/errors"
import { toast } from "sonner"
import useAuthorizationStore from "@/store/authorization"

const RegisterPage = () => {
  const { t } = useTranslation("authorization")
  const [searchParams] = useSearchParams()
  const redirectUrl = searchParams.get("redirectUrl")
  const signIn = useAuthorizationStore((state) => state.signIn)
  const form = useForm<RegisterSchema>({
    resolver: zodResolver(registerSchema),
    defaultValues: {
      first_name: "",
      last_name: "",
      email: "",
      password: "",
      confirm_password: "",
    },
  })

  // Full page navigation so proxy routes (/novnc, /codeserver, …) are not
  // handled by React Router (which would 404 them).
  const completeRedirect = () => {
    window.location.replace(redirectUrl || "/")
  }

  const registerMutation = useMutation({
    mutationFn: register,
    onSuccess: (data) => {
      signIn({
        user: data.user,
        tokens: data.tokens,
      })
      completeRedirect()
    },
    onError: (error) => {
      const ccc = getFieldsErrorsFromData(error)
      console.log("ccc", ccc)
      if (getFieldsErrorsFromData(error)?.length > 0) {
        getFieldsErrorsFromData(error).forEach((field) => {
          form.setError(field.field as keyof RegisterSchema, {
            message: field.message,
          })
        })
      } else {
        toast.error(
          getRequestErrorMessage(error, t("An unknown error occurred")),
          {
            description: error.message,
          }
        )
      }
    },
  })

  const onSubmit = ({ confirm_password, ...data }: RegisterSchema) => {
    void confirm_password
    registerMutation.mutate(data)
  }

  return (
    <>
      <AuthHeader
        title={t("Create your account")}
        subtitle={t("Start managing your workforce in minutes")}
      />
      <div className={authOAuthStackClass}>
        <GoogleSignInButton
          mode="register"
          className="text-[13px] sm:h-[38px]"
        />
        <AuthOAuthDivider label={t("or register with email")} />
      </div>
      <Form {...form}>
        <form className="space-y-3" onSubmit={form.handleSubmit(onSubmit)}>
          <div className="grid grid-cols-1 gap-2.5 min-[400px]:grid-cols-2">
            <FormField
              control={form.control}
              name="first_name"
              render={({ field }) => (
                <FormItem className={authFieldItemClass}>
                  <FormLabel
                    className={cn(authLabelClassName, authFieldLabelClass)}
                  >
                    {t("First name")}
                  </FormLabel>
                  <FormControl>
                    <input
                      type="text"
                      placeholder={t("John")}
                      autoComplete="given-name"
                      className={cn(authFieldClassName, authCompactInputClass)}
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="last_name"
              render={({ field }) => (
                <FormItem className={authFieldItemClass}>
                  <FormLabel
                    className={cn(authLabelClassName, authFieldLabelClass)}
                  >
                    {t("Last name")}
                  </FormLabel>
                  <FormControl>
                    <input
                      type="text"
                      placeholder={t("Doe")}
                      autoComplete="family-name"
                      className={cn(authFieldClassName, authCompactInputClass)}
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>
          <FormField
            control={form.control}
            name="email"
            render={({ field }) => (
              <FormItem className={authFieldItemClass}>
                <FormLabel
                  className={cn(authLabelClassName, authFieldLabelClass)}
                >
                  {t("Work email")}
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
          <FormPasswordField
            control={form.control}
            name="password"
            label={t("Password")}
            placeholder={t("Create a password")}
            autoComplete="new-password"
            itemClassName={authFieldItemClass}
            labelClassName={authFieldLabelClass}
            className={authCompactInputClass}
          />
          <FormPasswordField
            control={form.control}
            name="confirm_password"
            label={t("Confirm password")}
            placeholder={t("Repeat password")}
            autoComplete="new-password"
            itemClassName={authFieldItemClass}
            labelClassName={authFieldLabelClass}
            className={authCompactInputClass}
          />
          <AuthButton
            type="submit"
            className={authCompactButtonClass}
            disabled={registerMutation.isPending}
          >
            {registerMutation.isPending
              ? t("Creating account…")
              : t("Create account")}
          </AuthButton>
        </form>
      </Form>
      <p className={authFooterLinkClass}>
        {t("Already have an account?")}{" "}
        <Link
          to="/sign-in"
          className="font-medium text-foreground underline-offset-4 hover:underline"
        >
          {t("Sign in")}
        </Link>
      </p>
    </>
  )
}

export default RegisterPage
