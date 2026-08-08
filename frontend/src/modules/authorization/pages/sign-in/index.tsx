import { zodResolver } from "@hookform/resolvers/zod"
import { useMutation, useQuery } from "@tanstack/react-query"
import { useForm } from "react-hook-form"
import { useEffect, useState } from "react"
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
import { AuthHeader } from "../../components/layout"
import {
  authCompactButtonClass,
  authCompactInputClass,
  authFieldClassName,
  authFieldItemClass,
  authFieldLabelClass,
  authFooterLinkClass,
  authLabelClassName,
} from "../../lib/auth-form-styles"
import { cn } from "@/lib/utils"
import { AuthButton } from "../../components/AuthInput"
import { getBranding, signIn, signInSchema, type SignInSchema } from "./api"
import {
  getFieldsErrorsFromData,
  getRequestErrorMessage,
} from "@/lib/network/errors"
import { toast } from "sonner"
import useAuthorizationStore from "@/store/authorization"
import OtherSessions from "./components/other-sessions"
import { Button } from "@/components/ui/button"

const SignInPage = () => {
  const { t } = useTranslation("authorization")
  const [showGoBackButton, setShowGoBackButton] = useState(false)
  const [showAccountPicker, setShowAccountPicker] = useState(true)
  const sessions = useAuthorizationStore((state) => state.sessions)
  const [searchParams] = useSearchParams()
  const redirectUrl = searchParams.get("redirectUrl")
  const signInAction = useAuthorizationStore((state) => state.signIn)
  const setRedirectUrl = useAuthorizationStore((state) => state.setRedirectUrl)

  const brandingQuery = useQuery({
    queryKey: ["authorization", "branding"],
    queryFn: getBranding,
    staleTime: 5 * 60_000,
  })

  const osLabel =
    brandingQuery.data?.data?.os_label?.trim() ||
    brandingQuery.data?.data?.os_name?.trim() ||
    brandingQuery.data?.data?.workspace_name?.trim() ||
    ""

  useEffect(() => {
    if (redirectUrl) {
      setRedirectUrl(redirectUrl)
    }
  }, [redirectUrl, setRedirectUrl])

  const form = useForm<SignInSchema>({
    resolver: zodResolver(signInSchema),
    defaultValues: {
      email: "",
      password: "",
    },
  })

  // Full page navigation so proxy routes (/novnc, /codeserver, …) are not
  // handled by React Router (which would 404 them).
  const completeRedirect = () => {
    window.location.replace(redirectUrl || "/")
  }

  const signInMutation = useMutation({
    mutationFn: signIn,
    onSuccess: (data) => {
      console.log("data", data)
      signInAction({
        user: data?.user,
        tokens: data?.tokens,
      })
      completeRedirect()
    },
    onError: (error) => {
      if (getFieldsErrorsFromData(error)?.length > 0) {
        getFieldsErrorsFromData(error).forEach((field) => {
          form.setError(field.field as keyof SignInSchema, {
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

  const onSubmit = (data: SignInSchema) => {
    signInMutation.mutate(data)
  }

  if (sessions.length > 0 && showAccountPicker) {
    return (
      <OtherSessions
        onSelectSession={() => completeRedirect()}
        onUseAnotherAccount={() => {
          setShowAccountPicker(false)
          setShowGoBackButton(true)
        }}
        onSessionInvalid={() => {
          if (useAuthorizationStore.getState().sessions.length === 0) {
            setShowAccountPicker(false)
          }
        }}
      />
    )
  }

  return (
    <>
      <AuthHeader
        title={t("Welcome back")}
        subtitle={
          osLabel ? `Sign in to ${osLabel}` : t("Sign in to continue")
        }
      />
      <Form {...form}>
        <form className="space-y-3" onSubmit={form.handleSubmit(onSubmit)}>
          <FormField
            control={form.control}
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
                    type="text"
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
            placeholder={t("Enter your password")}
            autoComplete="current-password"
            itemClassName={authFieldItemClass}
            labelClassName={authFieldLabelClass}
            className={authCompactInputClass}
            footer={
              <div className="text-right">
                <Link
                  to="/forgot-password"
                  className="text-xs text-muted-foreground transition-colors hover:text-foreground hover:underline"
                >
                  {t("Forgot password?")}
                </Link>
              </div>
            }
          />
          {showGoBackButton ? (
            <div className="mt-1 flex items-stretch gap-2">
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="h-11 shrink-0 px-3 text-xs sm:h-[38px]"
                onClick={() => {
                  setShowGoBackButton(false)
                  setShowAccountPicker(true)
                }}
              >
                {t("Go back")}
              </Button>
              <AuthButton
                type="submit"
                className="h-11 min-h-11 flex-1 text-sm sm:h-[38px] sm:min-h-0"
                disabled={signInMutation.isPending}
              >
                {signInMutation.isPending ? t("Signing in…") : t("Sign in")}
              </AuthButton>
            </div>
          ) : (
            <AuthButton
              type="submit"
              className={authCompactButtonClass}
              disabled={signInMutation.isPending}
            >
              {signInMutation.isPending ? t("Signing in…") : t("Sign in")}
            </AuthButton>
          )}
        </form>
      </Form>
      <p className={authFooterLinkClass}>
        {t("Don't have an account?")}{" "}
        <Link
          to="/register"
          className="font-medium text-foreground underline-offset-4 hover:underline"
        >
          {t("Create account")}
        </Link>
      </p>
    </>
  )
}

export default SignInPage
