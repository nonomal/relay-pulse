import { useState, useEffect, useCallback, useRef } from 'react';
import { Helmet } from 'react-helmet-async';
import { Save, RefreshCw, Upload, Download } from 'lucide-react';
import { useAdminContext } from '../../hooks/admin/useAdminContext';
import { useAdminMonitors } from '../../hooks/admin/useAdminMonitors';
import { AdminApiError } from '../../hooks/admin/useAdminAuth';
import { API_BASE_URL } from '../../constants';
import type { GlobalSetting, BoardConfig, ConfigVersions } from '../../types/admin';

export default function SettingsPage() {
  const { adminFetch } = useAdminContext();
  const { importConfig } = useAdminMonitors({ adminFetch });
  const fileInputRef = useRef<HTMLInputElement>(null);

  // 全局设置
  const [globalSetting, setGlobalSetting] = useState<string>('');
  const [globalVersion, setGlobalVersion] = useState<number>(0);
  const [settingLoading, setSettingLoading] = useState(false);
  const [settingError, setSettingError] = useState<string | null>(null);
  const [settingSaved, setSettingSaved] = useState(false);

  // Board 配置
  const [boards, setBoards] = useState<BoardConfig[]>([]);
  const [boardsLoading, setBoardsLoading] = useState(false);

  // 配置版本
  const [versions, setVersions] = useState<ConfigVersions | null>(null);

  // 导入/导出状态
  const [importLoading, setImportLoading] = useState(false);
  const [importResult, setImportResult] = useState<string | null>(null);
  const [exportLoading, setExportLoading] = useState(false);

  const fetchGlobalSetting = useCallback(async () => {
    setSettingLoading(true);
    setSettingError(null);
    try {
      const resp = await adminFetch<{ data: GlobalSetting }>('/api/admin/settings/global');
      setGlobalSetting(JSON.stringify(resp.data.value, null, 2));
      setGlobalVersion(resp.data.version);
    } catch (err) {
      if (err instanceof AdminApiError && err.status === 404) {
        setGlobalSetting('{}');
        setGlobalVersion(0);
      } else {
        setSettingError(err instanceof Error ? err.message : '请求失败');
      }
    } finally {
      setSettingLoading(false);
    }
  }, [adminFetch]);

  const fetchBoards = useCallback(async () => {
    setBoardsLoading(true);
    try {
      const resp = await adminFetch<{ data: BoardConfig[] }>('/api/admin/boards');
      setBoards(resp.data);
    } catch {
      // 静默处理
    } finally {
      setBoardsLoading(false);
    }
  }, [adminFetch]);

  const fetchVersions = useCallback(async () => {
    try {
      const resp = await adminFetch<{ data: ConfigVersions }>('/api/admin/config/version');
      setVersions(resp.data);
    } catch {
      // 静默处理
    }
  }, [adminFetch]);

  useEffect(() => {
    fetchGlobalSetting();
    fetchBoards();
    fetchVersions();
  }, [fetchGlobalSetting, fetchBoards, fetchVersions]);

  const handleSaveGlobal = useCallback(async () => {
    setSettingError(null);
    setSettingSaved(false);

    // 验证 JSON 格式
    let parsed: unknown;
    try {
      parsed = JSON.parse(globalSetting);
    } catch {
      setSettingError('JSON 格式错误');
      return;
    }

    try {
      await adminFetch('/api/admin/settings/global', {
        method: 'PUT',
        body: JSON.stringify({ value: parsed }),
      });
      setSettingSaved(true);
      fetchGlobalSetting();
      fetchVersions();
      setTimeout(() => setSettingSaved(false), 2000);
    } catch (err) {
      setSettingError(err instanceof Error ? err.message : '保存失败');
    }
  }, [adminFetch, globalSetting, fetchGlobalSetting, fetchVersions]);

  const handleImport = useCallback(async (file: File) => {
    setImportLoading(true);
    setImportResult(null);
    try {
      const result = await importConfig(file);
      setImportResult(`导入完成：创建 ${result.created} 项，跳过 ${result.skipped} 项${result.errors?.length ? `，错误 ${result.errors.length} 项` : ''}`);
      fetchVersions();
    } catch (err) {
      setImportResult(`导入失败：${err instanceof Error ? err.message : '未知错误'}`);
    } finally {
      setImportLoading(false);
      if (fileInputRef.current) {
        fileInputRef.current.value = '';
      }
    }
  }, [importConfig, fetchVersions]);

  const handleExport = useCallback(async () => {
    setExportLoading(true);
    try {
      // 直接 fetch 获取 YAML，不经过 adminFetch（因其强制 JSON 解析）
      const token = sessionStorage.getItem('relay-pulse-admin-token') ?? '';
      const resp = await fetch(`${API_BASE_URL}/api/admin/export`, {
        headers: {
          'X-Config-Token': token,
          Accept: 'application/x-yaml',
        },
      });
      if (!resp.ok) {
        throw new Error(`导出失败 (${resp.status})`);
      }
      const blob = await resp.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `relay-pulse-config-${new Date().toISOString().slice(0, 10)}.yaml`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
    } catch (err) {
      setImportResult(`导出失败：${err instanceof Error ? err.message : '未知错误'}`);
    } finally {
      setExportLoading(false);
    }
  }, []);

  const handleFileSelect = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      handleImport(file);
    }
  }, [handleImport]);

  return (
    <>
      <Helmet>
        <title>全局设置 | RP Admin</title>
      </Helmet>

      <div className="space-y-6">
        <h1 className="text-xl font-bold text-primary">全局设置</h1>

        {/* 配置版本信息 */}
        {versions && (
          <div className="bg-surface border border-muted rounded-lg p-4">
            <h2 className="text-sm font-semibold text-primary mb-3">配置版本</h2>
            <div className="grid grid-cols-2 sm:grid-cols-5 gap-3">
              {Object.entries(versions).map(([key, value]) => (
                <div key={key} className="text-center">
                  <div className="text-xs text-secondary mb-1">{key}</div>
                  <div className="text-lg font-bold text-accent tabular-nums">{value}</div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* 全局设置编辑器 */}
        <div className="bg-surface border border-muted rounded-lg p-4 space-y-3">
          <div className="flex items-center justify-between">
            <h2 className="text-sm font-semibold text-primary">
              全局设置 (global)
              {globalVersion > 0 && (
                <span className="ml-2 text-xs text-muted font-normal">v{globalVersion}</span>
              )}
            </h2>
            <div className="flex items-center gap-2">
              <button
                onClick={fetchGlobalSetting}
                disabled={settingLoading}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm bg-elevated border border-muted rounded-md text-secondary hover:text-primary transition-colors"
              >
                <RefreshCw className={`w-3.5 h-3.5 ${settingLoading ? 'animate-spin' : ''}`} />
              </button>
              <button
                onClick={handleSaveGlobal}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm bg-accent text-white rounded-md hover:bg-accent-strong transition-colors"
              >
                <Save className="w-3.5 h-3.5" />
                保存
              </button>
            </div>
          </div>

          <textarea
            value={globalSetting}
            onChange={(e) => setGlobalSetting(e.target.value)}
            rows={10}
            spellCheck={false}
            className="w-full px-3 py-2 bg-elevated border border-muted rounded-md text-primary text-sm font-mono focus:outline-none focus:ring-2 focus:ring-accent/50 resize-y"
          />

          {settingError && <p className="text-sm text-danger">{settingError}</p>}
          {settingSaved && <p className="text-sm text-success">保存成功</p>}
        </div>

        {/* Board 配置（只读） */}
        <div className="bg-surface border border-muted rounded-lg p-4 space-y-3">
          <h2 className="text-sm font-semibold text-primary">Board 配置</h2>
          {boardsLoading ? (
            <p className="text-sm text-secondary">加载中...</p>
          ) : boards.length === 0 ? (
            <p className="text-sm text-secondary">暂无 Board 配置</p>
          ) : (
            <div className="space-y-2">
              {boards.map((b) => (
                <div
                  key={b.board}
                  className="flex items-center justify-between px-3 py-2 bg-elevated rounded-md"
                >
                  <div>
                    <span className="font-mono text-xs text-accent">{b.board}</span>
                    <span className="ml-2 text-sm text-primary">{b.display_name}</span>
                    {b.description && (
                      <span className="ml-2 text-xs text-secondary">{b.description}</span>
                    )}
                  </div>
                  <span className="text-xs text-muted tabular-nums">order: {b.sort_order}</span>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* 配置导入/导出 */}
        <div className="bg-surface border border-muted rounded-lg p-4 space-y-3">
          <h2 className="text-sm font-semibold text-primary">配置导入/导出</h2>
          <p className="text-xs text-secondary">
            导出配置为 YAML 格式（API Key 脱敏），或从 YAML 文件导入监测项配置。
          </p>
          <div className="flex items-center gap-3">
            <input
              ref={fileInputRef}
              type="file"
              accept=".yaml,.yml"
              onChange={handleFileSelect}
              className="hidden"
            />
            <button
              onClick={() => fileInputRef.current?.click()}
              disabled={importLoading}
              className="inline-flex items-center gap-1.5 px-4 py-2 text-sm bg-elevated border border-muted rounded-md text-secondary hover:text-primary hover:border-accent transition-colors disabled:opacity-50"
            >
              <Upload className="w-4 h-4" />
              {importLoading ? '导入中...' : '导入 YAML'}
            </button>
            <button
              onClick={handleExport}
              disabled={exportLoading}
              className="inline-flex items-center gap-1.5 px-4 py-2 text-sm bg-accent text-white rounded-md hover:bg-accent-strong transition-colors disabled:opacity-50"
            >
              <Download className="w-4 h-4" />
              {exportLoading ? '导出中...' : '导出 YAML'}
            </button>
          </div>
          {importResult && (
            <p className={`text-sm ${importResult.includes('失败') ? 'text-danger' : 'text-success'}`}>
              {importResult}
            </p>
          )}
        </div>
      </div>
    </>
  );
}
