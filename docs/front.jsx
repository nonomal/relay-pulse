import React, { useState, useEffect, useMemo, useRef } from 'react';
import { Activity, Server, Clock, Filter, RefreshCw, CheckCircle, AlertTriangle, Zap, Shield, LayoutGrid, List, ArrowUpDown, ArrowUp, ArrowDown } from 'lucide-react';

// --- 配置与模拟数据逻辑 ---

const PROVIDERS = [
  { id: '88code', name: '88code', services: ['cc', 'cx'] },
  { id: 'xychatai', name: 'xychatai', services: ['cx'] },
  { id: 'duckcoding', name: 'duckcoding', services: ['cc', 'cx'] },
  { id: 'www.right.codes', name: 'www.right.codes', services: ['cx'] },
];

const TIME_RANGES = [
  { id: '24h', label: '近24小时', points: 24, unit: 'hour' },
  { id: '7d', label: '近7天', points: 7, unit: 'day' },
  { id: '15d', label: '近15天', points: 15, unit: 'day' },
  { id: '30d', label: '近30天', points: 30, unit: 'day' },
];

// 状态枚举
const STATUS = {
  AVAILABLE: { color: 'bg-emerald-500', text: 'text-emerald-400', glow: 'shadow-[0_0_10px_rgba(16,185,129,0.6)]', label: '可用', weight: 3 },
  DEGRADED: { color: 'bg-amber-400', text: 'text-amber-400', glow: 'shadow-[0_0_10px_rgba(251,191,36,0.6)]', label: '波动', weight: 2 },
  UNAVAILABLE: { color: 'bg-rose-500', text: 'text-rose-400', glow: 'shadow-[0_0_10px_rgba(244,63,94,0.6)]', label: '不可用', weight: 1 },
};

// 模拟API调用
const fetchMockData = (timeRangeId) => {
  return new Promise((resolve) => {
    setTimeout(() => {
      const rangeConfig = TIME_RANGES.find(r => r.id === timeRangeId);
      const count = rangeConfig.points;
      
      const data = [];

      PROVIDERS.forEach(provider => {
        provider.services.forEach(service => {
          const history = Array.from({ length: count }).map((_, index) => {
            const rand = Math.random();
            let statusKey = 'AVAILABLE';
            if (rand > 0.95) statusKey = 'UNAVAILABLE';
            else if (rand > 0.85) statusKey = 'DEGRADED';

            return {
              index,
              status: statusKey,
              timestamp: new Date(Date.now() - (count - index) * (rangeConfig.unit === 'hour' ? 3600000 : 86400000)).toISOString()
            };
          });

          const currentStatus = history[history.length - 1].status;
          const uptime = parseFloat((history.filter(h => h.status === 'AVAILABLE').length / count * 100).toFixed(1));

          data.push({
            id: `${provider.id}-${service}`,
            providerId: provider.id,
            providerName: provider.name,
            serviceType: service,
            history,
            currentStatus,
            uptime
          });
        });
      });

      resolve(data);
    }, 600); 
  });
};

// --- 组件部分 ---

const StatusDot = ({ status, size = 'md' }) => {
  const sizeClass = size === 'sm' ? 'w-2 h-2' : 'w-3 h-3';
  return (
    <div className={`${sizeClass} rounded-full ${STATUS[status].color} ${STATUS[status].glow} transition-all duration-500`} />
  );
};

// 优化后的热力图块：只负责触发事件，不负责显示 Tooltip
const HeatmapBlock = ({ point, width, height = 'h-8', onHover, onLeave }) => {
  return (
    <div 
      className={`${height} rounded-sm transition-all duration-200 hover:scale-110 hover:z-10 cursor-pointer ${STATUS[point.status].color} opacity-80 hover:opacity-100`}
      style={{ width: width }}
      onMouseEnter={(e) => onHover(e, point)}
      onMouseLeave={onLeave}
    />
  );
};

