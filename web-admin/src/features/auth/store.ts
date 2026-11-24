import { create } from "zustand";

import type { AdminUser } from "./types";

const STORAGE_KEY = "payment_gateway_access_token";

function readStoredToken() {
  if (typeof window === "undefined") return null;
  return window.localStorage.getItem(STORAGE_KEY);
}

type AuthState = {
  accessToken: string | null;
  user: AdminUser | null;
  hydrated: boolean;
  hydrate: () => void;
  setSession: (accessToken: string, user: AdminUser) => void;
  clearSession: () => void;
};

export const useAuthStore = create<AuthState>((set) => ({
  accessToken: null,
  user: null,
  hydrated: false,
  hydrate: () =>
    set(() => ({
      accessToken: readStoredToken(),
      hydrated: true,
    })),
  setSession: (accessToken, user) => {
    window.localStorage.setItem(STORAGE_KEY, accessToken);
    set({ accessToken, user });
  },
  clearSession: () => {
    window.localStorage.removeItem(STORAGE_KEY);
    set({ accessToken: null, user: null });
  },
}));
