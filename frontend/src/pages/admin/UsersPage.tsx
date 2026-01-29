import { useState, useEffect, useCallback } from 'react';
import { Search, RefreshCw, Shield, ShieldOff, UserCheck, UserX } from 'lucide-react';
import type { AdminUser } from '../../types/admin';
import {
  fetchUsers,
  updateUser,
  getUserRoleLabel,
  getUserStatusLabel,
} from '../../api/admin';

// 角色徽章
function RoleBadge({ role }: { role: AdminUser['role'] }) {
  const isAdmin = role === 'admin';
  return (
    <span
      className={`px-2 py-1 text-xs font-medium rounded ${
        isAdmin ? 'bg-accent/20 text-accent' : 'bg-muted/20 text-muted'
      }`}
    >
      {getUserRoleLabel(role)}
    </span>
  );
}

// 状态徽章
function StatusBadge({ status }: { status: AdminUser['status'] }) {
  const isActive = status === 'active';
  return (
    <span
      className={`px-2 py-1 text-xs font-medium rounded ${
        isActive ? 'bg-success/20 text-success' : 'bg-danger/20 text-danger'
      }`}
    >
      {getUserStatusLabel(status)}
    </span>
  );
}

// 主页面组件
export default function UsersPage() {
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [total, setTotal] = useState(0);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // 筛选状态
  const [roleFilter, setRoleFilter] = useState<string>('');
  const [statusFilter, setStatusFilter] = useState<string>('');
  const [searchQuery, setSearchQuery] = useState('');
  const [page, setPage] = useState(0);
  const pageSize = 20;

  // 处理状态
  const [processingId, setProcessingId] = useState<string | null>(null);

  // 加载数据
  const loadData = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const res = await fetchUsers({
        role: roleFilter || undefined,
        status: statusFilter || undefined,
        offset: page * pageSize,
        limit: pageSize,
      });
      setUsers(res.data);
      setTotal(res.total);
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载失败');
    } finally {
      setIsLoading(false);
    }
  }, [roleFilter, statusFilter, page]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  // 切换角色
  const handleToggleRole = async (user: AdminUser) => {
    const newRole = user.role === 'admin' ? 'user' : 'admin';
    const action = newRole === 'admin' ? '提升为管理员' : '降级为普通用户';
    if (!confirm(`确定要将「${user.username}」${action}吗？`)) return;

    setProcessingId(user.id);
    try {
      await updateUser(user.id, { role: newRole });
      loadData();
    } catch (err) {
      alert(err instanceof Error ? err.message : '操作失败');
    } finally {
      setProcessingId(null);
    }
  };

  // 切换状态
  const handleToggleStatus = async (user: AdminUser) => {
    const newStatus = user.status === 'active' ? 'disabled' : 'active';
    const action = newStatus === 'active' ? '启用' : '禁用';
    if (!confirm(`确定要${action}用户「${user.username}」吗？`)) return;

    setProcessingId(user.id);
    try {
      await updateUser(user.id, { status: newStatus });
      loadData();
    } catch (err) {
      alert(err instanceof Error ? err.message : '操作失败');
    } finally {
      setProcessingId(null);
    }
  };

  // 过滤用户（本地搜索）
  const filteredUsers = searchQuery
    ? users.filter(
        (u) =>
          u.username.toLowerCase().includes(searchQuery.toLowerCase()) ||
          u.email?.toLowerCase().includes(searchQuery.toLowerCase())
      )
    : users;

  const totalPages = Math.ceil(total / pageSize);

  return (
    <div className="space-y-6">
      {/* 工具栏 */}
      <div className="flex flex-col sm:flex-row gap-4 items-start sm:items-center justify-between">
        <div className="flex flex-wrap items-center gap-4">
          {/* 搜索 */}
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted" />
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="搜索用户名或邮箱..."
              className="pl-10 pr-4 py-2 bg-surface border border-muted/30 rounded-lg text-primary placeholder-muted focus:outline-none focus:border-accent w-64"
            />
          </div>

          {/* 角色筛选 */}
          <select
            value={roleFilter}
            onChange={(e) => {
              setRoleFilter(e.target.value);
              setPage(0);
            }}
            className="px-3 py-2 bg-surface border border-muted/30 rounded-lg text-primary focus:outline-none focus:border-accent"
          >
            <option value="">全部角色</option>
            <option value="admin">管理员</option>
            <option value="user">普通用户</option>
          </select>

          {/* 状态筛选 */}
          <select
            value={statusFilter}
            onChange={(e) => {
              setStatusFilter(e.target.value);
              setPage(0);
            }}
            className="px-3 py-2 bg-surface border border-muted/30 rounded-lg text-primary focus:outline-none focus:border-accent"
          >
            <option value="">全部状态</option>
            <option value="active">正常</option>
            <option value="disabled">已禁用</option>
          </select>
        </div>

        <button
          onClick={loadData}
          disabled={isLoading}
          className="flex items-center gap-2 px-4 py-2 bg-surface hover:bg-elevated text-secondary hover:text-primary rounded-lg transition-colors"
        >
          <RefreshCw className={`w-4 h-4 ${isLoading ? 'animate-spin' : ''}`} />
          刷新
        </button>
      </div>

      {/* 错误提示 */}
      {error && (
        <div className="p-4 bg-danger/10 border border-danger/30 rounded-lg text-danger">
          {error}
        </div>
      )}

      {/* 表格 */}
      <div className="bg-surface rounded-lg border border-muted/20 overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-muted/10">
              <tr>
                <th className="px-4 py-3 text-left text-sm font-medium text-secondary">用户</th>
                <th className="px-4 py-3 text-left text-sm font-medium text-secondary">邮箱</th>
                <th className="px-4 py-3 text-left text-sm font-medium text-secondary">角色</th>
                <th className="px-4 py-3 text-left text-sm font-medium text-secondary">状态</th>
                <th className="px-4 py-3 text-left text-sm font-medium text-secondary">注册时间</th>
                <th className="px-4 py-3 text-right text-sm font-medium text-secondary">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-muted/10">
              {isLoading ? (
                <tr>
                  <td colSpan={6} className="px-4 py-12 text-center text-muted">
                    <RefreshCw className="w-6 h-6 animate-spin mx-auto mb-2" />
                    加载中...
                  </td>
                </tr>
              ) : filteredUsers.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-4 py-12 text-center text-muted">
                    暂无用户记录
                  </td>
                </tr>
              ) : (
                filteredUsers.map((user) => (
                  <tr key={user.id} className="hover:bg-muted/5">
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-3">
                        {user.avatar_url ? (
                          <img
                            src={user.avatar_url}
                            alt=""
                            className="w-8 h-8 rounded-full"
                          />
                        ) : (
                          <div className="w-8 h-8 rounded-full bg-muted/30 flex items-center justify-center">
                            <span className="text-sm font-medium text-muted">
                              {user.username.charAt(0).toUpperCase()}
                            </span>
                          </div>
                        )}
                        <div>
                          <p className="font-medium text-primary">{user.username}</p>
                          <p className="text-xs text-muted">GitHub ID: {user.github_id}</p>
                        </div>
                      </div>
                    </td>
                    <td className="px-4 py-3 text-secondary">
                      {user.email || '-'}
                    </td>
                    <td className="px-4 py-3">
                      <RoleBadge role={user.role} />
                    </td>
                    <td className="px-4 py-3">
                      <StatusBadge status={user.status} />
                    </td>
                    <td className="px-4 py-3 text-sm text-muted">
                      {new Date(user.created_at * 1000).toLocaleDateString()}
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center justify-end gap-2">
                        {/* 切换角色 */}
                        <button
                          onClick={() => handleToggleRole(user)}
                          disabled={processingId === user.id}
                          className={`p-1.5 rounded transition-colors ${
                            user.role === 'admin'
                              ? 'text-warning hover:bg-warning/20'
                              : 'text-accent hover:bg-accent/20'
                          }`}
                          title={user.role === 'admin' ? '降级为普通用户' : '提升为管理员'}
                        >
                          {user.role === 'admin' ? (
                            <ShieldOff className="w-4 h-4" />
                          ) : (
                            <Shield className="w-4 h-4" />
                          )}
                        </button>

                        {/* 切换状态 */}
                        <button
                          onClick={() => handleToggleStatus(user)}
                          disabled={processingId === user.id}
                          className={`p-1.5 rounded transition-colors ${
                            user.status === 'active'
                              ? 'text-danger hover:bg-danger/20'
                              : 'text-success hover:bg-success/20'
                          }`}
                          title={user.status === 'active' ? '禁用用户' : '启用用户'}
                        >
                          {user.status === 'active' ? (
                            <UserX className="w-4 h-4" />
                          ) : (
                            <UserCheck className="w-4 h-4" />
                          )}
                        </button>
                      </div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        {/* 分页 */}
        {totalPages > 1 && (
          <div className="px-4 py-3 border-t border-muted/20 flex items-center justify-between">
            <p className="text-sm text-muted">
              共 {total} 条记录
            </p>
            <div className="flex items-center gap-2">
              <button
                onClick={() => setPage((p) => Math.max(0, p - 1))}
                disabled={page === 0}
                className="px-3 py-1 text-sm bg-muted/20 hover:bg-muted/30 rounded disabled:opacity-50 transition-colors"
              >
                上一页
              </button>
              <span className="text-sm text-secondary">
                {page + 1} / {totalPages}
              </span>
              <button
                onClick={() => setPage((p) => Math.min(totalPages - 1, p + 1))}
                disabled={page >= totalPages - 1}
                className="px-3 py-1 text-sm bg-muted/20 hover:bg-muted/30 rounded disabled:opacity-50 transition-colors"
              >
                下一页
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
