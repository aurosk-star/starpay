import { RouterProvider } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { Toaster } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { ThemeProvider } from "@/components/theme-provider";
import { AuthScreen } from "@/features/auth/auth-screen";
import { me, refresh } from "@/features/auth/api";
import { useAuthStore } from "@/features/auth/store";

import { router } from "./router";

function App() {
  const { t } = useTranslation();
  const accessToken = useAuthStore((state) => state.accessToken);
  const hydrated = useAuthStore((state) => state.hydrated);
  const hydrate = useAuthStore((state) => state.hydrate);
  const setSession = useAuthStore((state) => state.setSession);
  const clearSession = useAuthStore((state) => state.clearSession);
  const [authChecking, setAuthChecking] = useState(true);

  useEffect(() => {
    hydrate();
  }, [hydrate]);

  useEffect(() => {
    if (!hydrated) return;
    setAuthChecking(true);
    if (!accessToken) {
      void refresh()
        .then((result) => setSession(result.access_token, result.user))
        .catch(() => clearSession())
        .finally(() => setAuthChecking(false));
      return;
    }
    void me(accessToken)
      .then((result) => setSession(accessToken, result.user))
      .catch(() => clearSession())
      .finally(() => setAuthChecking(false));
  }, [accessToken, clearSession, hydrated, setSession]);

  if (!hydrated || authChecking) {
    return (
      <main className="flex min-h-[100dvh] items-center justify-center bg-background text-sm text-muted-foreground">
        {t("auth.loading")}
      </main>
    );
  }

  if (!accessToken) {
    return (
      <ThemeProvider>
        <TooltipProvider>
          <AuthScreen />
          <Toaster richColors />
        </TooltipProvider>
      </ThemeProvider>
    );
  }

  return (
    <ThemeProvider>
      <TooltipProvider>
        <RouterProvider router={router} />
        <Toaster richColors />
      </TooltipProvider>
    </ThemeProvider>
  );
}

export default App;
