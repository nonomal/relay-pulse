import { useOutletContext } from 'react-router-dom';
import type { AdminFetchFn } from '../../hooks/admin/useAdminAuth';

/** 通过 Outlet context 传递给子页面的认证状态 */
export interface AdminOutletContext {
  adminFetch: AdminFetchFn;
}

/**
 * 子页面获取 adminFetch 的 Hook
 *
 * 使用 React Router 的 Outlet context 共享认证状态，
 * 确保所有子页面使用同一个 adminFetch 实例，
 * 401 时统一触发重新登录。
 */
export function useAdminContext(): AdminOutletContext {
  return useOutletContext<AdminOutletContext>();
}
