import { apiRequest } from "@/lib/api";

import type { AdminUser, AuthResponse, Role } from "./types";

export type SetupPayload = {
  username: string;
  email: string;
  password: string;
  display_name?: string;
};

export type LoginPayload = {
  username: string;
  password: string;
};

export function setupAdmin(payload: SetupPayload) {
  return apiRequest<AuthResponse>("/v1/admin/setup", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export function login(payload: LoginPayload) {
  return apiRequest<AuthResponse>("/v1/admin/auth/login", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export function refresh() {
  return apiRequest<AuthResponse>("/v1/admin/auth/refresh", {
    method: "POST",
  });
}

export function logout() {
  return apiRequest<void>("/v1/admin/auth/logout", {
    method: "POST",
  });
}

export function me(accessToken: string) {
  return apiRequest<{ user: AdminUser }>("/v1/admin/auth/me", {
    accessToken,
  });
}

export function listUsers(accessToken: string) {
  return apiRequest<{ items: AdminUser[] }>("/v1/admin/users", {
    accessToken,
  });
}

export type ManageUserPayload = {
  username: string;
  email: string;
  password?: string;
  display_name?: string;
  status: string;
  role_ids: number[];
};

export function createUser(accessToken: string, payload: ManageUserPayload) {
  return apiRequest<{ user: AdminUser }>("/v1/admin/users", {
    method: "POST",
    accessToken,
    body: JSON.stringify(payload),
  });
}

export function updateUser(
  accessToken: string,
  id: number,
  payload: ManageUserPayload,
) {
  return apiRequest<{ user: AdminUser }>(`/v1/admin/users/${id}`, {
    method: "PUT",
    accessToken,
    body: JSON.stringify(payload),
  });
}

export function deleteUser(accessToken: string, id: number) {
  return apiRequest<void>(`/v1/admin/users/${id}`, {
    method: "DELETE",
    accessToken,
  });
}

export function listRoles(accessToken: string) {
  return apiRequest<{ items: Role[] }>("/v1/admin/roles", {
    accessToken,
  });
}
