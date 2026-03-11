import { API_BASE_URL } from '../constants';

/** apiGet / apiPost 的可选参数 */
export interface ApiRequestOptions {
  signal?: AbortSignal;
  headers?: HeadersInit;
  /** 透传给 fetch 的额外选项（如 cache: 'no-store'） */
  fetchOptions?: Omit<RequestInit, 'method' | 'body' | 'headers' | 'signal'>;
}

interface ApiErrorOptions {
  status?: number;
  code?: string;
}

interface ParsedError {
  message?: string;
  code?: string;
}

/** API 请求失败时抛出的错误类型 */
export class ApiError extends Error {
  readonly status: number;
  readonly code?: string;

  constructor(message: string, options: ApiErrorOptions = {}) {
    super(message);
    this.name = 'ApiError';
    this.status = options.status ?? 0;
    this.code = options.code;
  }
}

function buildApiUrl(path: string): string {
  // 绝对 URL 直接使用（如 notifier 服务地址）
  if (/^https?:\/\//i.test(path)) {
    return path;
  }

  const base = API_BASE_URL.replace(/\/+$/, '');
  const normalizedPath = path.startsWith('/') ? path : `/${path}`;
  return `${base}${normalizedPath}`;
}

/**
 * 解析后端错误响应体，兼容两种格式：
 * - 旧版: { "error": "string" }
 * - 新版: { "error": { "code": "...", "message": "...", "request_id": "..." } }
 */
function parseErrorPayload(errorText: string): ParsedError {
  if (!errorText) return {};

  try {
    const parsed = JSON.parse(errorText) as {
      error?: string | { code?: unknown; message?: unknown };
      message?: unknown;
    };

    if (typeof parsed.error === 'string') {
      return { message: parsed.error };
    }

    if (parsed.error && typeof parsed.error === 'object') {
      return {
        code: typeof parsed.error.code === 'string' ? parsed.error.code : undefined,
        message: typeof parsed.error.message === 'string' ? parsed.error.message : undefined,
      };
    }

    if (typeof parsed.message === 'string') {
      return { message: parsed.message };
    }
  } catch {
    return { message: errorText };
  }

  return {};
}

/**
 * 从服务端错误响应文本中提取用户可见消息。
 * 兼容新旧两种错误格式，非 JSON 时回退到原始文本。
 */
export function extractErrorMessage(errorText: string, fallback: string): string {
  if (!errorText) return fallback;
  const { message } = parseErrorPayload(errorText);
  return message || errorText;
}

async function request<T>(path: string, init: RequestInit): Promise<T> {
  try {
    const response = await fetch(buildApiUrl(path), init);

    if (!response.ok) {
      const errorText = await response.text();
      const parsed = parseErrorPayload(errorText);

      throw new ApiError(
        extractErrorMessage(errorText, `请求失败 (${response.status})`),
        { status: response.status, code: parsed.code },
      );
    }

    const text = await response.text();
    if (!text) return undefined as T;

    try {
      return JSON.parse(text) as T;
    } catch {
      return text as T;
    }
  } catch (error) {
    if (error instanceof ApiError) throw error;
    if (error instanceof Error && error.name === 'AbortError') throw error;

    const message = error instanceof Error ? error.message : '网络请求失败';
    throw new ApiError(message, { status: 0, code: 'network_error' });
  }
}

/** 发起 GET 请求并解析 JSON 响应 */
export function apiGet<T>(path: string, options: ApiRequestOptions = {}): Promise<T> {
  return request<T>(path, {
    ...options.fetchOptions,
    method: 'GET',
    headers: options.headers,
    signal: options.signal,
  });
}

/** 发起 POST 请求，自动序列化 body 为 JSON */
export function apiPost<T>(path: string, body: unknown, options: ApiRequestOptions = {}): Promise<T> {
  const headers = new Headers(options.headers);
  if (!headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }

  return request<T>(path, {
    ...options.fetchOptions,
    method: 'POST',
    headers,
    body: JSON.stringify(body),
    signal: options.signal,
  });
}
