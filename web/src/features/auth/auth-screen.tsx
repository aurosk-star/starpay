import { useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { GalleryVerticalEnd } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
  FieldSeparator,
} from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { APIError } from "@/lib/api";

import { login, setupAdmin } from "./api";
import { useAuthStore } from "./store";

type Mode = "login" | "setup";

export function AuthScreen() {
  const { t } = useTranslation();
  const [mode, setMode] = useState<Mode>("login");
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);
  const setSession = useAuthStore((state) => state.setSession);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setPending(true);

    const form = new FormData(event.currentTarget);
    const username = String(form.get("username") ?? "");
    const password = String(form.get("password") ?? "");

    try {
      const result =
        mode === "setup"
          ? await setupAdmin({
              username,
              password,
              email: String(form.get("email") ?? ""),
              display_name: String(form.get("display_name") ?? ""),
            })
          : await login({ username, password });

      setSession(result.access_token, result.user);
    } catch (err) {
      if (err instanceof APIError) {
        setError(err.message);
      } else {
        setError(t("auth.requestFailed"));
      }
    } finally {
      setPending(false);
    }
  }

  return (
    <main className="grid min-h-svh lg:grid-cols-2">
      <div className="flex flex-col gap-4 p-6 md:p-10">
        <div className="flex justify-center gap-2 md:justify-start">
          <a href="/" className="flex items-center gap-2 font-medium">
            <div className="flex size-6 items-center justify-center rounded-md bg-primary text-primary-foreground">
              <GalleryVerticalEnd className="size-4" />
            </div>
            {t("common.productName")}
          </a>
        </div>
        <div className="flex flex-1 items-center justify-center">
          <div className="w-full max-w-xs">
            <form className="flex flex-col gap-6" onSubmit={handleSubmit}>
              <FieldGroup>
                <div className="flex flex-col items-center gap-1 text-center">
                  <h1 className="text-2xl font-bold">
                    {mode === "setup"
                      ? t("auth.setupTitle")
                      : t("auth.loginTitle")}
                  </h1>
                  <p className="text-balance text-sm text-muted-foreground">
                    {mode === "setup"
                      ? t("auth.setupDescription")
                      : t("auth.loginDescription")}
                  </p>
                </div>

                <Field>
                  <FieldLabel htmlFor="username">
                    {t("auth.username")}
                  </FieldLabel>
                  <Input
                    id="username"
                    name="username"
                    autoComplete="username"
                    required
                  />
                </Field>

                {mode === "setup" ? (
                  <>
                    <Field>
                      <FieldLabel htmlFor="email">{t("auth.email")}</FieldLabel>
                      <Input
                        id="email"
                        name="email"
                        type="email"
                        autoComplete="email"
                        required
                      />
                    </Field>
                    <Field>
                      <FieldLabel htmlFor="display_name">
                        {t("auth.displayName")}
                      </FieldLabel>
                      <Input id="display_name" name="display_name" />
                      <FieldDescription>
                        {t("auth.displayNameHint")}
                      </FieldDescription>
                    </Field>
                  </>
                ) : null}

                <Field>
                  <FieldLabel htmlFor="password">
                    {t("auth.password")}
                  </FieldLabel>
                  <Input
                    id="password"
                    name="password"
                    type="password"
                    autoComplete={
                      mode === "setup" ? "new-password" : "current-password"
                    }
                    required
                  />
                </Field>

                {error ? (
                  <p className="text-sm text-destructive">{error}</p>
                ) : null}

                <Field>
                  <Button type="submit" disabled={pending}>
                    {pending
                      ? t("auth.processing")
                      : mode === "setup"
                        ? t("auth.createAndEnter")
                        : t("auth.loginAction")}
                  </Button>
                </Field>

                <FieldSeparator>{t("auth.orSeparator")}</FieldSeparator>

                <Field>
                  <Button
                    variant="outline"
                    type="button"
                    onClick={() => {
                      setError(null);
                      setMode(mode === "setup" ? "login" : "setup");
                    }}
                  >
                    {mode === "setup"
                      ? t("auth.backToLogin")
                      : t("auth.setupAction")}
                  </Button>
                  <FieldDescription className="text-center">
                    {mode === "setup"
                      ? t("auth.hasAdmin")
                      : t("auth.firstDeploy")}
                  </FieldDescription>
                </Field>
              </FieldGroup>
            </form>
          </div>
        </div>
      </div>
      <div className="relative hidden bg-muted lg:block">
        <img
          src="/placeholder.svg"
          alt="Payment gateway login cover"
          className="absolute inset-0 h-full w-full object-cover dark:brightness-[0.2] dark:grayscale"
        />
      </div>
    </main>
  );
}
