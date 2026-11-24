import { useAuthStore } from "@/features/auth/store";
import i18n from "@/i18n";
import { resolveAPIBaseURL } from "./api-base-url";

// Development uses /api through Rsbuild; production calls the same origin.
const API_BASE_URL = resolveAPIBaseURL(import.meta.env.VITE_API_BASE_URL);

// 登录/初始化接口的 401 属于业务错误，不应触发「会话过期」逻辑。
const AUTH_ENTRY_PATHS = ["/v1/admin/auth/login", "/v1/admin/setup"];

type RequestOptions = RequestInit & {
  accessToken?: string | null;
};

type APIEnvelope<T> = {
  code: string;
  message: string;
  data: T;
  error?: {
    code: string;
    message: string;
  } | null;
};

export class APIError extends Error {
  status: number;
  code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "APIError";
    this.status = status;
    this.code = code;
  }
}

const MESSAGE_KEY_BY_CODE: Record<string, string> = {
  invalid_credentials: "errors.api.invalidCredentials",
  unauthorized: "errors.api.unauthorized",
  forbidden: "errors.api.forbidden",
  not_found: "errors.api.notFound",
  validation_failed: "errors.api.validation",
  rate_limited: "errors.api.rateLimited",
  network_error: "errors.api.network",
};

const MESSAGE_KEY_BY_STATUS: Record<number, string> = {
  401: "errors.api.unauthorized",
  403: "errors.api.forbidden",
  404: "errors.api.notFound",
  422: "errors.api.validation",
  429: "errors.api.rateLimited",
};

function resolveMessage(
  status: number,
  code: string,
  serverMessage?: string | null,
): string {
  if (status >= 500) return i18n.t("errors.api.server");

  const key = MESSAGE_KEY_BY_CODE[code] ?? MESSAGE_KEY_BY_STATUS[status];
  if (key) return i18n.t(key);

  // 未知错误码时优先展示后端返回的具体原因，避免丢失关键信息。
  return serverMessage || i18n.t("errors.api.unknown");
}

let lastUnauthorizedAt = 0;

function handleUnauthorized() {
  const now = Date.now();
  // 同一轮抖动中多个请求同时 401 时只处理一次。
  if (now - lastUnauthorizedAt < 3000) return;
  lastUnauthorizedAt = now;

  const { accessToken, clearSession } = useAuthStore.getState();
  if (!accessToken) return;

  // 清理会话后 ShellLayout 会自动切回登录页；错误提示由调用方的 catch 统一展示，避免重复弹窗。
  clearSession();
}

export async function apiRequest<T>(
  path: string,
  options: RequestOptions = {},
): Promise<T> {
  const headers = new Headers(options.headers);
  if (!headers.has("Content-Type") && options.body) {
    headers.set("Content-Type", "application/json");
  }
  if (options.accessToken) {
    headers.set("Authorization", `Bearer ${options.accessToken}`);
  }

  let response: Response;
  try {
    response = await fetch(`${API_BASE_URL}${path}`, {
      ...options,
      headers,
      credentials: "include",
    });
  } catch {
    throw new APIError(0, "network_error", i18n.t("errors.api.network"));
  }

  const data = await response.json().catch(() => null);
  if (!response.ok) {
    const code = data?.code ?? data?.error?.code ?? "request_failed";
    const serverMessage = data?.message ?? data?.error?.message ?? null;

    if (
      response.status === 401 &&
      !AUTH_ENTRY_PATHS.some((entry) => path.startsWith(entry))
    ) {
      handleUnauthorized();
    }

    throw new APIError(
      response.status,
      code,
      resolveMessage(response.status, code, serverMessage),
    );
  }
  const envelope = data as APIEnvelope<T>;
  if (envelope && typeof envelope === "object" && "data" in envelope) {
    return envelope.data;
  }
  return data as T;
}
