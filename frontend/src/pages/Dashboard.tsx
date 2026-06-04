import { useQuery, useQueryClient } from '@tanstack/react-query';
import { getStatsOverview, getTaskStats, getNodes } from '../lib/api';
import type { StatsOverview, DailyTaskStat, Node } from '../types';
import {
  HardDrive, TrendingUp, Server, HeartPulse,
  Download, RefreshCw, AlertTriangle, Loader2, Bug,
} from 'lucide-react';
import {
  LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer,
  PieChart, Pie, Cell,
} from 'recharts';

const STATUS_COLORS: Record<string, string> = {
  pending: '#8bd6b6',
  running: '#065f46',
  finished: '#004532',
  error: '#e0e3e5',
  waiting: '#d4a574',
};

const STATUS_LABELS: Record<string, string> = {
  pending: '等待中',
  running: '运行中',
  finished: '已完成',
  error: '错误',
  waiting: '等待Cookie',
};

export default function Dashboard() {
  const queryClient = useQueryClient();

  const now = new Date();
  const timestamp = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')} ${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}:${String(now.getSeconds()).padStart(2, '0')} UTC+8`;

  // ─── Queries ─────────────────────────────────────────────
  const {
    data: overview,
    isLoading: overviewLoading,
    error: overviewError,
  } = useQuery({
    queryKey: ['stats-overview'],
    queryFn: getStatsOverview,
  });

  const {
    data: taskStats,
    isLoading: taskStatsLoading,
    error: taskStatsError,
  } = useQuery({
    queryKey: ['task-stats'],
    queryFn: getTaskStats,
  });

  const {
    data: nodesData,
    isLoading: nodesLoading,
    error: nodesError,
  } = useQuery({
    queryKey: ['dashboard-nodes'],
    queryFn: () => getNodes({ page: 1, size: 200 }),
  });

  const isLoading = overviewLoading || taskStatsLoading || nodesLoading;
  const error = overviewError || taskStatsError || nodesError;

  // ─── Derived data ────────────────────────────────────────
  const stats = overview as StatsOverview | undefined;
  const dailyStats = (taskStats ?? []) as DailyTaskStat[];
  const allNodes: Node[] = (nodesData as any)?.data ?? [];

  // Metric cards from real overview data
  const metrics = stats
    ? [
        {
          label: '总任务数',
          value: stats.tasks.total.toLocaleString(),
          change: `已完成 ${stats.tasks.finished.toLocaleString()} 个`,
          up: true,
          icon: HardDrive,
        },
        {
          label: '运行中任务',
          value: stats.tasks.running.toLocaleString(),
          change: `等待中 ${stats.tasks.pending.toLocaleString()} 个`,
          up: stats.tasks.running > 0,
          icon: TrendingUp,
        },
        {
          label: '运行节点数',
          value: `${stats.nodes.online}`,
          change: `共 ${stats.nodes.total} 个节点`,
          up: stats.nodes.online === stats.nodes.total,
          icon: Server,
          progress: stats.nodes.total > 0
            ? Math.round((stats.nodes.online / stats.nodes.total) * 100)
            : 0,
        },
        {
          label: '爬虫总数',
          value: stats.spiders.toLocaleString(),
          change: `活跃定时任务 ${stats.active_schedules} 个`,
          up: true,
          icon: Bug,
        },
      ]
    : [];

  // Chart data – daily task stats
  const chartData = dailyStats.map((d) => ({
    date: d.date.slice(5), // "MM-DD"
    total: d.total,
    finished: d.finished,
    error: d.error,
  }));

  // Pie chart – task status distribution
  const pieData = stats
    ? (['pending', 'running', 'finished', 'error'] as const)
        .map((key) => ({
          name: STATUS_LABELS[key],
          value: stats.tasks[key],
          color: STATUS_COLORS[key],
        }))
        .filter((d) => d.value > 0)
    : [];

  const pieTotal = pieData.reduce((s, d) => s + d.value, 0);

  // Top nodes by CPU
  const topNodes = [...allNodes]
    .sort((a, b) => (b.cpu_usage || 0) - (a.cpu_usage || 0))
    .slice(0, 6);

  // ─── Refresh handler ────────────────────────────────────
  const handleRefresh = () => {
    queryClient.invalidateQueries({ queryKey: ['stats-overview'] });
    queryClient.invalidateQueries({ queryKey: ['task-stats'] });
    queryClient.invalidateQueries({ queryKey: ['dashboard-nodes'] });
  };

  // ─── Loading state ──────────────────────────────────────
  if (isLoading) {
    return (
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <h1 className="text-xl font-bold text-on-surface">控制台概览</h1>
        </div>
        <div className="bg-surface-container-lowest rounded-xl p-12 tonal-depth text-center">
          <Loader2 className="w-8 h-8 text-primary animate-spin mx-auto" />
          <p className="text-sm text-on-surface-variant mt-3">加载仪表盘数据...</p>
        </div>
      </div>
    );
  }

  // ─── Error state ────────────────────────────────────────
  if (error) {
    return (
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <h1 className="text-xl font-bold text-on-surface">控制台概览</h1>
        </div>
        <div className="bg-surface-container-lowest rounded-xl p-8 tonal-depth text-center">
          <AlertTriangle className="w-10 h-10 text-tertiary mx-auto mb-3" />
          <p className="text-sm font-medium text-on-surface">加载失败</p>
          <p className="text-xs text-on-surface-variant mt-1">{(error as Error).message}</p>
          <button
            onClick={handleRefresh}
            className="mt-4 px-4 py-2 text-xs font-medium text-primary bg-primary-fixed/20 rounded-lg hover:bg-primary-fixed/30 transition-colors"
          >
            重试
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <h1 className="text-xl font-bold text-on-surface">控制台概览</h1>
          <div className="flex items-center gap-2 px-3 py-1 rounded-full bg-primary-fixed/20">
            <span className="relative flex h-1.5 w-1.5">
              <span className="pulse-live absolute inline-flex h-full w-full rounded-full bg-primary opacity-75" />
              <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-primary" />
            </span>
            <span className="text-[11px] text-primary font-medium">实时同步</span>
          </div>
          <span className="text-xs text-outline font-mono">{timestamp}</span>
        </div>
        <div className="flex items-center gap-2">
          <button className="flex items-center gap-1.5 px-3 py-2 text-xs text-on-surface-variant bg-surface-container-high rounded-lg hover:bg-surface-container-highest transition-colors">
            <Download className="w-3.5 h-3.5" />
            导出月度报表
          </button>
          <button
            onClick={handleRefresh}
            className="flex items-center gap-1.5 px-3 py-2 text-xs text-on-surface-variant bg-surface-container-high rounded-lg hover:bg-surface-container-highest transition-colors"
          >
            <RefreshCw className="w-3.5 h-3.5" />
            刷新视图
          </button>
        </div>
      </div>

      {/* Metric Cards */}
      <div className="grid grid-cols-4 gap-4">
        {metrics.map((m) => (
          <div key={m.label} className="bg-surface-container-lowest rounded-xl p-5 tonal-depth border-l-4 border-primary/80">
            <div className="flex items-start justify-between">
              <div className="space-y-3">
                <p className="text-xs text-on-surface-variant font-medium">{m.label}</p>
                <p className="text-2xl font-bold text-on-surface tracking-tight">{m.value}</p>
                <p className="text-[11px] text-primary flex items-center gap-1">
                  <TrendingUp className="w-3 h-3" />
                  {m.change}
                </p>
              </div>
              <div className="w-9 h-9 rounded-lg bg-primary-fixed/20 flex items-center justify-center">
                <m.icon className="w-[18px] h-[18px] text-primary" />
              </div>
            </div>
            {m.progress !== undefined && (
              <div className="mt-3 h-1.5 rounded-full bg-surface-container-high overflow-hidden">
                <div
                  className="h-full rounded-full bg-gradient-to-r from-primary to-primary-container transition-all"
                  style={{ width: `${m.progress}%` }}
                />
              </div>
            )}
          </div>
        ))}
      </div>

      {/* Bento Grid */}
      <div className="grid grid-cols-12 gap-4">
        {/* Task Trend Chart (7 days) */}
        <div className="col-span-8 bg-surface-container-lowest rounded-xl p-6 tonal-depth">
          <div className="flex items-center justify-between mb-6">
            <div>
              <h3 className="text-sm font-semibold text-on-surface">任务趋势 (近7天)</h3>
              <p className="text-xs text-on-surface-variant mt-1">
                总计: {dailyStats.reduce((s, d) => s + d.total, 0)} 个任务
              </p>
            </div>
            <div className="flex gap-2">
              <span className="px-2 py-0.5 text-[10px] font-medium rounded bg-primary text-on-primary">总数</span>
              <span className="px-2 py-0.5 text-[10px] font-medium rounded bg-[#065f46] text-white">完成</span>
              <span className="px-2 py-0.5 text-[10px] font-medium rounded bg-surface-container-high text-on-surface-variant">错误</span>
            </div>
          </div>
          <ResponsiveContainer width="100%" height={220}>
            <LineChart data={chartData}>
              <XAxis dataKey="date" tick={{ fontSize: 11 }} stroke="#bec9c2" axisLine={false} tickLine={false} />
              <YAxis tick={{ fontSize: 11 }} stroke="#bec9c2" axisLine={false} tickLine={false} />
              <Tooltip
                contentStyle={{
                  background: '#ffffff',
                  border: 'none',
                  borderRadius: '8px',
                  boxShadow: '0 4px 20px rgba(0,69,50,0.08)',
                  fontSize: '12px',
                }}
              />
              <Line type="monotone" dataKey="total" name="总数" stroke="#004532" strokeWidth={2} dot={false} />
              <Line type="monotone" dataKey="finished" name="完成" stroke="#065f46" strokeWidth={2} dot={false} strokeDasharray="5 3" />
              <Line type="monotone" dataKey="error" name="错误" stroke="#e0e3e5" strokeWidth={2} dot={false} strokeDasharray="3 3" />
            </LineChart>
          </ResponsiveContainer>
          {/* Daily breakdown below chart */}
          <div className="grid grid-cols-4 gap-3 mt-4 pt-4">
            {dailyStats.slice(-4).map((d) => (
              <div key={d.date} className="text-center">
                <p className="text-xs text-on-surface-variant">{d.date.slice(5)}</p>
                <p className="text-sm font-semibold text-on-surface mt-0.5">{d.total} 个任务</p>
              </div>
            ))}
          </div>
        </div>

        {/* Task Status Distribution (Pie) */}
        <div className="col-span-4 bg-surface-container-lowest rounded-xl p-6 tonal-depth">
          <h3 className="text-sm font-semibold text-on-surface mb-4">任务状态分布</h3>
          {pieData.length > 0 ? (
            <>
              <ResponsiveContainer width="100%" height={200}>
                <PieChart>
                  <Pie
                    data={pieData}
                    cx="50%"
                    cy="50%"
                    innerRadius={55}
                    outerRadius={80}
                    dataKey="value"
                    strokeWidth={0}
                  >
                    {pieData.map((entry) => (
                      <Cell key={entry.name} fill={entry.color} />
                    ))}
                  </Pie>
                  <Tooltip
                    contentStyle={{
                      background: '#ffffff',
                      border: 'none',
                      borderRadius: '8px',
                      boxShadow: '0 4px 20px rgba(0,69,50,0.08)',
                      fontSize: '12px',
                    }}
                  />
                </PieChart>
              </ResponsiveContainer>
              <div className="space-y-2 mt-2">
                {pieData.map((p) => (
                  <div key={p.name} className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <span className="w-2.5 h-2.5 rounded-full" style={{ backgroundColor: p.color }} />
                      <span className="text-xs text-on-surface-variant">{p.name}</span>
                    </div>
                    <span className="text-xs font-medium text-on-surface">
                      {p.value} ({pieTotal > 0 ? Math.round((p.value / pieTotal) * 100) : 0}%)
                    </span>
                  </div>
                ))}
              </div>
            </>
          ) : (
            <div className="flex items-center justify-center h-[200px]">
              <p className="text-xs text-on-surface-variant">暂无任务数据</p>
            </div>
          )}
        </div>

        {/* Error Tasks Summary */}
        <div className="col-span-6 bg-surface-container-lowest rounded-xl p-6 tonal-depth">
          <h3 className="text-sm font-semibold text-on-surface mb-4">任务异常概况</h3>
          <div className="space-y-3">
            {stats && stats.tasks.error > 0 ? (
              <div className="p-3 rounded-lg bg-surface-container-low border-l-4 border-tertiary">
                <div className="flex items-start gap-2">
                  <AlertTriangle className="w-4 h-4 text-tertiary mt-0.5 shrink-0" />
                  <div>
                    <p className="text-xs font-medium text-on-surface">
                      共 {stats.tasks.error} 个错误任务
                    </p>
                    <p className="text-[11px] text-on-surface-variant mt-0.5">
                      错误率 {stats.tasks.total > 0
                        ? ((stats.tasks.error / stats.tasks.total) * 100).toFixed(1)
                        : '0.0'}%，请前往任务列表查看详情
                    </p>
                  </div>
                </div>
              </div>
            ) : (
              <div className="p-3 rounded-lg bg-surface-container-low border-l-4 border-primary/20">
                <div className="flex items-start gap-2">
                  <HeartPulse className="w-4 h-4 text-primary mt-0.5 shrink-0" />
                  <div>
                    <p className="text-xs font-medium text-on-surface">系统运行正常</p>
                    <p className="text-[11px] text-on-surface-variant mt-0.5">暂无错误任务</p>
                  </div>
                </div>
              </div>
            )}
            {stats && stats.tasks.pending > 0 && (
              <div className="p-3 rounded-lg bg-surface-container-low border-l-4 border-tertiary-container">
                <div className="flex items-start gap-2">
                  <Loader2 className="w-4 h-4 text-tertiary-container mt-0.5 shrink-0" />
                  <div>
                    <p className="text-xs font-medium text-on-surface">
                      {stats.tasks.pending} 个任务等待执行
                    </p>
                    <p className="text-[11px] text-on-surface-variant mt-0.5">
                      当前运行中 {stats.tasks.running} 个
                    </p>
                  </div>
                </div>
              </div>
            )}
            {/* Daily error trend */}
            {dailyStats.some((d) => d.error > 0) && (
              <div className="p-3 rounded-lg bg-surface-container-low border-l-4 border-primary/20">
                <p className="text-xs font-medium text-on-surface mb-2">近7天错误趋势</p>
                <div className="flex items-end gap-1 h-8">
                  {dailyStats.map((d) => {
                    const maxErr = Math.max(...dailyStats.map((x) => x.error), 1);
                    const h = (d.error / maxErr) * 100;
                    return (
                      <div
                        key={d.date}
                        className="flex-1 rounded-sm bg-tertiary/60"
                        style={{ height: `${Math.max(h, 4)}%` }}
                        title={`${d.date}: ${d.error} 错误`}
                      />
                    );
                  })}
                </div>
              </div>
            )}
          </div>
        </div>

        {/* Node Load – Top nodes by CPU */}
        <div className="col-span-6 bg-surface-container-lowest rounded-xl p-6 tonal-depth">
          <h3 className="text-sm font-semibold text-on-surface mb-4">节点负载概况</h3>
          <div className="flex items-center gap-4 mb-6">
            <div className="flex items-center gap-1.5">
              <span className="w-2 h-2 rounded-full bg-tertiary" />
              <span className="text-[11px] text-on-surface-variant">高负载</span>
            </div>
            <div className="flex items-center gap-1.5">
              <span className="w-2 h-2 rounded-full bg-primary" />
              <span className="text-[11px] text-on-surface-variant">正常</span>
            </div>
            <div className="flex items-center gap-1.5">
              <span className="w-2 h-2 rounded-full bg-surface-container-highest" />
              <span className="text-[11px] text-on-surface-variant">空闲</span>
            </div>
          </div>
          {topNodes.length > 0 ? (
            <div className="space-y-4">
              {topNodes.map((node) => {
                const load = node.cpu_usage || 0;
                return (
                  <div key={node._id} className="space-y-1.5">
                    <div className="flex items-center justify-between">
                      <span className="text-xs font-medium text-on-surface">{node.name || node.key}</span>
                      <span className="text-xs text-on-surface-variant">{load.toFixed(1)}%</span>
                    </div>
                    <div className="h-2 rounded-full bg-surface-container-high overflow-hidden">
                      <div
                        className={`h-full rounded-full transition-all ${
                          load > 80 ? 'bg-tertiary' : load > 50 ? 'bg-primary' : 'bg-primary-fixed-dim'
                        }`}
                        style={{ width: `${load}%` }}
                      />
                    </div>
                  </div>
                );
              })}
            </div>
          ) : (
            <div className="text-center py-8">
              <Server className="w-8 h-8 text-outline mx-auto mb-2" />
              <p className="text-xs text-on-surface-variant">暂无节点数据</p>
            </div>
          )}
          <div className="mt-6 p-4 rounded-lg bg-surface-container-low">
            <p className="text-xs text-on-surface-variant">
              全网部署 <span className="text-on-surface font-medium">{stats?.nodes.total ?? 0}</span> 个节点，在线
              <span className="text-on-surface font-medium"> {stats?.nodes.online ?? 0}</span> 个
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
