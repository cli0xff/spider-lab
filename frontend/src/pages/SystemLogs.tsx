import { useState, useEffect, useRef, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { getSystemLogs } from '../lib/api';
import type { SystemLogEntry } from '../types';
import { Search, Trash2, Download, Loader2, AlertTriangle } from 'lucide-react';

const PAGE_SIZE = 200;

const levelColors = {
  INFO: 'text-emerald-400',
  WARN: 'text-amber-400',
  ERROR: 'text-red-400',
  DEBUG: 'text-blue-400',
} as const;

function formatTime(timestamp: string): string {
  try {
    const d = new Date(timestamp);
    const h = String(d.getHours()).padStart(2, '0');
    const m = String(d.getMinutes()).padStart(2, '0');
    const s = String(d.getSeconds()).padStart(2, '0');
    const ms = String(d.getMilliseconds()).padStart(3, '0');
    return `${h}:${m}:${s}.${ms}`;
  } catch {
    return timestamp;
  }
}

export default function SystemLogs() {
  const [search, setSearch] = useState('');
  const [level, setLevel] = useState('all');
  const [page, setPage] = useState(1);
  const [autoScroll, setAutoScroll] = useState(true);
  const termRef = useRef<HTMLDivElement>(null);

  // Reset to page 1 when level filter changes
  useEffect(() => {
    setPage(1);
  }, [level]);

  const queryParams: Record<string, any> = { page, size: PAGE_SIZE };
  if (level !== 'all') queryParams.level = level.toLowerCase();

  const { data, isLoading, error } = useQuery({
    queryKey: ['systemLogs', level, page],
    queryFn: () => getSystemLogs(queryParams),
  });

  const allLogs: SystemLogEntry[] = (data as any)?.data ?? [];
  const total: number = (data as any)?.total ?? 0;

  // Client-side search filtering
  const filtered = useMemo(() => {
    if (!search) return allLogs;
    const q = search.toLowerCase();
    return allLogs.filter(
      (l) =>
        l.message.toLowerCase().includes(q) ||
        l.module.toLowerCase().includes(q),
    );
  }, [allLogs, search]);

  // Summary counts derived from current page data & total
  const errorCount = allLogs.filter((l) => l.level === 'error').length;
  const warnCount = allLogs.filter((l) => l.level === 'warn').length;
  const infoCount = allLogs.filter((l) => l.level === 'info').length;
  const debugCount = allLogs.filter((l) => l.level === 'debug').length;

  // Auto-scroll when new data arrives
  useEffect(() => {
    if (autoScroll && termRef.current) {
      termRef.current.scrollTop = termRef.current.scrollHeight;
    }
  }, [autoScroll, filtered]);

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-on-surface">系统日志</h1>
          <p className="text-sm text-on-surface-variant mt-1">来自分布式网络节点的实时遥测和错误报告</p>
        </div>
        <div className="flex items-center gap-2">
          <span className="flex items-center gap-1.5 px-3 py-1 rounded-full bg-primary-fixed/20 text-[11px] text-primary font-medium">
            <span className="relative flex h-1.5 w-1.5">
              <span className="pulse-live absolute inline-flex h-full w-full rounded-full bg-primary opacity-75" />
              <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-primary" />
            </span>
            实时推送已激活
          </span>
        </div>
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-4 gap-4">
        {[
          { label: '日志总数', value: total.toLocaleString(), sub: `当前页 ${allLogs.length} 条`, accent: 'border-l-4 border-primary' },
          { label: '错误日志', value: String(errorCount), sub: '当前页 ERROR 数量', accent: 'border-l-4 border-tertiary-container' },
          { label: '警告日志', value: String(warnCount), sub: '当前页 WARN 数量', accent: 'border-l-4 border-amber-400' },
          { label: '信息 / 调试', value: `${infoCount} / ${debugCount}`, sub: '当前页 INFO / DEBUG', accent: 'border-l-4 border-blue-400' },
        ].map((c) => (
          <div key={c.label} className={`bg-surface-container-lowest rounded-xl p-4 tonal-depth ${c.accent}`}>
            <p className="text-[11px] text-on-surface-variant">{c.label}</p>
            <p className="text-xl font-bold text-on-surface mt-1">{c.value}</p>
            <p className="text-[10px] text-on-surface-variant mt-1">{c.sub}</p>
          </div>
        ))}
      </div>

      {/* Terminal */}
      <div className="rounded-xl overflow-hidden tonal-depth">
        {/* Terminal Header */}
        <div className="bg-[#0d1c1c] px-5 py-3 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="relative flex-1 max-w-xs">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-emerald-600" />
              <input
                type="text"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="搜索日志内容..."
                className="pl-8 pr-3 py-1.5 text-xs bg-[#142828] text-emerald-100 rounded-lg w-56 outline-none focus:ring-1 focus:ring-emerald-600/50 placeholder:text-emerald-800"
              />
            </div>
            <select
              value={level}
              onChange={(e) => setLevel(e.target.value)}
              className="text-xs bg-[#142828] text-emerald-100 rounded-lg px-3 py-1.5 outline-none"
            >
              <option value="all">所有级别</option>
              <option value="info">INFO</option>
              <option value="warn">WARN</option>
              <option value="error">ERROR</option>
              <option value="debug">DEBUG</option>
            </select>
          </div>
          <div className="flex items-center gap-2">
            <label className="flex items-center gap-1.5 cursor-pointer">
              <input type="checkbox" checked={autoScroll} onChange={(e) => setAutoScroll(e.target.checked)} className="accent-emerald-500 w-3 h-3" />
              <span className="text-[11px] text-emerald-500">自动滚动</span>
            </label>
            <button className="px-2 py-1 text-[11px] text-emerald-500 bg-[#142828] rounded hover:bg-[#1a3333] transition-colors flex items-center gap-1">
              <Download className="w-3 h-3" />导出
            </button>
            <button className="px-2 py-1 text-[11px] text-emerald-500 bg-[#142828] rounded hover:bg-[#1a3333] transition-colors flex items-center gap-1">
              <Trash2 className="w-3 h-3" />清空
            </button>
          </div>
        </div>

        {/* Log Area */}
        <div ref={termRef} className="bg-[#0a1414] p-4 h-[420px] overflow-y-auto font-mono text-[12px] leading-[1.8]">
          {isLoading && (
            <div className="flex flex-col items-center justify-center h-full gap-3">
              <Loader2 className="w-6 h-6 text-emerald-500 animate-spin" />
              <span className="text-emerald-600 text-xs">加载日志数据...</span>
            </div>
          )}

          {error && !isLoading && (
            <div className="flex flex-col items-center justify-center h-full gap-3">
              <AlertTriangle className="w-8 h-8 text-red-400" />
              <span className="text-red-400 text-xs">加载失败: {(error as Error).message}</span>
              <span className="text-emerald-700 text-[10px]">请检查网络连接后重试</span>
            </div>
          )}

          {!isLoading && !error && filtered.length === 0 && (
            <div className="flex flex-col items-center justify-center h-full gap-2">
              <span className="text-emerald-700 text-xs">
                {search ? '没有匹配的日志条目' : '暂无日志数据'}
              </span>
            </div>
          )}

          {!isLoading && !error && filtered.map((log) => {
            const upperLevel = log.level.toUpperCase() as keyof typeof levelColors;
            return (
              <div key={log._id} className="flex gap-3 hover:bg-[#142828]/50 px-2 py-0.5 rounded group">
                <span className="text-emerald-700 shrink-0">{formatTime(log.timestamp)}</span>
                <span className={`shrink-0 w-12 font-bold ${levelColors[upperLevel] ?? 'text-emerald-400'}`}>[{upperLevel}]</span>
                <span className="text-emerald-600 shrink-0 w-28 truncate" title={log.module}>{log.module}</span>
                <span className="text-emerald-200">{log.message}</span>
                {log.detail && (
                  <span className="text-emerald-800 text-[10px] hidden group-hover:inline ml-2" title={log.detail}>
                    [{log.detail.length > 60 ? log.detail.slice(0, 60) + '...' : log.detail}]
                  </span>
                )}
              </div>
            );
          })}

          {!isLoading && !error && (
            <div className="flex items-center gap-1 mt-2 px-2">
              <span className="text-emerald-600">root@observer:~#</span>
              <span className="w-2 h-4 bg-emerald-400 animate-pulse" />
            </div>
          )}
        </div>

        {/* Status Bar */}
        <div className="bg-[#0d1c1c] px-5 py-2 flex items-center justify-between text-[10px] text-emerald-700">
          <div className="flex items-center gap-4">
            <span>总计: {total.toLocaleString()} 条</span>
            <span>当前页: {filtered.length} 条</span>
            <span>UTF-8</span>
            {totalPages > 1 && (
              <span className="flex items-center gap-1.5">
                <button
                  disabled={page <= 1}
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                  className="px-1.5 py-0.5 rounded bg-[#142828] hover:bg-[#1a3333] disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
                >
                  ‹ 上一页
                </button>
                <span>{page} / {totalPages}</span>
                <button
                  disabled={page >= totalPages}
                  onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                  className="px-1.5 py-0.5 rounded bg-[#142828] hover:bg-[#1a3333] disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
                >
                  下一页 ›
                </button>
              </span>
            )}
          </div>
          <div className="flex items-center gap-2">
            <span className={`w-1.5 h-1.5 rounded-full ${error ? 'bg-red-500' : 'bg-emerald-500'}`} />
            <span>{error ? '连接异常' : '已连接'}</span>
          </div>
        </div>
      </div>
    </div>
  );
}
