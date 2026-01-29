// v1.0 认证相关类型定义

// 用户角色
export type UserRole = 'admin' | 'user';

// 用户状态
export type UserStatus = 'active' | 'disabled';

// 用户信息
export interface User {
  id: string;
  username: string;
  avatar_url: string;
  email: string;
  role: UserRole;
  status?: UserStatus;
  created_at?: number;
}

// 登录响应
export interface LoginResponse {
  user: User;
  token: string;
}

// 获取当前用户响应
export interface MeResponse {
  user: User;
}

// 认证错误响应
export interface AuthErrorResponse {
  error: string;
  code: string;
}

// 认证状态
export interface AuthState {
  user: User | null;
  token: string | null;
  isLoading: boolean;
  isAuthenticated: boolean;
  isAdmin: boolean;
}

// 认证上下文类型
export interface AuthContextType extends AuthState {
  login: () => void;
  logout: () => Promise<void>;
  refreshUser: () => Promise<void>;
}
