const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080";

/** 表示带 HTTP 状态码的后端错误，供登录过期等场景分别处理。 */
export class APIError extends Error {
  /** 保存可读错误消息和 HTTP 状态码。 */
  constructor(message: string, public readonly status: number) {
    super(message);
    this.name = "APIError";
  }
}

/** 发起携带会话 Cookie 的 Go API 请求，并保留结构化错误状态。 */
export async function apiRequest<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: { "Content-Type": "application/json", ...init?.headers },
    credentials: "include",
    cache: "no-store",
  });
  if (response.status === 204) return undefined as T;
  const data = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new APIError(data.error ?? `请求失败（HTTP ${response.status}）`, response.status);
  }
  return data as T;
}

/** 返回用于 EventSource 和文件下载的完整后端地址。 */
export function apiURL(path: string): string {
  return `${API_BASE}${path}`;
}
