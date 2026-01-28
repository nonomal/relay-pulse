import { useState, useCallback, useMemo } from 'react';
import { API_BASE_URL } from '../../constants';

const STORAGE_KEY = 'relay-pulse-admin-token';

/** Admin API 错误 */
export class AdminApiError extends Error {
  status: number;
  body?: unknown;

  constructor(message: string, status: number, body?: unknown) {
    super(message);
    this.name = 'AdminApiError';
    this.status = status;
    this.body = body;
  }
}

/** 从 sessionStorage 读取 token */
function readToken(): string {
  try {
    return sessionStorage.getItem(STORAGE_KEY) ?? '';
  } catch {
    return '';
  }
}

/** 保存 token 到 sessionStorage */
function writeToken(token: string): void {
  try {
    if (token) {
      sessionStorage.setItem(STORAGE_KEY, token);
    } else {
      sessionStorage.removeItem(STORAGE_KEY);
    }
  } catch {
    // sessionStorage 不可用时静默失败
  }
}

/**
 * Admin 认证 Hook
 *
 * 提供 token 管理和带认证的 fetch 封装。
 * Token 存储在 sessionStorage 中（关闭标签页后自动清除）。
 */
export function useAdminAuth() {
  const [token, setTokenState] = useState(readToken);

  const isAuthenticated = token !== '';

  const login = useCallback((newToken: string) => {
    const trimmed = newToken.trim();
    writeToken(trimmed);
    setTokenState(trimmed);
  }, []);

  const logout = useCallback(() => {
    writeToken('');
    setTokenState('');
  }, []);

  /**
   * 带认证的 fetch 封装
   *
   * 自动添加 X-Config-Token 头和 Content-Type。
   * 非 2xx 响应抛出 AdminApiError。
   */
  const adminFetch = useCallback(
    async <T = unknown>(
      path: string,
      options: RequestInit = {},
    ): Promise<T> => {
      const currentToken = readToken();
      if (!currentToken) {
        throw new AdminApiError('未认证', 401);
      }

      const headers = new Headers(options.headers);
      headers.set('X-Config-Token', currentToken);
      // 仅当 body 不是 FormData 时才设置 Content-Type
      // FormData 需要浏览器自动设置 boundary
      if (!headers.has('Content-Type') && options.body && !(options.body instanceof FormData)) {
        headers.set('Content-Type', 'application/json');
      }

      const url = `${API_BASE_URL}${path}`;
      const response = await fetch(url, { ...options, headers });

      if (response.status === 401) {
        // Token 失效，自动登出
        writeToken('');
        setTokenState('');
        throw new AdminApiError('Token 无效或已过期', 401);
      }

      if (!response.ok) {
        let body: unknown;
        try {
          body = await response.json();
        } catch {
          body = await response.text().catch(() => null);
        }
        const msg =
          (body && typeof body === 'object' && 'error' in body
            ? String((body as Record<string, unknown>).error)
            : null) ?? `请求失败 (${response.status})`;
        throw new AdminApiError(msg, response.status, body);
      }

      // 204 No Content
      if (response.status === 204) {
        return undefined as T;
      }

      return response.json() as Promise<T>;
    },
    [],
  );

  return useMemo(
    () => ({ token, isAuthenticated, login, logout, adminFetch }),
    [token, isAuthenticated, login, logout, adminFetch],
  );
}

export type AdminFetchFn = ReturnType<typeof useAdminAuth>['adminFetch'];
