import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  getSchedules, createSchedule, updateSchedule, deleteSchedule,
  enableSchedule, disableSchedule, getSpiders,
} from '../lib/api';
import type { Schedule, Spider } from '../types';
import {
  Clock, Plus, Trash2, Settings, X, Loader2,
  AlertTriangle, Search,
} from 'lucide-react';

const modeLabel: Record<string, string> = {
  'all-nodes': '全部节点',
  'selected-nodes': '指定节点',
  'random': '随机分配',
};

interface ScheduleForm {
  name: string;
  spider_id: string;
  cron: string;
  cmd: string;
  param: string;
  mode: string;
  description: string;
}

const emptyForm: ScheduleForm = {
  name: '', spider_id: '', cron: '', cmd: '',
  param: '', mode: 'all-nodes', description: '',
};

export default function Schedules() {
  const queryClient = useQueryClient();
  const [search, setSearch] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const [editingSchedule, setEditingSchedule] = useState<Schedule | null>(null);
  const [deletingSchedule, setDeletingSchedule] = useState<Schedule | null>(null);
  const [form, setForm] = useState<ScheduleForm>(emptyForm);
  const [toast, setToast] = useState<{ msg: string; type: 'success' | 'error' } | null>(null);

  const showToast = (msg: string, type: 'success' | 'error' = 'success') => {
    setToast({ msg, type });
    setTimeout(() => setToast(null), 3000);
  };

  // Queries
  const { data, isLoading, error } = useQuery({
    queryKey: ['schedules'],
    queryFn: () => getSchedules({ page: 1, size: 200 }),
  });

  const allSchedules: Schedule[] = data?.data ?? [];
  const schedules = search
    ? allSchedules.filter((s) =>
        s.name.toLowerCase().includes(search.toLowerCase()) ||
        (s.spider_name || '').toLowerCase().includes(search.toLowerCase()) ||
        s.cron.includes(search))
    : allSchedules;

  const enabledCount = allSchedules.filter((s) => s.enabled).length;

  // Fetch spiders for dropdown
  const { data: spidersData } = useQuery({
    queryKey: ['spiders'],
    queryFn: () => getSpiders({ page: 1, size: 200 }),
  });
  const spiders: Spider[] = spidersData?.data ?? [];

  // Mutations
  const createMut = useMutation({
    mutationFn: (data: Partial<Schedule>) => createSchedule(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['schedules'] });
      setShowCreate(false);
      setForm(emptyForm);
      showToast('定时任务创建成功');
    },
    onError: (e: Error) => showToast(e.message || '创建失败', 'error'),
  });

  const updateMut = useMutation({
    mutationFn: ({ id, data }: { id: string; data: Partial<Schedule> }) => updateSchedule(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['schedules'] });
      setEditingSchedule(null);
      setForm(emptyForm);
      showToast('定时任务已更新');
    },
    onError: (e: Error) => showToast(e.message || '更新失败', 'error'),
  });

  const deleteMut = useMutation({
    mutationFn: (id: string) => deleteSchedule(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['schedules'] });
      setDeletingSchedule(null);
      showToast('定时任务已删除');
    },
    onError: (e: Error) => showToast(e.message || '删除失败', 'error'),
  });

  const enableMut = useMutation({
    mutationFn: (id: string) => enableSchedule(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['schedules'] });
      showToast('定时任务已启用');
    },
    onError: (e: Error) => showToast(e.message || '操作失败', 'error'),
  });

  const disableMut = useMutation({
    mutationFn: (id: string) => disableSchedule(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['schedules'] });
      showToast('定时任务已禁用');
    },
    onError: (e: Error) => showToast(e.message || '操作失败', 'error'),
  });

  const handleToggle = (sch: Schedule) => {
    if (sch.enabled) {
      disableMut.mutate(sch.id);
    } else {
      enableMut.mutate(sch.id);
    }
  };

  const handleCreate = () => {
    if (!form.name.trim()) return showToast('请输入任务名称', 'error');
    if (!form.spider_id) return showToast('请选择爬虫', 'error');
    if (!form.cron.trim()) return showToast('请输入 Cron 表达式', 'error');
    createMut.mutate(form as unknown as Partial<Schedule>);
  };

  const handleUpdate = () => {
    if (!editingSchedule) return;
    updateMut.mutate({ id: editingSchedule.id, data: form as unknown as Partial<Schedule> });
  };

  const openEdit = (sch: Schedule) => {
    setForm({
      name: sch.name,
      spider_id: sch.spider_id,
      cron: sch.cron,
      cmd: sch.cmd || '',
      param: sch.param || '',
      mode: sch.mode || 'all-nodes',
      description: sch.description || '',
    });
    setEditingSchedule(sch);
  };

  const isModalOpen = showCreate || !!editingSchedule;
  const isMutating = createMut.isPending || updateMut.isPending;

  return (
    <div className="space-y-6">
      {/* Toast */}
      {toast && (
        <div className={`fixed top-20 right-6 z-50 px-4 py-3 rounded-lg text-sm font-medium tonal-depth transition-all ${
          toast.type === 'success' ? 'bg-primary-fixed text-primary' : 'bg-error-container text-on-error-container'
        }`}>
          {toast.msg}
        </div>
      )}

      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-on-surface">定时任务</h1>
          <p className="text-sm text-on-surface-variant mt-1">
            管理采集任务的自动调度计划
            {allSchedules.length > 0 && (
              <span className="ml-2 text-outline">
                ({enabledCount} 个启用 / {allSchedules.length} 个总计)
              </span>
            )}
          </p>
        </div>
        <button
          onClick={() => { setForm(emptyForm); setShowCreate(true); }}
          className="flex items-center gap-1.5 px-4 py-2 text-xs font-medium text-on-primary bg-gradient-to-br from-primary to-primary-container rounded-lg hover:opacity-90 active:scale-95 transition-all tonal-depth"
        >
          <Plus className="w-4 h-4" />
          新建定时任务
        </button>
      </div>

      {/* Search */}
      <div className="flex items-center gap-3">
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-outline" />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="搜索任务名称 / 爬虫 / Cron..."
            className="w-full pl-9 pr-4 py-2 text-xs bg-surface-container-highest rounded-lg outline-none focus:ring-2 focus:ring-primary/30 transition-all placeholder:text-outline-variant"
          />
          {search && (
            <button onClick={() => setSearch('')} className="absolute right-3 top-1/2 -translate-y-1/2">
              <X className="w-3.5 h-3.5 text-outline hover:text-on-surface" />
            </button>
          )}
        </div>
      </div>

      {/* Loading */}
      {isLoading && (
        <div className="bg-surface-container-lowest rounded-xl p-12 tonal-depth text-center">
          <Loader2 className="w-8 h-8 text-primary animate-spin mx-auto" />
          <p className="text-sm text-on-surface-variant mt-3">加载定时任务...</p>
        </div>
      )}

      {/* Error */}
      {error && !isLoading && (
        <div className="bg-surface-container-lowest rounded-xl p-8 tonal-depth text-center">
          <AlertTriangle className="w-10 h-10 text-tertiary mx-auto mb-3" />
          <p className="text-sm font-medium text-on-surface">加载失败</p>
          <p className="text-xs text-on-surface-variant mt-1">{(error as Error).message}</p>
          <button
            onClick={() => queryClient.invalidateQueries({ queryKey: ['schedules'] })}
            className="mt-4 px-4 py-2 text-xs font-medium text-primary bg-primary-fixed/20 rounded-lg hover:bg-primary-fixed/30 transition-colors"
          >
            重试
          </button>
        </div>
      )}

      {/* Empty */}
      {!isLoading && !error && schedules.length === 0 && (
        <div className="bg-surface-container-lowest rounded-xl p-12 tonal-depth text-center">
          <div className="w-16 h-16 rounded-2xl bg-primary-fixed/20 flex items-center justify-center mx-auto mb-4">
            <Clock className="w-8 h-8 text-primary" />
          </div>
          <p className="text-sm font-semibold text-on-surface">
            {search ? '没有找到匹配的定时任务' : '暂无定时任务'}
          </p>
          <p className="text-xs text-on-surface-variant mt-1.5">
            {search ? '请尝试调整搜索条件' : '点击「新建定时任务」创建您的第一个自动调度计划'}
          </p>
          {!search && (
            <button
              onClick={() => { setForm(emptyForm); setShowCreate(true); }}
              className="mt-4 px-4 py-2 text-xs font-medium text-on-primary bg-gradient-to-br from-primary to-primary-container rounded-lg hover:opacity-90 transition-all"
            >
              新建定时任务
            </button>
          )}
        </div>
      )}

      {/* Schedule List */}
      {!isLoading && !error && schedules.length > 0 && (
        <div className="space-y-3">
          {schedules.map((sch) => (
            <div key={sch.id} className="bg-surface-container-lowest rounded-xl p-5 tonal-depth flex items-center justify-between">
              <div className="flex items-center gap-4">
                <div className={`w-10 h-10 rounded-lg flex items-center justify-center ${sch.enabled ? 'bg-primary-fixed/20' : 'bg-surface-container-high'}`}>
                  <Clock className={`w-5 h-5 ${sch.enabled ? 'text-primary' : 'text-outline'}`} />
                </div>
                <div>
                  <h3 className="text-sm font-semibold text-on-surface">{sch.name}</h3>
                  <p className="text-[11px] text-on-surface-variant mt-0.5">
                    爬虫: {sch.spider_name || sch.spider_id}
                    {sch.description && <span className="ml-2 text-outline">· {sch.description}</span>}
                  </p>
                </div>
              </div>

              <div className="flex items-center gap-8">
                <div className="text-right">
                  <p className="text-xs font-mono text-on-surface">{sch.cron}</p>
                  <p className="text-[11px] text-on-surface-variant mt-0.5">
                    {modeLabel[sch.mode] || sch.mode}
                    <span className="ml-2 text-outline">
                      创建于 {new Date(sch.created_at).toLocaleDateString('zh-CN')}
                    </span>
                  </p>
                </div>

                <div className="flex items-center gap-4">
                  <label className="relative inline-flex items-center cursor-pointer">
                    <input
                      type="checkbox"
                      checked={sch.enabled}
                      onChange={() => handleToggle(sch)}
                      disabled={enableMut.isPending || disableMut.isPending}
                      className="sr-only peer"
                    />
                    <div className="w-9 h-5 bg-surface-container-highest rounded-full peer peer-checked:bg-primary peer-checked:after:translate-x-full after:content-[''] after:absolute after:top-0.5 after:left-0.5 after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all" />
                  </label>
                  <button
                    onClick={() => openEdit(sch)}
                    className="p-1.5 rounded-lg hover:bg-surface-container-high transition-colors"
                    title="编辑"
                  >
                    <Settings className="w-4 h-4 text-on-surface-variant" />
                  </button>
                  <button
                    onClick={() => setDeletingSchedule(sch)}
                    className="p-1.5 rounded-lg hover:bg-error-container/30 transition-colors group/del"
                    title="删除"
                  >
                    <Trash2 className="w-4 h-4 text-on-surface-variant group-hover/del:text-error" />
                  </button>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* ===== Create / Edit Modal ===== */}
      {isModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-on-surface/30" onClick={() => { setShowCreate(false); setEditingSchedule(null); }} />
          <div className="relative bg-surface-container-lowest rounded-2xl shadow-editorial w-full max-w-lg mx-4 max-h-[85vh] overflow-y-auto">
            <div className="p-6">
              <div className="flex items-center justify-between mb-6">
                <h2 className="text-base font-bold text-on-surface">
                  {editingSchedule ? '编辑定时任务' : '新建定时任务'}
                </h2>
                <button
                  onClick={() => { setShowCreate(false); setEditingSchedule(null); }}
                  className="p-1.5 rounded-lg hover:bg-surface-container-high transition-colors"
                >
                  <X className="w-4 h-4 text-on-surface-variant" />
                </button>
              </div>

              <div className="space-y-4">
                {/* Name */}
                <div className="space-y-1.5">
                  <label className="text-xs font-medium text-on-surface-variant">
                    任务名称 <span className="text-error">*</span>
                  </label>
                  <div className="relative">
                    <Clock className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-outline" />
                    <input
                      type="text"
                      value={form.name}
                      onChange={(e) => setForm({ ...form, name: e.target.value })}
                      placeholder="例如: 微博热点定时采集"
                      className="w-full pl-10 pr-4 py-3 bg-surface-container-highest rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary/30 transition-all placeholder:text-outline-variant"
                    />
                  </div>
                </div>

                {/* Spider */}
                <div className="space-y-1.5">
                  <label className="text-xs font-medium text-on-surface-variant">
                    选择爬虫 <span className="text-error">*</span>
                  </label>
                  <select
                    value={form.spider_id}
                    onChange={(e) => setForm({ ...form, spider_id: e.target.value })}
                    className="w-full px-3 py-3 bg-surface-container-highest rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary/30 transition-all"
                  >
                    <option value="">请选择爬虫...</option>
                    {spiders.map((sp) => (
                      <option key={sp._id} value={sp._id}>{sp.name}</option>
                    ))}
                  </select>
                </div>

                {/* Cron */}
                <div className="space-y-1.5">
                  <label className="text-xs font-medium text-on-surface-variant">
                    Cron 表达式 <span className="text-error">*</span>
                  </label>
                  <input
                    type="text"
                    value={form.cron}
                    onChange={(e) => setForm({ ...form, cron: e.target.value })}
                    placeholder="例如: 0 */5 * * * (每5分钟)"
                    className="w-full px-4 py-3 bg-surface-container-highest rounded-lg text-sm font-mono outline-none focus:ring-2 focus:ring-primary/30 transition-all placeholder:text-outline-variant"
                  />
                  <p className="text-[11px] text-outline">
                    支持标准 Cron 格式: 分 时 日 月 周
                  </p>
                </div>

                {/* Mode */}
                <div className="space-y-1.5">
                  <label className="text-xs font-medium text-on-surface-variant">运行模式</label>
                  <select
                    value={form.mode}
                    onChange={(e) => setForm({ ...form, mode: e.target.value })}
                    className="w-full px-3 py-3 bg-surface-container-highest rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary/30 transition-all"
                  >
                    <option value="all-nodes">全部节点</option>
                    <option value="selected-nodes">指定节点</option>
                    <option value="random">随机分配</option>
                  </select>
                </div>

                {/* Command (optional override) */}
                <div className="space-y-1.5">
                  <label className="text-xs font-medium text-on-surface-variant">执行命令 (可选)</label>
                  <input
                    type="text"
                    value={form.cmd}
                    onChange={(e) => setForm({ ...form, cmd: e.target.value })}
                    placeholder="留空则使用爬虫默认命令"
                    className="w-full px-4 py-3 bg-surface-container-highest rounded-lg text-sm font-mono outline-none focus:ring-2 focus:ring-primary/30 transition-all placeholder:text-outline-variant"
                  />
                </div>

                {/* Param */}
                <div className="space-y-1.5">
                  <label className="text-xs font-medium text-on-surface-variant">运行参数 (可选)</label>
                  <input
                    type="text"
                    value={form.param}
                    onChange={(e) => setForm({ ...form, param: e.target.value })}
                    placeholder="运行时传入的参数"
                    className="w-full px-4 py-3 bg-surface-container-highest rounded-lg text-sm font-mono outline-none focus:ring-2 focus:ring-primary/30 transition-all placeholder:text-outline-variant"
                  />
                </div>

                {/* Description */}
                <div className="space-y-1.5">
                  <label className="text-xs font-medium text-on-surface-variant">描述</label>
                  <textarea
                    value={form.description}
                    onChange={(e) => setForm({ ...form, description: e.target.value })}
                    placeholder="简要描述此定时任务的用途..."
                    rows={2}
                    className="w-full px-4 py-3 bg-surface-container-highest rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary/30 transition-all resize-none placeholder:text-outline-variant"
                  />
                </div>
              </div>

              {/* Actions */}
              <div className="flex items-center justify-end gap-2 mt-6 pt-4">
                <button
                  onClick={() => { setShowCreate(false); setEditingSchedule(null); }}
                  className="px-4 py-2 text-xs text-on-surface-variant bg-surface-container-high rounded-lg hover:bg-surface-container-highest transition-colors"
                >
                  取消
                </button>
                <button
                  onClick={editingSchedule ? handleUpdate : handleCreate}
                  disabled={isMutating}
                  className="flex items-center gap-1.5 px-5 py-2 text-xs font-medium text-on-primary bg-gradient-to-br from-primary to-primary-container rounded-lg hover:opacity-90 active:scale-95 transition-all disabled:opacity-60 tonal-depth"
                >
                  {isMutating && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
                  {editingSchedule ? '保存更改' : '创建任务'}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* ===== Delete Confirmation ===== */}
      {deletingSchedule && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-on-surface/30" onClick={() => setDeletingSchedule(null)} />
          <div className="relative bg-surface-container-lowest rounded-2xl shadow-editorial w-full max-w-sm mx-4 p-6">
            <div className="flex flex-col items-center text-center">
              <div className="w-12 h-12 rounded-full bg-error-container/30 flex items-center justify-center mb-4">
                <AlertTriangle className="w-6 h-6 text-error" />
              </div>
              <h3 className="text-sm font-bold text-on-surface">确认删除定时任务</h3>
              <p className="text-xs text-on-surface-variant mt-2 leading-relaxed">
                即将删除定时任务 <span className="font-medium text-on-surface">{deletingSchedule.name}</span>，
                此操作不可恢复。已执行的历史任务记录不会被删除。
              </p>
            </div>
            <div className="flex items-center gap-2 mt-6">
              <button
                onClick={() => setDeletingSchedule(null)}
                className="flex-1 px-4 py-2 text-xs text-on-surface-variant bg-surface-container-high rounded-lg hover:bg-surface-container-highest transition-colors"
              >
                取消
              </button>
              <button
                onClick={() => deleteMut.mutate(deletingSchedule.id)}
                disabled={deleteMut.isPending}
                className="flex-1 flex items-center justify-center gap-1.5 px-4 py-2 text-xs font-medium text-on-primary bg-error rounded-lg hover:opacity-90 transition-all disabled:opacity-60"
              >
                {deleteMut.isPending && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
                确认删除
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
