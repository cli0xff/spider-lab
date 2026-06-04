import { useState, useRef, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { getTasks, cancelTask, restartTask, deleteTask, getTask, getLogs, getTaskResults } from '../lib/api';
import type { Task, LogEntry, ResultItem } from '../types';
import {
  RefreshCw, Loader2, AlertTriangle, ListChecks,
  XCircle, RotateCcw, Clock, ChevronLeft, ChevronRight,
  FileText, Table, Server, Bug, X, Database, Trash2,
} from 'lucide-react';

const statusConfig: Record<Task['status'], { label: string; bg: string; text: string; dot: string }> = {
  pending:   { label: '等待中',   bg: 'bg-secondary-container/50',      text: 'text-on-secondary-container', dot: 'bg-secondary' },
  running:   { label: '运行中',   bg: 'bg-primary-fixed/20',            text: 'text-primary',                dot: 'bg-primary pulse-live' },
  finished:  { label: '已完成',   bg: 'bg-primary-fixed/30',            text: 'text-primary',                dot: 'bg-primary' },
  error:     { label: '采集异常', bg: 'bg-error-container/30',          text: 'text-on-error-container',     dot: 'bg-tertiary' },
  cancelled: { label: '已取消',   bg: 'bg-surface-container-high',      text: 'text-on-surface-variant',     dot: 'bg-outline' },
  waiting:   { label: '等待Cookie', bg: 'bg-tertiary-container/30',     text: 'text-on-tertiary-container',  dot: 'bg-tertiary pulse-live' },
};

const statusFilters: { value: string; label: string }[] = [
  { value: '',          label: '全部状态' },
  { value: 'pending',   label: '等待中' },
  { value: 'running',   label: '运行中' },
  { value: 'finished',  label: '已完成' },
  { value: 'error',     label: '异常' },
  { value: 'cancelled', label: '已取消' },
  { value: 'waiting',   label: '等待Cookie' },
];

const PAGE_SIZE_OPTIONS = [20, 50, 100];

/** Format seconds into human-readable duration */
function formatDuration(startTs?: string, endTs?: string): string {
  if (!startTs || !endTs) return '-';
  const start = new Date(startTs).getTime();
  const end = new Date(endTs).getTime();
  if (isNaN(start) || isNaN(end) || end <= start) return '-';
  const s = Math.round((end - start) / 1000);
  if (s < 60) return `${s}秒`;
  const m = Math.floor(s / 60);
  const rem = s % 60;
  if (m < 60) return rem > 0 ? `${m}分${rem}秒` : `${m}分`;
  const h = Math.floor(m / 60);
  const remM = m % 60;
  return remM > 0 ? `${h}时${remM}分` : `${h}时`;
}

/** Format ISO date string to compact locale string */
function formatDate(dateStr: string | undefined): string {
  if (!dateStr) return '-';
  try {
    return new Date(dateStr).toLocaleString('zh-CN', {
      month: '2-digit', day: '2-digit',
      hour: '2-digit', minute: '2-digit',
    });
  } catch {
    return '-';
  }
}

/** Format ISO timestamp to HH:MM:SS for log display */
function formatTime(ts: string | undefined): string {
  if (!ts) return '--:--:--';
  try {
    return new Date(ts).toLocaleTimeString('zh-CN', {
      hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false,
    });
  } catch {
    return '--:--:--';
  }
}

const RESULT_PAGE_SIZE = 50;

export default function Tasks() {
  const queryClient = useQueryClient();
  const [statusFilter, setStatusFilter] = useState('');
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [toast, setToast] = useState<{ msg: string; type: 'success' | 'error' } | null>(null);
  const [selectedTaskId, setSelectedTaskId] = useState<string | null>(null);
  const [detailTab, setDetailTab] = useState<'results' | 'info' | 'logs'>('logs');
  const [resultPage, setResultPage] = useState(1);
  const logEndRef = useRef<HTMLDivElement>(null);

  const showToast = (msg: string, type: 'success' | 'error' = 'success') => {
    setToast({ msg, type });
    setTimeout(() => setToast(null), 3000);
  };

  // ── Query ──────────────────────────────────────────────────────────
  const { data, isLoading, error, isRefetching } = useQuery({
    queryKey: ['tasks', statusFilter, page, pageSize],
    queryFn: () =>
      getTasks({
        status: statusFilter || undefined,
        page,
        size: pageSize,
      }),
    refetchInterval: (query) => {
      const items: Task[] = (query.state.data as any)?.data ?? [];
      return items.some((t) => t.status === 'running' || t.status === 'pending' || t.status === 'waiting') ? 5000 : false;
    },
  });

  const tasks: Task[] = data?.data ?? [];
  const total: number = (data as any)?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));

  // ── Mutations ──────────────────────────────────────────────────────
  const cancelMut = useMutation({
    mutationFn: (id: string) => cancelTask(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['tasks'] });
      showToast('任务已取消');
    },
    onError: (e: Error) => showToast(e.message || '取消任务失败', 'error'),
  });

  const restartMut = useMutation({
    mutationFn: (id: string) => restartTask(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['tasks'] });
      showToast('任务已重新启动');
    },
    onError: (e: Error) => showToast(e.message || '重启任务失败', 'error'),
  });

  const deleteMut = useMutation({
    mutationFn: (id: string) => deleteTask(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['tasks'] });
      showToast('任务已删除');
    },
    onError: (e: Error) => showToast(e.message || '删除任务失败', 'error'),
  });

  // ── Derived counts (from current page – best-effort) ──────────────
  const runningCount = tasks.filter((t) => t.status === 'running').length;

  // ── Task Detail Queries ────────────────────────────────────────────
  const { data: detailTask, isLoading: detailLoading } = useQuery({
    queryKey: ['task-detail', selectedTaskId],
    queryFn: () => getTask(selectedTaskId!),
    enabled: !!selectedTaskId,
    refetchInterval: (query) => {
      const t = query.state.data as Task | undefined;
      return t?.status === 'running' ? 3000 : false;
    },
  });

  const { data: logsData } = useQuery({
    queryKey: ['task-logs', selectedTaskId],
    queryFn: () => getLogs(selectedTaskId!, { page: 1, size: 1000 }),
    enabled: !!selectedTaskId,
    refetchInterval: detailTask?.status === 'running' ? 3000 : false,
  });
  const taskLogs: LogEntry[] = logsData?.data ?? [];

  const { data: resultsData } = useQuery({
    queryKey: ['task-results', selectedTaskId, resultPage],
    queryFn: () => getTaskResults(selectedTaskId!, { page: resultPage, size: RESULT_PAGE_SIZE }),
    enabled: !!selectedTaskId,
  });
  const taskResults: ResultItem[] = resultsData?.data ?? [];
  const totalResults: number = (resultsData as any)?.total ?? 0;
  const resultTotalPages = Math.max(1, Math.ceil(totalResults / RESULT_PAGE_SIZE));

  // auto-scroll log viewer to bottom when new logs arrive
  useEffect(() => {
    if (detailTab === 'logs') {
      logEndRef.current?.scrollIntoView({ behavior: 'smooth' });
    }
  }, [taskLogs.length, detailTab]);

  const openTaskDetail = (taskId: string) => {
    setSelectedTaskId(taskId);
    setDetailTab('logs');
    setResultPage(1);
  };

  return (
    <div className="space-y-6">
      {/* Toast */}
      {toast && (
        <div
          className={`fixed top-20 right-6 z-50 px-4 py-3 rounded-lg text-sm font-medium tonal-depth transition-all ${
            toast.type === 'success'
              ? 'bg-primary-fixed text-primary'
              : 'bg-error-container text-on-error-container'
          }`}
        >
          {toast.msg}
        </div>
      )}

      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-on-surface">任务队列</h1>
          <p className="text-sm text-on-surface-variant mt-1">
            共计 {total} 个采集任务
            {runningCount > 0 && `，正在运行 ${runningCount} 个`}
          </p>
        </div>
        <button
          onClick={() => queryClient.invalidateQueries({ queryKey: ['tasks'] })}
          disabled={isRefetching}
          className="flex items-center gap-1.5 px-3 py-2 text-xs text-on-surface-variant bg-surface-container-high rounded-lg hover:bg-surface-container-highest transition-colors disabled:opacity-60"
        >
          <RefreshCw className={`w-3.5 h-3.5${isRefetching ? ' animate-spin' : ''}`} />
          {isRefetching ? '刷新中...' : '刷新'}
        </button>
      </div>

      {/* Status Filter Tabs */}
      <div className="flex gap-2">
        {statusFilters.map((f) => (
          <button
            key={f.value}
            onClick={() => {
              setStatusFilter(f.value);
              setPage(1);
            }}
            className={`px-3 py-1.5 text-xs rounded-lg transition-colors ${
              statusFilter === f.value
                ? 'bg-primary text-on-primary font-medium'
                : 'bg-surface-container-high text-on-surface-variant hover:bg-surface-container-highest'
            }`}
          >
            {f.label}
          </button>
        ))}
      </div>

      {/* Loading */}
      {isLoading && (
        <div className="bg-surface-container-lowest rounded-xl p-12 tonal-depth text-center">
          <Loader2 className="w-8 h-8 text-primary animate-spin mx-auto" />
          <p className="text-sm text-on-surface-variant mt-3">加载任务数据...</p>
        </div>
      )}

      {/* Error */}
      {error && !isLoading && (
        <div className="bg-surface-container-lowest rounded-xl p-8 tonal-depth text-center">
          <AlertTriangle className="w-10 h-10 text-tertiary mx-auto mb-3" />
          <p className="text-sm font-medium text-on-surface">加载失败</p>
          <p className="text-xs text-on-surface-variant mt-1">{(error as Error).message}</p>
          <button
            onClick={() => queryClient.invalidateQueries({ queryKey: ['tasks'] })}
            className="mt-4 px-4 py-2 text-xs font-medium text-primary bg-primary-fixed/20 rounded-lg hover:bg-primary-fixed/30 transition-colors"
          >
            重试
          </button>
        </div>
      )}

      {/* Empty */}
      {!isLoading && !error && tasks.length === 0 && (
        <div className="bg-surface-container-lowest rounded-xl p-12 tonal-depth text-center">
          <div className="w-16 h-16 rounded-2xl bg-primary-fixed/20 flex items-center justify-center mx-auto mb-4">
            <ListChecks className="w-8 h-8 text-primary" />
          </div>
          <p className="text-sm font-semibold text-on-surface">
            {statusFilter ? '没有找到匹配的任务' : '暂无任务'}
          </p>
          <p className="text-xs text-on-surface-variant mt-1.5">
            {statusFilter ? '请尝试调整筛选条件' : '运行爬虫后任务会出现在此列表'}
          </p>
        </div>
      )}

      {/* Tasks Table */}
      {!isLoading && !error && tasks.length > 0 && (
        <div className="bg-surface-container-lowest rounded-xl tonal-depth overflow-hidden">
          <table className="w-full">
            <thead>
              <tr className="bg-surface-container-low">
                <th className="text-left px-6 py-3 text-[10px] font-semibold text-on-surface-variant uppercase tracking-wider">爬虫 & 任务 ID</th>
                <th className="text-left px-6 py-3 text-[10px] font-semibold text-on-surface-variant uppercase tracking-wider">节点</th>
                <th className="text-left px-6 py-3 text-[10px] font-semibold text-on-surface-variant uppercase tracking-wider">当前状态</th>
                <th className="text-left px-6 py-3 text-[10px] font-semibold text-on-surface-variant uppercase tracking-wider">采集结果</th>
                <th className="text-left px-6 py-3 text-[10px] font-semibold text-on-surface-variant uppercase tracking-wider">耗时</th>
                <th className="text-left px-6 py-3 text-[10px] font-semibold text-on-surface-variant uppercase tracking-wider">创建时间</th>
                <th className="text-right px-6 py-3 text-[10px] font-semibold text-on-surface-variant uppercase tracking-wider">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-outline-variant/15">
              {tasks.map((task) => {
                const st = statusConfig[task.status];
                return (
                  <tr key={task._id} className="hover:bg-surface-container-low transition-colors">
                    {/* Spider & ID */}
                    <td className="px-6 py-4">
                      <p className="text-xs font-medium text-on-surface truncate max-w-[220px]">
                        {task.spider?.name || task.spider_id || '-'}
                      </p>
                      <p className="text-[11px] text-on-surface-variant font-mono mt-0.5 truncate max-w-[220px]">
                        {task._id}
                      </p>
                    </td>

                    {/* Node */}
                    <td className="px-6 py-4">
                      <span className="text-[11px] text-on-surface">
                        {task.node?.name || task.node_id || '-'}
                      </span>
                    </td>

                    {/* Status */}
                    <td className="px-6 py-4">
                      <span
                        className={`inline-flex items-center gap-1.5 px-2 py-1 rounded-full text-[11px] font-medium ${st.bg} ${st.text}`}
                      >
                        <span className={`w-1.5 h-1.5 rounded-full ${st.dot}`} />
                        {st.label}
                      </span>
                      {task.error && (
                        <p className="text-[10px] text-tertiary mt-1 truncate max-w-[160px]" title={task.error}>
                          {task.error}
                        </p>
                      )}
                    </td>

                    {/* Result Count */}
                    <td className="px-6 py-4">
                      <span className="text-xs font-mono text-on-surface">
                        {task.result_count != null
                          ? task.result_count.toLocaleString()
                          : '-'}
                      </span>
                    </td>

                    {/* Runtime Duration */}
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-1">
                        <Clock className="w-3 h-3 text-outline" />
                        <span className="text-[11px] text-on-surface-variant">
                          {formatDuration(task.start_ts, task.end_ts)}
                        </span>
                      </div>
                    </td>

                    {/* Created At */}
                    <td className="px-6 py-4">
                      <span className="text-[11px] text-on-surface-variant font-mono">
                        {formatDate(task.created_at)}
                      </span>
                    </td>

                    {/* Actions */}
                    <td className="px-6 py-4">
                      <div className="flex items-center justify-end gap-1">
                        {/* View Logs – for all statuses */}
                        <button
                          onClick={() => openTaskDetail(task._id)}
                          className="group/btn inline-flex items-center gap-1 p-1.5 text-[11px] font-medium text-primary bg-primary-fixed/20 rounded-lg hover:bg-primary-fixed/30 transition-all"
                          title="查看日志"
                        >
                          <FileText className="w-3.5 h-3.5" />
                          <span className="max-w-0 overflow-hidden group-hover/btn:max-w-[4em] transition-all duration-200">日志</span>
                        </button>

                        {/* Cancel – for running / pending tasks */}
                        {(task.status === 'running' || task.status === 'pending' || task.status === 'waiting') && (
                          <button
                            onClick={() => cancelMut.mutate(task._id)}
                            disabled={cancelMut.isPending}
                            className="group/btn inline-flex items-center gap-1 p-1.5 text-[11px] font-medium text-on-error-container bg-error-container/30 rounded-lg hover:bg-error-container/50 transition-all disabled:opacity-50"
                            title="取消任务"
                          >
                            <XCircle className="w-3.5 h-3.5" />
                            <span className="max-w-0 overflow-hidden group-hover/btn:max-w-[4em] transition-all duration-200">取消</span>
                          </button>
                        )}

                        {/* Restart – for error / cancelled tasks */}
                        {(task.status === 'error' || task.status === 'cancelled') && (
                          <button
                            onClick={() => restartMut.mutate(task._id)}
                            disabled={restartMut.isPending}
                            className="group/btn inline-flex items-center gap-1 p-1.5 text-[11px] font-medium text-on-primary bg-primary rounded-lg hover:opacity-90 transition-all disabled:opacity-50"
                            title="重新运行"
                          >
                            <RotateCcw className="w-3.5 h-3.5" />
                            <span className="max-w-0 overflow-hidden group-hover/btn:max-w-[4em] transition-all duration-200">重试</span>
                          </button>
                        )}

                        {/* Delete – for finished / error / cancelled tasks */}
                        {(task.status === 'finished' || task.status === 'error' || task.status === 'cancelled') && (
                          <button
                            onClick={() => deleteMut.mutate(task._id)}
                            disabled={deleteMut.isPending}
                            className="group/btn inline-flex items-center gap-1 p-1.5 text-[11px] font-medium text-on-error-container bg-error-container/30 rounded-lg hover:bg-error-container/50 transition-all disabled:opacity-50"
                            title="删除任务"
                          >
                            <Trash2 className="w-3.5 h-3.5" />
                            <span className="max-w-0 overflow-hidden group-hover/btn:max-w-[4em] transition-all duration-200">删除</span>
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>

          {/* Pagination */}
          <div className="flex items-center justify-between px-6 py-3 bg-surface-container-low">
            {/* Page size selector */}
            <div className="flex items-center gap-2">
              <span className="text-[11px] text-on-surface-variant">每页</span>
              <select
                value={pageSize}
                onChange={(e) => {
                  setPageSize(Number(e.target.value));
                  setPage(1);
                }}
                className="text-[11px] px-2 py-1 rounded-lg bg-surface-container-highest text-on-surface outline-none focus:ring-2 focus:ring-primary/30"
              >
                {PAGE_SIZE_OPTIONS.map((s) => (
                  <option key={s} value={s}>{s} 条</option>
                ))}
              </select>
              <span className="text-[11px] text-on-surface-variant">
                共 {total} 条
              </span>
            </div>

            {/* Page controls */}
            <div className="flex items-center gap-1">
              <button
                onClick={() => setPage((p) => Math.max(1, p - 1))}
                disabled={page <= 1}
                className="p-1.5 rounded-lg hover:bg-surface-container-highest transition-colors disabled:opacity-30 disabled:cursor-not-allowed"
              >
                <ChevronLeft className="w-4 h-4 text-on-surface-variant" />
              </button>
              <span className="text-[11px] text-on-surface px-2 font-medium">
                {page} / {totalPages}
              </span>
              <button
                onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                disabled={page >= totalPages}
                className="p-1.5 rounded-lg hover:bg-surface-container-highest transition-colors disabled:opacity-30 disabled:cursor-not-allowed"
              >
                <ChevronRight className="w-4 h-4 text-on-surface-variant" />
              </button>
            </div>
          </div>
        </div>
      )}

      {/* ── Task Detail Modal ────────────────────────────────────── */}
      {selectedTaskId && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40" onClick={() => setSelectedTaskId(null)}>
          <div className="bg-surface-container-lowest rounded-2xl w-[760px] max-h-[85vh] overflow-hidden tonal-depth" onClick={(e) => e.stopPropagation()}>
            {/* Modal Header */}
            <div className="flex items-center justify-between px-6 py-4 bg-surface-container-low">
              <h2 className="text-sm font-bold text-on-surface flex items-center gap-2">
                <FileText className="w-4 h-4 text-primary" />
                任务详情
                {detailTask && (
                  <span className="text-xs font-normal text-on-surface-variant">
                    — {detailTask.spider?.name || detailTask.cmd || detailTask._id}
                  </span>
                )}
              </h2>
              <button onClick={() => setSelectedTaskId(null)} className="p-1 rounded-lg hover:bg-surface-container-highest transition-colors">
                <X className="w-4 h-4 text-on-surface-variant" />
              </button>
            </div>

            {/* Tabs */}
            <div className="flex gap-1 px-6 pt-3 pb-0 bg-surface-container-low">
              {([
                { key: 'results' as const, label: '采集数据', icon: Table, count: totalResults },
                { key: 'info' as const, label: '任务信息', icon: Server },
                { key: 'logs' as const, label: '运行日志', icon: FileText, count: taskLogs.length },
              ]).map((tab) => (
                <button
                  key={tab.key}
                  onClick={() => setDetailTab(tab.key)}
                  className={`flex items-center gap-1.5 px-4 py-2 text-xs rounded-t-lg transition-colors ${
                    detailTab === tab.key
                      ? 'bg-surface-container-lowest text-primary font-medium'
                      : 'text-on-surface-variant hover:bg-surface-container-highest'
                  }`}
                >
                  <tab.icon className="w-3.5 h-3.5" />
                  {tab.label}
                  {tab.count != null && tab.count > 0 && (
                    <span className="text-[10px] bg-primary/10 text-primary px-1.5 rounded-full">{tab.count}</span>
                  )}
                </button>
              ))}
            </div>

            {detailLoading ? (
              <div className="flex items-center justify-center py-16">
                <Loader2 className="w-6 h-6 text-primary animate-spin" />
              </div>
            ) : (
              <div className="overflow-y-auto max-h-[calc(85vh-120px)]">
                {/* Results Tab */}
                {detailTab === 'results' && (
                  <div className="p-4">
                    {taskResults.length === 0 ? (
                      <div className="text-center py-12">
                        <Database className="w-8 h-8 text-outline mx-auto mb-3" />
                        <p className="text-xs text-on-surface-variant">暂无采集数据</p>
                        <p className="text-[10px] text-outline mt-1">运行爬虫后，stdout 输出的 JSON 行将在此显示</p>
                      </div>
                    ) : (
                      <>
                        <div className="text-[10px] text-on-surface-variant mb-2">
                          共 {totalResults} 条数据，当前第 {resultPage} / {resultTotalPages} 页
                        </div>
                        <div className="overflow-x-auto rounded-lg">
                          <table className="w-full text-xs">
                            <thead>
                              <tr className="bg-surface-container-high">
                                <th className="text-left px-3 py-2 text-[10px] font-semibold text-on-surface-variant uppercase tracking-wider w-8">#</th>
                                {taskResults.length > 0 && Object.keys(taskResults[0].item).slice(0, 6).map((key) => (
                                  <th key={key} className="text-left px-3 py-2 text-[10px] font-semibold text-on-surface-variant uppercase tracking-wider max-w-[180px]">
                                    {key}
                                  </th>
                                ))}
                              </tr>
                            </thead>
                            <tbody className="divide-y divide-outline-variant/10">
                              {taskResults.map((r, idx) => {
                                const keys = Object.keys(r.item).slice(0, 6);
                                return (
                                  <tr key={r._id} className="hover:bg-surface-container-low transition-colors">
                                    <td className="px-3 py-2 text-[10px] text-on-surface-variant">{(resultPage - 1) * RESULT_PAGE_SIZE + idx + 1}</td>
                                    {keys.map((key) => {
                                      const val = r.item[key];
                                      const display = typeof val === 'object' ? JSON.stringify(val) : String(val ?? '');
                                      return (
                                        <td key={key} className="px-3 py-2 text-xs text-on-surface max-w-[180px] truncate" title={display}>
                                          {display}
                                        </td>
                                      );
                                    })}
                                  </tr>
                                );
                              })}
                            </tbody>
                          </table>
                        </div>
                        {resultTotalPages > 1 && (
                          <div className="flex items-center justify-center gap-2 mt-3">
                            <button
                              disabled={resultPage <= 1}
                              onClick={() => setResultPage((p) => Math.max(1, p - 1))}
                              className="px-3 py-1 text-xs rounded-lg bg-surface-container-high text-on-surface-variant hover:bg-surface-container-highest disabled:opacity-30 transition-colors"
                            >
                              上一页
                            </button>
                            <span className="text-[11px] text-on-surface-variant">{resultPage} / {resultTotalPages}</span>
                            <button
                              disabled={resultPage >= resultTotalPages}
                              onClick={() => setResultPage((p) => Math.min(resultTotalPages, p + 1))}
                              className="px-3 py-1 text-xs rounded-lg bg-surface-container-high text-on-surface-variant hover:bg-surface-container-highest disabled:opacity-30 transition-colors"
                            >
                              下一页
                            </button>
                          </div>
                        )}
                      </>
                    )}
                  </div>
                )}

                {/* Info Tab */}
                {detailTab === 'info' && detailTask && (
                  <div>
                    <div className="grid grid-cols-2 gap-4 px-6 py-4">
                      <div>
                        <p className="text-[10px] text-on-surface-variant uppercase tracking-wider">任务 ID</p>
                        <p className="text-xs font-mono text-on-surface mt-1">{detailTask._id}</p>
                      </div>
                      <div>
                        <p className="text-[10px] text-on-surface-variant uppercase tracking-wider">状态</p>
                        <span className={`inline-block mt-1 px-2 py-0.5 text-[11px] rounded font-medium ${(statusConfig[detailTask.status] ?? statusConfig.finished).bg} ${(statusConfig[detailTask.status] ?? statusConfig.finished).text}`}>
                          {(statusConfig[detailTask.status] ?? statusConfig.finished).label}
                        </span>
                      </div>
                      <div>
                        <p className="text-[10px] text-on-surface-variant uppercase tracking-wider flex items-center gap-1"><Server className="w-3 h-3" />爬虫</p>
                        <p className="text-xs text-on-surface mt-1">{detailTask.spider?.name ?? detailTask.cmd ?? '-'}</p>
                      </div>
                      <div>
                        <p className="text-[10px] text-on-surface-variant uppercase tracking-wider flex items-center gap-1"><Server className="w-3 h-3" />执行节点</p>
                        <p className="text-xs text-on-surface mt-1">{detailTask.node?.name ?? '-'}</p>
                      </div>
                      <div>
                        <p className="text-[10px] text-on-surface-variant uppercase tracking-wider flex items-center gap-1"><Clock className="w-3 h-3" />创建时间</p>
                        <p className="text-xs font-mono text-on-surface mt-1">{formatTime(detailTask.created_at)}</p>
                      </div>
                      <div>
                        <p className="text-[10px] text-on-surface-variant uppercase tracking-wider flex items-center gap-1"><Clock className="w-3 h-3" />耗时</p>
                        <p className="text-xs font-mono text-on-surface mt-1">{formatDuration(detailTask.start_ts, detailTask.end_ts)}</p>
                      </div>
                      <div>
                        <p className="text-[10px] text-on-surface-variant uppercase tracking-wider">采集结果数</p>
                        <p className="text-xs font-bold text-on-surface mt-1">{detailTask.result_count != null ? detailTask.result_count.toLocaleString() : '-'}</p>
                      </div>
                      <div>
                        <p className="text-[10px] text-on-surface-variant uppercase tracking-wider">执行命令</p>
                        <p className="text-xs font-mono text-on-surface mt-1">{detailTask.cmd ?? '-'}</p>
                      </div>
                    </div>
                    {detailTask.error && (
                      <div className="mx-6 mb-4 p-3 rounded-lg bg-error-container/20">
                        <p className="text-[10px] text-on-error-container uppercase tracking-wider flex items-center gap-1 mb-1"><Bug className="w-3 h-3" />错误信息</p>
                        <p className="text-xs text-on-error-container font-mono whitespace-pre-wrap">{detailTask.error}</p>
                      </div>
                    )}
                  </div>
                )}

                {/* Logs Tab */}
                {detailTab === 'logs' && (
                  <div className="p-4">
                    {taskLogs.length === 0 ? (
                      <p className="text-xs text-on-surface-variant py-8 text-center">暂无日志</p>
                    ) : (
                      <div className="bg-[#0a1414] rounded-lg p-3 max-h-[400px] overflow-y-auto font-mono text-[11px] leading-[1.7] space-y-0.5">
                        {taskLogs.map((log) => (
                          <div key={log._id ?? log.timestamp} className="flex gap-2">
                            <span className="text-emerald-700 shrink-0">{formatTime(log.timestamp)}</span>
                            <span className={`shrink-0 w-10 font-bold ${log.level === 'error' ? 'text-red-400' : log.level === 'warn' ? 'text-amber-400' : 'text-emerald-400'}`}>[{log.level?.toUpperCase()}]</span>
                            <span className="text-emerald-200">{log.content}</span>
                          </div>
                        ))}
                        <div ref={logEndRef} />
                      </div>
                    )}
                  </div>
                )}
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
