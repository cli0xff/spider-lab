import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQuery, useMutation } from '@tanstack/react-query';
import { createTask, getSpiders, getNodes } from '../lib/api';
import type { Spider, Node } from '../types';
import {
  Globe, TrendingUp, ShoppingBag, Video, Link2, PlusCircle,
  ArrowLeft, ArrowRight, Check, Loader2, Bug,
} from 'lucide-react';

const steps = ['目标与平台', '采集规则', '节点分配', '计划与确认'];

const platforms = [
  { id: 'weibo', name: '微博 Weibo', icon: Globe },
  { id: 'xueqiu', name: '雪球 Xueqiu', icon: TrendingUp },
  { id: 'xhs', name: '小红书 XHS', icon: ShoppingBag },
  { id: 'douyin', name: '抖音 Douyin', icon: Video },
  { id: 'url', name: '通用网页 URL', icon: Link2 },
  { id: 'custom', name: '自定义接入', icon: PlusCircle },
];

const modeOptions = [
  { value: 'all-nodes', label: '全部节点', desc: '任务将分发到所有可用节点' },
  { value: 'selected-nodes', label: '指定节点', desc: '手动选择执行任务的节点' },
  { value: 'random', label: '随机分配', desc: '系统自动选择最优节点' },
];

const modeLabel: Record<string, string> = {
  'all-nodes': '全部节点',
  'selected-nodes': '指定节点',
  random: '随机分配',
};

