// 前端开发环境统一通过 /api 代理转发到后端，避免手工切换地址。
const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "/api";

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
    this.status = status;
    this.code = code;
  }
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

  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...options,
    headers,
    credentials: "include",
  });

  const data = await response.json().catch(() => null);
  if (!response.ok) {
    throw new APIError(
      response.status,
      data?.code ?? data?.error?.code ?? "request_failed",
      data?.message ?? data?.error?.message ?? "请求失败",
    );
  }
  const envelope = data as APIEnvelope<T>;
  if (envelope && typeof envelope === "object" && "data" in envelope) {
    return envelope.data;
  }
  return data as T;
}