export default function ServiceMonitor() {
  const [loading, setLoading] = useState(true);
  const [rawData, setRawData] = useState([]);
  const [filterService, setFilterService] = useState('all');
  const [filterProvider, setFilterProvider] = useState('all');
  const [timeRange, setTimeRange] = useState('24h');
  const [viewMode, setViewMode] = useState('table');
  const [sortConfig, setSortConfig] = useState({ key: 'uptime', direction: 'desc' });
  
  // 全局 Tooltip 状态
  const [tooltip, setTooltip] = useState({ show: false, x: 0, y: 0, data: null });

  useEffect(() => {
    loadData(timeRange);
  }, [timeRange]);

  const loadData = async (range) => {
    setLoading(true);
    const data = await fetchMockData(range);
    setRawData(data);
    setLoading(false);
  };

  const handleSort = (key) => {
    let direction = 'desc';
    if (sortConfig.key === key && sortConfig.direction === 'desc') {
      direction = 'asc';
    }
    setSortConfig({ key, direction });
  };

  // Tooltip 处理函数
  const handleBlockHover = (e, point) => {
    const rect = e.target.getBoundingClientRect();
    // 计算 Tooltip 位置：显示在元素上方居中
    setTooltip({
      show: true,
      x: rect.left + rect.width / 2,
      y: rect.top - 10, // 上浮 10px
      data: point
    });
  };

  const handleBlockLeave = () => {
    setTooltip(prev => ({ ...prev, show: false }));
  };

  const processedData = useMemo(() => {
    let filtered = rawData.filter(item => {
      const matchService = filterService === 'all' || item.serviceType === filterService;
      const matchProvider = filterProvider === 'all' || item.providerId === filterProvider;
      return matchService && matchProvider;
    });

    if (sortConfig.key) {
      filtered.sort((a, b) => {
        let aValue = a[sortConfig.key];
        let bValue = b[sortConfig.key];

        if (sortConfig.key === 'currentStatus') {
          aValue = STATUS[a.currentStatus].weight;
          bValue = STATUS[b.currentStatus].weight;
        }

        if (aValue < bValue) return sortConfig.direction === 'asc' ? -1 : 1;
        if (aValue > bValue) return sortConfig.direction === 'asc' ? 1 : -1;
        return 0;
      });
    }

    return filtered;
  }, [rawData, filterService, filterProvider, sortConfig]);

  const stats = useMemo(() => {
    const total = processedData.length;
    const healthy = processedData.filter(i => i.currentStatus === 'AVAILABLE').length;
    const issues = total - healthy;
    return { total, healthy, issues };
  }, [processedData]);

  const SortIcon = ({ columnKey }) => {
    if (sortConfig.key !== columnKey) return <ArrowUpDown size={14} className="opacity-30 ml-1" />;
    return sortConfig.direction === 'asc' 
      ? <ArrowUp size={14} className="text-cyan-400 ml-1" /> 
      : <ArrowDown size={14} className="text-cyan-400 ml-1" />;
  };

  return (
    <div className="min-h-screen bg-slate-950 text-slate-200 font-sans selection:bg-cyan-500 selection:text-white overflow-x-hidden">
      {/* 全局 Tooltip (Fixed Position) */}
      {tooltip.show && tooltip.data && (
        <div 
          className="fixed z-50 pointer-events-none transition-opacity duration-200"
          style={{ 
            left: tooltip.x, 
            top: tooltip.y, 
            transform: 'translate(-50%, -100%)' // 居中并向上偏移
          }}
        >
          <div className="bg-slate-900/95 backdrop-blur-md text-slate-200 text-xs p-3 rounded-lg border border-slate-700 shadow-[0_10px_40px_-10px_rgba(0,0,0,0.8)] whitespace-nowrap flex flex-col items-center gap-1">
            <div className="text-slate-400">{new Date(tooltip.data.timestamp).toLocaleString()}</div>
            <div className={`font-bold text-sm ${STATUS[tooltip.data.status].text}`}>
              {STATUS[tooltip.data.status].label}
            </div>
            {/* 小三角箭头 */}
            <div className="absolute -bottom-1.5 left-1/2 -translate-x-1/2 w-3 h-3 bg-slate-900 border-r border-b border-slate-700 transform rotate-45"></div>
          </div>
        </div>
      )}

      {/* 背景装饰 */}
      <div className="fixed top-0 left-0 w-full h-full overflow-hidden pointer-events-none z-0">
        <div className="absolute top-[-10%] right-[-10%] w-[600px] h-[600px] bg-blue-600/10 rounded-full blur-[120px]" />
        <div className="absolute bottom-[-10%] left-[-10%] w-[600px] h-[600px] bg-cyan-600/10 rounded-full blur-[120px]" />
      </div>

      <div className="relative z-10 max-w-7xl mx-auto px-4 py-8 sm:px-6 lg:px-8">
        
        {/* 头部 Header */}
        <header className="flex flex-col md:flex-row justify-between items-start md:items-center mb-10 gap-4 border-b border-slate-800/50 pb-6">
          <div>
            <div className="flex items-center gap-3 mb-2">
              <div className="p-2 bg-cyan-500/10 rounded-lg border border-cyan-500/20">
                <Activity className="w-6 h-6 text-cyan-400" />
              </div>
              <h1 className="text-3xl font-bold bg-clip-text text-transparent bg-gradient-to-r from-cyan-400 via-blue-400 to-purple-400">
                Service Horizon
              </h1>
            </div>
            <p className="text-slate-400 text-sm flex items-center gap-2">
              <span className="inline-block w-2 h-2 rounded-full bg-emerald-500 animate-pulse"></span>
              实时监测API中转服务可用性矩阵
            </p>
          </div>

          <div className="flex gap-4 text-sm">
            <div className="px-4 py-2 rounded-xl bg-slate-900/50 border border-slate-800 backdrop-blur-sm flex items-center gap-3 shadow-lg">
              <div className="p-1.5 rounded-full bg-emerald-500/10 text-emerald-400"><CheckCircle size={16} /></div>
              <div>
                <div className="text-slate-400 text-xs">正常运行</div>
                <div className="font-mono font-bold text-emerald-400">{stats.healthy}</div>
              </div>
            </div>
            <div className="px-4 py-2 rounded-xl bg-slate-900/50 border border-slate-800 backdrop-blur-sm flex items-center gap-3 shadow-lg">
              <div className="p-1.5 rounded-full bg-rose-500/10 text-rose-400"><AlertTriangle size={16} /></div>
              <div>
                <div className="text-slate-400 text-xs">异常告警</div>
                <div className="font-mono font-bold text-rose-400">{stats.issues}</div>
              </div>
            </div>
          </div>
        </header>

        {/* 控制栏 Controls */}
        <div className="flex flex-col lg:flex-row gap-4 mb-8">
          <div className="flex-1 flex flex-wrap gap-4 items-center bg-slate-900/40 p-3 rounded-2xl border border-slate-800/50 backdrop-blur-md">
            <div className="flex items-center gap-2 text-slate-400 text-sm font-medium px-2">
              <Filter size={16} />
            </div>
            <select 
              value={filterProvider}
              onChange={(e) => setFilterProvider(e.target.value)}
              className="bg-slate-800 text-slate-200 text-sm rounded-lg border border-slate-700 focus:ring-2 focus:ring-cyan-500 focus:border-transparent p-2 outline-none transition-all hover:bg-slate-750"
            >
              <option value="all">所有服务商</option>
              {PROVIDERS.map(p => (
                <option key={p.id} value={p.id}>{p.name}</option>
              ))}
            </select>
            <select 
              value={filterService}
              onChange={(e) => setFilterService(e.target.value)}
              className="bg-slate-800 text-slate-200 text-sm rounded-lg border border-slate-700 focus:ring-2 focus:ring-cyan-500 focus:border-transparent p-2 outline-none transition-all hover:bg-slate-750"
            >
              <option value="all">所有服务</option>
              <option value="cc">Claude Code (cc)</option>
              <option value="cx">Codex (cx)</option>
            </select>
            <div className="w-px h-8 bg-slate-700 mx-2 hidden sm:block"></div>
            <div className="flex bg-slate-800 rounded-lg p-1 border border-slate-700">
              <button
                onClick={() => setViewMode('table')}
                className={`p-1.5 rounded ${viewMode === 'table' ? 'bg-slate-700 text-cyan-400 shadow' : 'text-slate-400 hover:text-slate-200'}`}
                title="表格视图"
              >
                <List size={18} />
              </button>
              <button
                onClick={() => setViewMode('grid')}
                className={`p-1.5 rounded ${viewMode === 'grid' ? 'bg-slate-700 text-cyan-400 shadow' : 'text-slate-400 hover:text-slate-200'}`}
                title="卡片视图"
              >
                <LayoutGrid size={18} />
              </button>
            </div>
            <button 
              onClick={() => loadData(timeRange)}
              className="ml-auto p-2 rounded-lg bg-cyan-500/10 text-cyan-400 hover:bg-cyan-500/20 transition-colors border border-cyan-500/20 group"
              title="刷新数据"
            >
              <RefreshCw size={18} className={`transition-transform ${loading ? 'animate-spin' : 'group-hover:rotate-180'}`} />
            </button>
          </div>

          <div className="bg-slate-900/40 p-2 rounded-2xl border border-slate-800/50 backdrop-blur-md flex items-center gap-1">
             {TIME_RANGES.map(range => (
               <button
                key={range.id}
                onClick={() => setTimeRange(range.id)}
                className={`px-3 py-2 text-xs font-medium rounded-xl transition-all duration-200 whitespace-nowrap ${
                  timeRange === range.id 
                    ? 'bg-gradient-to-br from-cyan-500 to-blue-600 text-white shadow-lg shadow-cyan-500/25' 
                    : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800'
                }`}
               >
                 {range.label}
               </button>
             ))}
          </div>
        </div>

        {loading && processedData.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-64 text-slate-500 gap-4">
            <div className="w-12 h-12 border-4 border-cyan-500/20 border-t-cyan-500 rounded-full animate-spin" />
            <p className="animate-pulse">正在同步数据节点...</p>
          </div>
        ) : processedData.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-20 text-slate-600">
            <Server size={64} className="mb-4 opacity-20" />
            <p className="text-lg">未找到符合条件的服务节点</p>
          </div>
        ) : (
          <>
            {viewMode === 'table' && (
              <div className="overflow-x-auto rounded-2xl border border-slate-800/50 shadow-xl">
                <table className="w-full text-left border-collapse bg-slate-900/40 backdrop-blur-sm">
                  <thead>
                    <tr className="border-b border-slate-700/50 text-slate-400 text-xs uppercase tracking-wider">
                      <th className="p-4 font-medium cursor-pointer hover:text-cyan-400 transition-colors" onClick={() => handleSort('providerName')}>
                        <div className="flex items-center">服务商 <SortIcon columnKey="providerName" /></div>
                      </th>
                      <th className="p-4 font-medium cursor-pointer hover:text-cyan-400 transition-colors" onClick={() => handleSort('serviceType')}>
                        <div className="flex items-center">服务 <SortIcon columnKey="serviceType" /></div>
                      </th>
                      <th className="p-4 font-medium cursor-pointer hover:text-cyan-400 transition-colors" onClick={() => handleSort('currentStatus')}>
                        <div className="flex items-center">当前状态 <SortIcon columnKey="currentStatus" /></div>
                      </th>
                      <th className="p-4 font-medium cursor-pointer hover:text-cyan-400 transition-colors" onClick={() => handleSort('uptime')}>
                        <div className="flex items-center">可用率(质量) <SortIcon columnKey="uptime" /></div>
                      </th>
                      <th className="p-4 font-medium w-1/3 min-w-[200px]">
                        <div className="flex items-center gap-2">
                          历史趋势 
                          <span className="text-[10px] normal-case opacity-50 border border-slate-700 px-1 rounded">{TIME_RANGES.find(r => r.id === timeRange).label}</span>
                        </div>
                      </th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800/50 text-sm">
                    {processedData.map((item) => (
                      <tr key={item.id} className="group hover:bg-slate-800/40 transition-colors">
                        <td className="p-4 font-medium text-slate-200">{item.providerName}</td>
                        <td className="p-4">
                           <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-mono border ${
                            item.serviceType === 'cc' ? 'border-purple-500/30 text-purple-300 bg-purple-500/10' : 'border-blue-500/30 text-blue-300 bg-blue-500/10'
                          }`}>
                            {item.serviceType === 'cc' && <Zap size={10} className="mr-1"/>}
                            {item.serviceType === 'cx' && <Shield size={10} className="mr-1"/>}
                            {item.serviceType.toUpperCase()}
                          </span>
                        </td>
                        <td className="p-4">
                          <div className="flex items-center gap-2">
                             <StatusDot status={item.currentStatus} size="sm" />
                             <span className={STATUS[item.currentStatus].text}>{STATUS[item.currentStatus].label}</span>
                          </div>
                        </td>
                        <td className="p-4 font-mono font-bold text-slate-200">
                          <span className={`${item.uptime >= 99 ? 'text-emerald-400' : item.uptime >= 90 ? 'text-amber-400' : 'text-rose-400'}`}>
                            {item.uptime}%
                          </span>
                        </td>
                        <td className="p-4">
                          <div className="flex gap-[2px] h-6 w-full max-w-xs">
                            {item.history.map((point, idx) => (
                              <HeatmapBlock 
                                key={idx} 
                                point={point} 
                                width={`${100 / item.history.length}%`} 
                                height="h-full"
                                onHover={handleBlockHover}
                                onLeave={handleBlockLeave}
                              />
                            ))}
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}

            {viewMode === 'grid' && (
              <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-6">
                {processedData.map((item) => (
                  <div key={item.id} className="group relative bg-slate-900/60 border border-slate-800 hover:border-cyan-500/30 rounded-2xl p-6 transition-all duration-300 hover:shadow-[0_0_30px_rgba(6,182,212,0.1)] backdrop-blur-sm overflow-hidden">
                    <div className={`absolute top-0 left-0 w-full h-1 ${STATUS[item.currentStatus].color}`} />
                    <div className="flex justify-between items-start mb-6">
                      <div className="flex gap-4 items-center">
                        <div className="w-12 h-12 rounded-xl bg-slate-800 flex items-center justify-center border border-slate-700 group-hover:border-slate-600 transition-colors">
                          {item.serviceType === 'cc' ? <Zap className="text-purple-400" size={24} /> : <Shield className="text-blue-400" size={24} />}
                        </div>
                        <div>
                          <div className="flex items-center gap-2">
                            <h3 className="text-lg font-bold text-slate-100">{item.providerName}</h3>
                            <span className={`px-2 py-0.5 rounded text-[10px] font-mono border ${item.serviceType === 'cc' ? 'border-purple-500/30 text-purple-300 bg-purple-500/10' : 'border-blue-500/30 text-blue-300 bg-blue-500/10'}`}>
                              {item.serviceType.toUpperCase()}
                            </span>
                          </div>
                          <div className="flex items-center gap-2 mt-1 text-xs text-slate-400 font-mono">
                            <Activity size={12} />
                            <span>可用率: {item.uptime}%</span>
                          </div>
                        </div>
                      </div>
                      <div className="flex flex-col items-end gap-1">
                         <div className="flex items-center gap-2 px-3 py-1 rounded-full bg-slate-800 border border-slate-700">
                            <StatusDot status={item.currentStatus} />
                            <span className={`text-xs font-bold ${STATUS[item.currentStatus].text}`}>{STATUS[item.currentStatus].label}</span>
                         </div>
                      </div>
                    </div>
                    <div>
                      <div className="flex justify-between text-xs text-slate-500 mb-2">
                        <span className="flex items-center gap-1"><Clock size={12}/> {timeRange === '24h' ? '24h' : `${parseInt(timeRange)}d`}</span>
                        <span>Now</span>
                      </div>
                      <div className="flex gap-[3px] h-10 w-full">
                        {item.history.map((point, idx) => (
                          <HeatmapBlock 
                            key={idx} 
                            point={point} 
                            width={`${100 / item.history.length}%`} 
                            onHover={handleBlockHover}
                            onLeave={handleBlockLeave}
                          />
                        ))}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}