export default function CreateTask() {
  const navigate = useNavigate();
  const [step, setStep] = useState(0);
  const [form, setForm] = useState({
    platform: '',
    spiderId: '',
    name: '',
    targets: '',
    frequency: '300',
    concurrency: '16',
    depth: '2',
    proxyRotation: '10',
    mode: 'all-nodes',
    nodeIds: [] as string[],
    cron: '',
    priority: '5',
  });
  const [toast, setToast] = useState<{ msg: string; type: 'success' | 'error' } | null>(null);

  const showToast = (msg: string, type: 'success' | 'error' = 'success') => {
    setToast({ msg, type });
    setTimeout(() => setToast(null), 3000);
  };

  // ── Queries ──────────────────────────────────────────────────────────
  const { data: spidersData, isLoading: spidersLoading } = useQuery({
    queryKey: ['spiders'],
    queryFn: () => getSpiders({ page: 1, size: 200 }),
  });
  const spiders: Spider[] = (spidersData as any)?.data ?? [];

  const { data: nodesData, isLoading: nodesLoading } = useQuery({
    queryKey: ['nodes'],
    queryFn: () => getNodes({ page: 1, size: 200 }),
  });
  const nodes: Node[] = (nodesData as any)?.data ?? [];

  // ── Mutation ─────────────────────────────────────────────────────────
  const createMut = useMutation({
    mutationFn: (data: Parameters<typeof createTask>[0]) => createTask(data),
    onSuccess: () => {
      showToast('任务创建成功');
      setTimeout(() => navigate('/tasks'), 600);
    },
    onError: (e: Error) => showToast(e.message || '任务创建失败', 'error'),
  });

  // ── Derived state ────────────────────────────────────────────────────
  // Try to match spiders by platform keyword; fall back to showing all
  const platformSpiders = form.platform
    ? spiders.filter(
        (s) =>
          s.name.toLowerCase().includes(form.platform) ||
          (s.project && s.project.toLowerCase().includes(form.platform)),
      )
    : [];
  const displayedSpiders = platformSpiders.length > 0 ? platformSpiders : spiders;

  const selectedSpider = spiders.find((s) => s._id === form.spiderId);

  // ── Handlers ─────────────────────────────────────────────────────────
  const handleSubmit = () => {
    if (!form.spiderId) return showToast('请选择一个爬虫', 'error');
    if (form.mode === 'selected-nodes' && form.nodeIds.length === 0) {
      return showToast('请选择至少一个节点', 'error');
    }
    createMut.mutate({
      spider_id: form.spiderId,
      node_ids: form.mode === 'selected-nodes' ? form.nodeIds : undefined,
      param: form.targets || undefined,
      mode: form.mode,
      priority: Number(form.priority),
    });
  };

  // ── Render ───────────────────────────────────────────────────────────
  return (
    <div className="max-w-5xl mx-auto space-y-8">
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
          <h1 className="text-xl font-bold text-on-surface">新建采集任务</h1>
          <p className="text-sm text-on-surface-variant mt-1">
            配置高精度分布式数据采集流，确保数据的一致性与时效性。
          </p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={() => navigate(-1)}
            className="px-3 py-2 text-xs text-on-surface-variant bg-surface-container-high rounded-lg hover:bg-surface-container-highest transition-colors"
          >
            取消
          </button>
          <button className="px-3 py-2 text-xs text-on-surface-variant bg-surface-container-high rounded-lg hover:bg-surface-container-highest transition-colors">
            保存草稿
          </button>
        </div>
      </div>

      {/* Stepper */}
      <div className="flex items-center gap-0">
        {steps.map((s, i) => (
          <div key={s} className="flex-1 flex items-center">
            <div className="flex items-center gap-2">
              <div
                className={`w-8 h-8 rounded-full flex items-center justify-center text-xs font-semibold ${
                  i <= step
                    ? 'bg-primary text-on-primary'
                    : 'bg-surface-container-high text-on-surface-variant'
                }`}
              >
                {i < step ? <Check className="w-4 h-4" /> : i + 1}
              </div>
              <span
                className={`text-xs ${
                  i <= step ? 'text-on-surface font-medium' : 'text-on-surface-variant'
                }`}
              >
                {s}
              </span>
            </div>
            {i < steps.length - 1 && (
              <div className="flex-1 h-0.5 mx-3 rounded bg-surface-container-high overflow-hidden">
                <div
                  className={`h-full bg-primary transition-all ${i < step ? 'w-full' : 'w-0'}`}
                />
              </div>
            )}
          </div>
        ))}
      </div>

      {/* Step Content */}
      <div className="bg-surface-container-lowest rounded-xl p-8 tonal-depth">
        {/* ──────── Step 0: Platform & Spider ──────── */}
        {step === 0 && (
          <div className="space-y-6">
            {/* Platform selection (UI concept, unchanged) */}
            <div>
              <h3 className="text-sm font-semibold text-on-surface mb-4">选择数据源平台</h3>
              <div className="grid grid-cols-3 gap-3">
                {platforms.map((p) => (
                  <button
                    key={p.id}
                    onClick={() => setForm({ ...form, platform: p.id, spiderId: '' })}
                    className={`flex items-center gap-3 p-4 rounded-xl transition-all ${
                      form.platform === p.id
                        ? 'bg-primary-fixed/20 tonal-depth ring-2 ring-primary/30'
                        : 'bg-surface-container-low hover:bg-surface-container'
                    }`}
                  >
                    <p.icon
                      className={`w-5 h-5 ${
                        form.platform === p.id ? 'text-primary' : 'text-on-surface-variant'
                      }`}
                    />
                    <span
                      className={`text-sm ${
                        form.platform === p.id ? 'text-primary font-medium' : 'text-on-surface'
                      }`}
                    >
                      {p.name}
                    </span>
                  </button>
                ))}
              </div>
            </div>

            {/* Spider selection (fetched from API) */}
            <div>
              <h3 className="text-sm font-semibold text-on-surface mb-4">
                选择爬虫 <span className="text-error">*</span>
              </h3>
              {spidersLoading ? (
                <div className="flex items-center gap-2 text-xs text-on-surface-variant py-4">
                  <Loader2 className="w-4 h-4 animate-spin" />
                  加载爬虫列表...
                </div>
              ) : displayedSpiders.length === 0 ? (
                <p className="text-xs text-on-surface-variant py-2">暂无可用爬虫</p>
              ) : (
                <>
                  {form.platform && platformSpiders.length === 0 && spiders.length > 0 && (
                    <p className="text-[11px] text-on-surface-variant mb-2">
                      没有匹配当前平台的爬虫，显示全部：
                    </p>
                  )}
                  <div className="space-y-2 max-h-52 overflow-y-auto">
                    {displayedSpiders.map((spider) => (
                      <button
                        key={spider._id}
                        onClick={() => setForm({ ...form, spiderId: spider._id })}
                        className={`w-full flex items-center gap-3 p-3 rounded-xl text-left transition-all ${
                          form.spiderId === spider._id
                            ? 'bg-primary-fixed/20 tonal-depth ring-2 ring-primary/30'
                            : 'bg-surface-container-low hover:bg-surface-container'
                        }`}
                      >
                        <div className="w-8 h-8 rounded-lg bg-primary-fixed/20 flex items-center justify-center shrink-0">
                          <Bug className="w-4 h-4 text-primary" />
                        </div>
                        <div className="min-w-0 flex-1">
                          <p
                            className={`text-sm truncate ${
                              form.spiderId === spider._id
                                ? 'text-primary font-medium'
                                : 'text-on-surface'
                            }`}
                          >
                            {spider.name}
                          </p>
                          <p className="text-[11px] text-on-surface-variant font-mono truncate">
                            {spider.cmd}
                          </p>
                        </div>
                        {spider.project && (
                          <span className="text-[10px] px-1.5 py-0.5 rounded bg-surface-container-high text-on-surface-variant shrink-0">
                            {spider.project}
                          </span>
                        )}
                      </button>
                    ))}
                  </div>
                </>
              )}
            </div>

            {/* Target definition */}
            <div className="space-y-4">
              <h3 className="text-sm font-semibold text-on-surface">定义采集目标</h3>
              <div className="space-y-1.5">
                <label className="text-xs font-medium text-on-surface-variant">采集对象名称</label>
                <input
                  type="text"
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                  placeholder="例如: 行业趋势分析_2024Q4"
                  className="w-full px-4 py-3 bg-surface-container-highest rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary/30 transition-all placeholder:text-outline-variant"
                />
                <p className="text-[11px] text-outline">用于在系统中唯一标识该采集任务</p>
              </div>
              <div className="space-y-1.5">
                <label className="text-xs font-medium text-on-surface-variant">
                  目标 URL 或关键词
                </label>
                <textarea
                  value={form.targets}
                  onChange={(e) => setForm({ ...form, targets: e.target.value })}
                  placeholder="请输入完整的目标 URL 或以逗号分隔的搜索关键词..."
                  rows={4}
                  className="w-full px-4 py-3 bg-surface-container-highest rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary/30 transition-all resize-none placeholder:text-outline-variant"
                />
                <p className="text-[11px] text-outline">支持批量粘贴，每行一个地址</p>
              </div>
            </div>
          </div>
        )}

        {/* ──────── Step 1: Rules (unchanged) ──────── */}
        {step === 1 && (
          <div className="space-y-6">
            <h3 className="text-sm font-semibold text-on-surface mb-4">配置采集规则</h3>
            <div className="grid grid-cols-2 gap-6">
              <div className="space-y-1.5">
                <label className="text-xs font-medium text-on-surface-variant">
                  采集频率 (秒)
                </label>
                <input
                  type="number"
                  value={form.frequency}
                  onChange={(e) => setForm({ ...form, frequency: e.target.value })}
                  className="w-full px-4 py-3 bg-surface-container-highest rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary/30 transition-all"
                />
              </div>
              <div className="space-y-1.5">
                <label className="text-xs font-medium text-on-surface-variant">并发任务数</label>
                <input
                  type="number"
                  value={form.concurrency}
                  onChange={(e) => setForm({ ...form, concurrency: e.target.value })}
                  className="w-full px-4 py-3 bg-surface-container-highest rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary/30 transition-all"
                />
              </div>
              <div className="space-y-1.5">
                <label className="text-xs font-medium text-on-surface-variant">采集深度</label>
                <input
                  type="number"
                  value={form.depth}
                  onChange={(e) => setForm({ ...form, depth: e.target.value })}
                  className="w-full px-4 py-3 bg-surface-container-highest rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary/30 transition-all"
                />
              </div>
              <div className="space-y-1.5">
                <label className="text-xs font-medium text-on-surface-variant">
                  代理切换间隔 (分钟)
                </label>
                <input
                  type="number"
                  value={form.proxyRotation}
                  onChange={(e) => setForm({ ...form, proxyRotation: e.target.value })}
                  className="w-full px-4 py-3 bg-surface-container-highest rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary/30 transition-all"
                />
              </div>
            </div>
            <div className="p-4 rounded-lg bg-primary-fixed/10">
              <p className="text-xs text-primary">
                采集频率过高可能导致代理池快速消耗或触发目标站点的 WAF
                拦截。建议根据目标站点的防护等级合理配置。
              </p>
            </div>
          </div>
        )}

        {/* ──────── Step 2: Node assignment ──────── */}
        {step === 2 && (
          <div className="space-y-6">
            <h3 className="text-sm font-semibold text-on-surface mb-4">节点分配策略</h3>
            <div className="space-y-3">
              {modeOptions.map((opt) => (
                <button
                  key={opt.value}
                  onClick={() =>
                    setForm({
                      ...form,
                      mode: opt.value,
                      nodeIds: opt.value !== 'selected-nodes' ? [] : form.nodeIds,
                    })
                  }
                  className={`w-full flex items-start gap-3 p-4 rounded-xl text-left transition-all ${
                    form.mode === opt.value
                      ? 'bg-primary-fixed/20 tonal-depth'
                      : 'bg-surface-container-low hover:bg-surface-container'
                  }`}
                >
                  <div
                    className={`w-5 h-5 rounded-full flex items-center justify-center mt-0.5 ${
                      form.mode === opt.value ? 'bg-primary' : 'bg-surface-container-highest'
                    }`}
                  >
                    {form.mode === opt.value && <Check className="w-3 h-3 text-on-primary" />}
                  </div>
                  <div>
                    <p
                      className={`text-sm ${
                        form.mode === opt.value ? 'text-primary font-medium' : 'text-on-surface'
                      }`}
                    >
                      {opt.label}
                    </p>
                    <p className="text-xs text-on-surface-variant mt-0.5">{opt.desc}</p>
                  </div>
                </button>
              ))}
            </div>

            {/* Node checkboxes (visible when "指定节点" is selected) */}
            {form.mode === 'selected-nodes' && (
              <div className="space-y-2 pt-2">
                <h4 className="text-xs font-semibold text-on-surface">
                  选择节点 <span className="text-error">*</span>
                </h4>
                {nodesLoading ? (
                  <div className="flex items-center gap-2 text-xs text-on-surface-variant py-4">
                    <Loader2 className="w-4 h-4 animate-spin" />
                    加载节点列表...
                  </div>
                ) : nodes.length === 0 ? (
                  <p className="text-xs text-on-surface-variant py-2">暂无可用节点</p>
                ) : (
                  <div className="space-y-1.5 max-h-56 overflow-y-auto rounded-lg bg-surface-container-highest p-2">
                    {nodes.map((node) => {
                      const checked = form.nodeIds.includes(node._id);
                      return (
                        <label
                          key={node._id}
                          className={`flex items-center gap-2.5 px-3 py-2 rounded-lg cursor-pointer transition-colors ${
                            checked ? 'bg-primary-fixed/20' : 'hover:bg-surface-container-low'
                          }`}
                        >
                          <input
                            type="checkbox"
                            checked={checked}
                            onChange={() => {
                              setForm({
                                ...form,
                                nodeIds: checked
                                  ? form.nodeIds.filter((id) => id !== node._id)
                                  : [...form.nodeIds, node._id],
                              });
                            }}
                            className="accent-primary w-3.5 h-3.5"
                          />
                          <div className="flex-1 min-w-0">
                            <span className="text-xs font-medium text-on-surface">
                              {node.name || node.key}
                            </span>
                            <span
                              className={`ml-2 text-[10px] px-1.5 py-0.5 rounded ${
                                node.status === 'online'
                                  ? 'bg-primary-fixed/15 text-primary'
                                  : 'bg-surface-container-high text-on-surface-variant'
                              }`}
                            >
                              {node.status === 'online' ? '在线' : '离线'}
                            </span>
                          </div>
                          <span className="text-[10px] text-on-surface-variant">{node.ip}</span>
                        </label>
                      );
                    })}
                  </div>
                )}
                {form.nodeIds.length > 0 && (
                  <p className="text-[11px] text-on-surface-variant">
                    已选择 {form.nodeIds.length} 个节点
                  </p>
                )}
              </div>
            )}
          </div>
        )}

        {/* ──────── Step 3: Confirm ──────── */}
        {step === 3 && (
          <div className="space-y-6">
            <h3 className="text-sm font-semibold text-on-surface mb-4">计划与确认</h3>
            <div className="grid grid-cols-2 gap-6">
              <div className="space-y-1.5">
                <label className="text-xs font-medium text-on-surface-variant">CRON 表达式</label>
                <input
                  type="text"
                  value={form.cron}
                  onChange={(e) => setForm({ ...form, cron: e.target.value })}
                  placeholder="0 0/5 * * * ?"
                  className="w-full px-4 py-3 bg-surface-container-highest rounded-lg text-sm font-mono outline-none focus:ring-2 focus:ring-primary/30 transition-all placeholder:text-outline-variant"
                />
              </div>
              <div className="space-y-1.5">
                <label className="text-xs font-medium text-on-surface-variant">
                  优先级 (1-10)
                </label>
                <input
                  type="number"
                  min={1}
                  max={10}
                  value={form.priority}
                  onChange={(e) => setForm({ ...form, priority: e.target.value })}
                  className="w-full px-4 py-3 bg-surface-container-highest rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary/30 transition-all"
                />
              </div>
            </div>

            <div className="p-5 rounded-xl bg-surface-container-low space-y-3">
              <h4 className="text-xs font-semibold text-on-surface">任务预览</h4>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <span className="text-[11px] text-on-surface-variant">平台：</span>
                  <span className="text-xs text-on-surface">{form.platform || '未选择'}</span>
                </div>
                <div>
                  <span className="text-[11px] text-on-surface-variant">爬虫：</span>
                  <span className="text-xs text-on-surface">
                    {selectedSpider?.name || '未选择'}
                  </span>
                </div>
                <div>
                  <span className="text-[11px] text-on-surface-variant">名称：</span>
                  <span className="text-xs text-on-surface">{form.name || '未填写'}</span>
                </div>
                <div>
                  <span className="text-[11px] text-on-surface-variant">模式：</span>
                  <span className="text-xs text-on-surface">
                    {modeLabel[form.mode] || form.mode}
                  </span>
                </div>
                {form.mode === 'selected-nodes' && (
                  <div>
                    <span className="text-[11px] text-on-surface-variant">节点数：</span>
                    <span className="text-xs text-on-surface">{form.nodeIds.length}</span>
                  </div>
                )}
                <div>
                  <span className="text-[11px] text-on-surface-variant">优先级：</span>
                  <span className="text-xs text-on-surface">{form.priority}</span>
                </div>
                <div>
                  <span className="text-[11px] text-on-surface-variant">CRON：</span>
                  <span className="text-xs text-on-surface font-mono">
                    {form.cron || '未设置'}
                  </span>
                </div>
              </div>
            </div>
          </div>
        )}
      </div>

      {/* Navigation */}
      <div className="flex items-center justify-between">
        <button
          onClick={() => step > 0 && setStep(step - 1)}
          disabled={step === 0}
          className="flex items-center gap-1.5 px-4 py-2 text-xs text-on-surface-variant bg-surface-container-high rounded-lg hover:bg-surface-container-highest transition-colors disabled:opacity-40"
        >
          <ArrowLeft className="w-4 h-4" />
          上一步
        </button>
        {step < steps.length - 1 ? (
          <button
            onClick={() => setStep(step + 1)}
            className="flex items-center gap-1.5 px-4 py-2 text-xs font-medium text-on-primary bg-gradient-to-br from-primary to-primary-container rounded-lg hover:opacity-90 active:scale-95 transition-all tonal-depth"
          >
            继续下一步
            <ArrowRight className="w-4 h-4" />
          </button>
        ) : (
          <button
            onClick={handleSubmit}
            disabled={createMut.isPending}
            className="flex items-center gap-1.5 px-6 py-2 text-xs font-medium text-on-primary bg-gradient-to-br from-primary to-primary-container rounded-lg hover:opacity-90 active:scale-95 transition-all tonal-depth disabled:opacity-60"
          >
            {createMut.isPending ? (
              <Loader2 className="w-4 h-4 animate-spin" />
            ) : (
              <Check className="w-4 h-4" />
            )}
            {createMut.isPending ? '创建中...' : '确认并创建任务'}
          </button>
        )}
      </div>
    </div>
  );
}
