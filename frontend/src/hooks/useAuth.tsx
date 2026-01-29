import { createContext, useContext, useState, useEffect, useCallback, useRef } from 'react';
import type { ReactNode } from 'react';
import type { User, AuthContextType, AuthState, MeResponse } from '../types/auth';

// 本地存储键
const TOKEN_KEY = 'relay-pulse-token';
const USER_KEY = 'relay-pulse-user';

// API 基础路径
const API_BASE = '/api';

// 默认认证状态
const defaultAuthState: AuthState = {
  user: null,
  token: null,
  isLoading: true,
  isAuthenticated: false,
  isAdmin: false,
};

// 创建认证上下文
const AuthContext = createContext<AuthContextType | null>(null);

// 从本地存储获取初始状态
function getInitialState(): AuthState {
  try {
    const token = localStorage.getItem(TOKEN_KEY);
    const userStr = localStorage.getItem(USER_KEY);

    if (token && userStr) {
      const user = JSON.parse(userStr) as User;
      return {
        user,
        token,
        isLoading: true, // 仍需验证 token 有效性
        isAuthenticated: true,
        isAdmin: user.role === 'admin',
      };
    }
  } catch {
    // 忽略解析错误
  }
  return defaultAuthState;
}

// 保存到本地存储
function saveToStorage(token: string, user: User): void {
  localStorage.setItem(TOKEN_KEY, token);
  localStorage.setItem(USER_KEY, JSON.stringify(user));
}

// 清除本地存储
function clearStorage(): void {
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(USER_KEY);
}

// AuthProvider 组件 Props
interface AuthProviderProps {
  children: ReactNode;
}

// AuthProvider 组件
export function AuthProvider({ children }: AuthProviderProps) {
  const [state, setState] = useState<AuthState>(getInitialState);
  const initializedRef = useRef(false);

  // 刷新用户信息（返回 Promise，不直接调用 setState）
  const fetchUser = useCallback(async (): Promise<AuthState> => {
    const token = localStorage.getItem(TOKEN_KEY);
    if (!token) {
      return {
        ...defaultAuthState,
        isLoading: false,
      };
    }

    try {
      const response = await fetch(`${API_BASE}/auth/me`, {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });

      if (!response.ok) {
        // Token 无效，清除状态
        clearStorage();
        return {
          ...defaultAuthState,
          isLoading: false,
        };
      }

      const data: MeResponse = await response.json();
      const user = data.user;

      // 更新存储
      saveToStorage(token, user);
      return {
        user,
        token,
        isLoading: false,
        isAuthenticated: true,
        isAdmin: user.role === 'admin',
      };
    } catch {
      // 网络错误，保持当前用户但标记加载完成
      const currentToken = localStorage.getItem(TOKEN_KEY);
      const currentUserStr = localStorage.getItem(USER_KEY);
      if (currentToken && currentUserStr) {
        try {
          const user = JSON.parse(currentUserStr) as User;
          return {
            user,
            token: currentToken,
            isLoading: false,
            isAuthenticated: true,
            isAdmin: user.role === 'admin',
          };
        } catch {
          // 忽略
        }
      }
      return {
        ...defaultAuthState,
        isLoading: false,
      };
    }
  }, []);

  // 公开的刷新方法
  const refreshUser = useCallback(async () => {
    const newState = await fetchUser();
    setState(newState);
  }, [fetchUser]);

  // 登录（跳转到 GitHub OAuth）
  const login = useCallback(() => {
    // 保存当前路径用于登录后跳转
    const currentPath = window.location.pathname + window.location.search;
    sessionStorage.setItem('relay-pulse-redirect', currentPath);

    // 跳转到 GitHub OAuth 登录
    window.location.href = `${API_BASE}/auth/github/login`;
  }, []);

  // 登出
  const logout = useCallback(async () => {
    const token = localStorage.getItem(TOKEN_KEY);

    try {
      if (token) {
        await fetch(`${API_BASE}/auth/logout`, {
          method: 'POST',
          headers: {
            Authorization: `Bearer ${token}`,
          },
        });
      }
    } catch {
      // 忽略登出请求错误
    }

    // 清除本地状态
    clearStorage();
    setState({
      ...defaultAuthState,
      isLoading: false,
    });
  }, []);

  // 初始化：验证 token 和处理 OAuth 回调
  useEffect(() => {
    if (initializedRef.current) return;
    initializedRef.current = true;

    const init = async () => {
      // 检查 URL 中是否有 OAuth 回调 token
      const params = new URLSearchParams(window.location.search);
      const urlToken = params.get('token');

      if (urlToken) {
        // 从 URL 获取 token（OAuth 回调）
        localStorage.setItem(TOKEN_KEY, urlToken);

        // 清除 URL 中的 token 参数
        const url = new URL(window.location.href);
        url.searchParams.delete('token');
        window.history.replaceState({}, '', url.toString());

        // 获取重定向路径
        const redirect = sessionStorage.getItem('relay-pulse-redirect');
        if (redirect) {
          sessionStorage.removeItem('relay-pulse-redirect');
        }

        // 刷新用户信息
        const newState = await fetchUser();
        setState(newState);

        // 跳转到之前的页面
        if (redirect && redirect !== window.location.pathname) {
          window.location.href = redirect;
        }
      } else {
        // 正常初始化：验证现有 token
        const newState = await fetchUser();
        setState(newState);
      }
    };

    init();
  }, [fetchUser]);

  const contextValue: AuthContextType = {
    ...state,
    login,
    logout,
    refreshUser,
  };

  return (
    <AuthContext.Provider value={contextValue}>
      {children}
    </AuthContext.Provider>
  );
}

// useAuth Hook
export function useAuth(): AuthContextType {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}

// 导出 AuthContext 供测试使用
export { AuthContext };
