import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { getNodes, deleteNode, enableNode, disableNode, updateNode } from '../lib/api';
import type { Node } from '../types';
import {
  RefreshCw, Trash2, X, Loader2,
  AlertTriangle, Server, Search, Settings, Wifi, WifiOff,
  Monitor, HardDrive, Cpu, Database, Globe, Container,
} from 'lucide-react';

const statusMap: Record<string, { label: string; color: string }> = {
  online: { label: '在线', color: 'bg-primary' },
  offline: { label: '离线', color: 'bg-outline' },
};

const nodeIcons = [Server, Monitor, HardDrive, Cpu, Database, Globe, Container];
const nodeIconColors = [
  'bg-primary/10 text-primary',
  'bg-secondary-container/40 text-on-secondary-container',
  'bg-primary-fixed/30 text-on-primary-fixed-variant',
  'bg-tertiary-container/20 text-tertiary',
  'bg-surface-container-highest text-on-surface-variant',
  'bg-primary-fixed/20 text-primary',
  'bg-secondary-container/30 text-secondary',
];

function getNodeIcon(node: { _id: string; is_master: boolean }) {
  if (node.is_master) return { Icon: Server, color: 'bg-primary/15 text-primary' };
  const hash = node._id.split('').reduce((acc, c) => acc + c.charCodeAt(0), 0);
  const idx = hash % nodeIcons.length;
  return { Icon: nodeIcons[idx], color: nodeIconColors[idx] };
}

export default function Nodes() {
  const queryClient = useQueryClient();
  const [statusFilter, setStatusFilter] = useState('online');
  const [search, setSearch] = useState('');
  const [deletingNode, setDeletingNode] = useState<Node | null>(null);
  const [editingNode, setEditingNode] = useState<Node | null>(null);
  const [editForm, setEditForm] = useState({ name: '', description: '', max_runners: 8 });
  const [toast, setToast] = useState<{ msg: string; type: 'success' | 'error' } | null>(null);

  const showToast = (msg: string, type: 'success' | 'error' = 'success') => {
    setToast({ msg, type });
    setTimeout(() => setToast(null), 3000);
  };

  // Queries
  const { data, isLoading, error } = useQuery({
    queryKey: ['nodes', statusFilter],
    queryFn: () => getNodes({ status: statusFilter || undefined, page: 1, size: 200 }),
  });

  const allNodes: Node[] = (data as any)?.data ?? [];
  const nodes = search
    ? allNodes.filter((n) => n.name.toLowerCase().includes(search.toLowerCase()) || n.key.toLowerCase().includes(search.toLowerCase()) || n.ip.includes(search))
    : allNodes;

  const onlineCount = allNodes.filter((n) => n.status === 'online').length;
  const offlineCount = allNodes.filter((n) => n.status !== 'online').length;
  const avgCpu = allNodes.length > 0
    ? allNodes.reduce((acc, n) => acc + (n.cpu_usage || 0), 0) / allNodes.length
    : 0;

  // Mutations
  const deleteMut = useMutation({
    mutationFn: (id: string) => deleteNode(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['nodes'] });
      setDeletingNode(null);
      showToast('节点已移除');
    },
    onError: (e: Error) => showToast(e.message || '删除失败', 'error'),
  });

  const enableMut = useMutation({
    mutationFn: (id: string) => enableNode(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['nodes'] });
      showToast('节点已启用');
    },
    onError: (e: Error) => showToast(e.message || '操作失败', 'error'),
  });

  const disableMut = useMutation({
    mutationFn: (id: string) => disableNode(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['nodes'] });
      showToast('节点已禁用');
    },
    onError: (e: Error) => showToast(e.message || '操作失败', 'error'),
  });

  const updateMut = useMutation({
    mutationFn: ({ id, data }: { id: string; data: Record<string, any> }) => updateNode(id, data as any),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['nodes'] });
      setEditingNode(null);
      showToast('节点信息已更新');
    },
    onError: (e: Error) => showToast(e.message || '更新失败', 'error'),
  });

  const openEdit = (node: Node) => {
    setEditForm({ name: node.name || '', description: node.description || '', max_runners: node.max_runners || 8 });
    setEditingNode(node);
  };

  const handleUpdate = () => {
    if (!editingNode) return;
    updateMut.mutate({ id: editingNode._id, data: editForm });
  };

  const formatDuration = (dateStr: string) => {
    if (!dateStr || new Date(dateStr).getTime() <= 0) return null;
    const ms = Date.now() - new Date(dateStr).getTime();
    if (ms < 0 || isNaN(ms)) return null;
    if (ms < 60000) return '刚刚';
    const days = Math.floor(ms / 86400000);
    const hours = Math.floor((ms % 86400000) / 3600000);
    const mins = Math.floor((ms % 3600000) / 60000);
    if (days > 0) return `${days}d ${hours}h ${mins}m`;
    if (hours > 0) return `${hours}h ${mins}m`;
    return `${mins}m`;
  };

  const formatHeartbeat = (node: Node) => {
    const hb = formatDuration(node.last_heartbeat);
    if (hb) return hb + ' 前';
    const created = formatDuration(node.created_at);
    if (created) return created + ' 前 (注册)';
    return new Date(node.created_at).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' });
  };

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
          <h1 className="text-xl font-bold text-on-surface">节点运行状态</h1>
          <p className="text-sm text-on-surface-variant mt-1">
            当前全网共 {allNodes.length} 个采集节点
          </p>
        </div>
        <button
          onClick={() => queryClient.invalidateQueries({ queryKey: ['nodes'] })}
          className="flex items-center gap-1.5 px-3 py-2 text-xs text-on-surface-variant bg-surface-container-high rounded-lg hover:bg-surface-container-highest transition-colors"
        >
          <RefreshCw className="w-3.5 h-3.5" />
          刷新状态
        </button>
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-3 gap-4">
        <div className="bg-surface-container-lowest rounded-xl p-5 tonal-depth">
          <p className="text-xs text-on-surface-variant">平均 CPU 占用率</p>
          <p className="text-2xl font-bold text-on-surface mt-2">{avgCpu.toFixed(1)}%</p>
          <div className="mt-3 h-1.5 rounded-full bg-surface-container-high overflow-hidden">
            <div className={`h-full rounded-full ${avgCpu > 85 ? 'bg-tertiary' : 'bg-primary'}`} style={{ width: `${Math.min(avgCpu, 100)}%` }} />
          </div>
        </div>
        <div className="bg-surface-container-lowest rounded-xl p-5 tonal-depth">
          <p className="text-xs text-on-surface-variant">在线节点</p>
          <p className="text-2xl font-bold text-on-surface mt-2">
            {onlineCount}
            <span className="text-sm font-normal text-on-surface-variant"> / {allNodes.length}</span>
          </p>
        </div>
        <div className="bg-surface-container-lowest rounded-xl p-5 tonal-depth">
          <p className="text-xs text-on-surface-variant">离线节点</p>
          <p className="text-2xl font-bold text-tertiary mt-2">{offlineCount}</p>
        </div>
      </div>

      {/* Search & Filter */}
      <div className="flex items-center gap-3">
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-outline" />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="搜索名称 / Key / IP..."
            className="w-full pl-9 pr-4 py-2 text-xs bg-surface-container-highest rounded-lg outline-none focus:ring-2 focus:ring-primary/30 transition-all placeholder:text-outline-variant"
          />
          {search && (
            <button onClick={() => setSearch('')} className="absolute right-3 top-1/2 -translate-y-1/2">
              <X className="w-3.5 h-3.5 text-outline hover:text-on-surface" />
            </button>
          )}
        </div>
        <div className="flex gap-1">
          {[
            { value: '', label: '全部' },
            { value: 'online', label: '在线' },
            { value: 'offline', label: '离线' },
          ].map((f) => (
            <button
              key={f.value}
              onClick={() => setStatusFilter(f.value)}
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
      </div>

      {/* Loading */}
      {isLoading && (
        <div className="bg-surface-container-lowest rounded-xl p-12 tonal-depth text-center">
          <Loader2 className="w-8 h-8 text-primary animate-spin mx-auto" />
          <p className="text-sm text-on-surface-variant mt-3">加载节点数据...</p>
        </div>
      )}

      {/* Error */}
      {error && !isLoading && (
        <div className="bg-surface-container-lowest rounded-xl p-8 tonal-depth text-center">
          <AlertTriangle className="w-10 h-10 text-tertiary mx-auto mb-3" />
          <p className="text-sm font-medium text-on-surface">加载失败</p>
          <p className="text-xs text-on-surface-variant mt-1">{(error as Error).message}</p>
          <button
            onClick={() => queryClient.invalidateQueries({ queryKey: ['nodes'] })}
            className="mt-4 px-4 py-2 text-xs font-medium text-primary bg-primary-fixed/20 rounded-lg hover:bg-primary-fixed/30 transition-colors"
          >
            重试
          </button>
        </div>
      )}

      {/* Empty */}
      {!isLoading && !error && nodes.length === 0 && (
        <div className="bg-surface-container-lowest rounded-xl p-12 tonal-depth text-center">
          <div className="w-16 h-16 rounded-2xl bg-primary-fixed/20 flex items-center justify-center mx-auto mb-4">
            <Server className="w-8 h-8 text-primary" />
          </div>
          <p className="text-sm font-semibold text-on-surface">
            {search || statusFilter ? '没有找到匹配的节点' : '暂无节点'}
          </p>
          <p className="text-xs text-on-surface-variant mt-1.5">
            {search || statusFilter ? '请尝试调整搜索条件' : '启动 Worker 节点后会自动注册到此列表'}
          </p>
        </div>
      )}

      {/* Node Table */}
      {!isLoading && !error && nodes.length > 0 && (
        <div className="bg-surface-container-lowest rounded-xl tonal-depth overflow-hidden">
          <div className="px-6 py-4">
            <h3 className="text-sm font-semibold text-on-surface">实时节点列表</h3>
          </div>
          <table className="w-full">
            <thead>
              <tr className="bg-surface-container-low">
                <th className="text-left px-6 py-3 text-[10px] font-semibold text-on-surface-variant uppercase tracking-wider">节点</th>
                <th className="text-left px-6 py-3 text-[10px] font-semibold text-on-surface-variant uppercase tracking-wider">Key / IP</th>
                <th className="text-left px-6 py-3 text-[10px] font-semibold text-on-surface-variant uppercase tracking-wider">CPU / 内存</th>
                <th className="text-left px-6 py-3 text-[10px] font-semibold text-on-surface-variant uppercase tracking-wider">Runner</th>
                <th className="text-left px-6 py-3 text-[10px] font-semibold text-on-surface-variant uppercase tracking-wider">心跳</th>
                <th className="text-right px-6 py-3 text-[10px] font-semibold text-on-surface-variant uppercase tracking-wider">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-outline-variant/15">
              {nodes.map((node) => {
                const st = statusMap[node.status] ?? statusMap.offline;
                const { Icon: NodeIcon, color: iconColor } = getNodeIcon(node);
                return (
                  <tr key={node._id} className="hover:bg-surface-container-low transition-colors">
                    {/* Name & Status */}
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-3">
                        <div className={`w-8 h-8 rounded-lg flex items-center justify-center shrink-0 ${iconColor}`}>
                          <NodeIcon className="w-4 h-4" />
                        </div>
                        <div className="min-w-0">
                          <div className="flex items-center gap-2">
                            <span className={`w-2 h-2 rounded-full shrink-0 ${st.color}`} />
                            <p className="text-xs font-semibold text-on-surface truncate">{node.name || node.key}</p>
                          </div>
                          <p className="text-[10px] text-on-surface-variant mt-0.5 ml-4">
                            {st.label}
                            {node.is_master && <span className="ml-1.5 px-1.5 py-0.5 rounded bg-primary-fixed/15 text-primary text-[9px] font-medium">Master</span>}
                            {node.enabled === false && <span className="ml-1.5 px-1.5 py-0.5 rounded bg-error-container/30 text-on-error-container text-[9px] font-medium">已禁用</span>}
                          </p>
                        </div>
                      </div>
                    </td>

                    {/* Key / IP */}
                    <td className="px-6 py-4">
                      <p className="text-[11px] font-mono text-on-surface truncate max-w-[180px]" title={node.key}>{node.key}</p>
                      <p className="text-[10px] text-on-surface-variant mt-0.5">{node.ip || '-'}{node.port ? `:${node.port}` : ''}</p>
                    </td>

                    {/* CPU / Memory */}
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-2">
                        <div className="w-16 h-1.5 rounded-full bg-surface-container-high overflow-hidden">
                          <div
                            className={`h-full rounded-full ${(node.cpu_usage || 0) > 85 ? 'bg-tertiary' : 'bg-primary'}`}
                            style={{ width: `${Math.min(node.cpu_usage || 0, 100)}%` }}
                          />
                        </div>
                        <span className={`text-[11px] font-mono ${(node.cpu_usage || 0) > 85 ? 'text-tertiary' : 'text-on-surface-variant'}`}>
                          {(node.cpu_usage || 0).toFixed(1)}%
                        </span>
                      </div>
                      <div className="flex items-center gap-2 mt-1">
                        <div className="w-16 h-1.5 rounded-full bg-surface-container-high overflow-hidden">
                          <div
                            className={`h-full rounded-full ${(node.memory_usage || 0) > 85 ? 'bg-tertiary' : 'bg-secondary'}`}
                            style={{ width: `${Math.min(node.memory_usage || 0, 100)}%` }}
                          />
                        </div>
                        <span className="text-[11px] font-mono text-on-surface-variant">
                          {(node.memory_usage || 0).toFixed(1)}%
                        </span>
                      </div>
                    </td>

                    {/* Runners */}
                    <td className="px-6 py-4">
                      <span className="text-xs text-on-surface font-medium">{node.active_runners || 0}</span>
                      <span className="text-[11px] text-on-surface-variant"> / {node.max_runners || 0}</span>
                      <p className="text-[10px] text-on-surface-variant mt-0.5">可用 {node.available_runners || 0}</p>
                    </td>

                    {/* Heartbeat */}
                    <td className="px-6 py-4">
                      <span className="text-[11px] text-on-surface-variant font-mono">
                        {formatHeartbeat(node)}
                      </span>
                    </td>

                    {/* Actions */}
                    <td className="px-6 py-4">
                      <div className="flex items-center justify-end gap-1">
                        {/* Edit */}
                        <button
                          onClick={() => openEdit(node)}
                          className="p-1.5 rounded-lg hover:bg-surface-container-high transition-colors"
                          title="编辑"
                        >
                          <Settings className="w-3.5 h-3.5 text-on-surface-variant" />
                        </button>

                        {/* Enable / Disable */}
                        {node.enabled === false ? (
                          <button
                            onClick={() => enableMut.mutate(node._id)}
                            className="p-1.5 rounded-lg hover:bg-primary-fixed/20 transition-colors"
                            title="启用节点"
                          >
                            <Wifi className="w-3.5 h-3.5 text-primary" />
                          </button>
                        ) : (
                          <button
                            onClick={() => disableMut.mutate(node._id)}
                            className="p-1.5 rounded-lg hover:bg-surface-container-high transition-colors"
                            title="禁用节点"
                          >
                            <WifiOff className="w-3.5 h-3.5 text-on-surface-variant" />
                          </button>
                        )}

                        {/* Delete */}
                        <button
                          onClick={() => setDeletingNode(node)}
                          className="p-1.5 rounded-lg hover:bg-error-container/30 transition-colors group/del"
                          title="移除节点"
                        >
                          <Trash2 className="w-3.5 h-3.5 text-on-surface-variant group-hover/del:text-error" />
                        </button>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {/* ===== Edit Node Modal ===== */}
      {editingNode && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-on-surface/30" onClick={() => setEditingNode(null)} />
          <div className="relative bg-surface-container-lowest rounded-2xl shadow-editorial w-full max-w-md mx-4 p-6">
            <div className="flex items-center justify-between mb-6">
              <h2 className="text-base font-bold text-on-surface">编辑节点</h2>
              <button onClick={() => setEditingNode(null)} className="p-1.5 rounded-lg hover:bg-surface-container-high transition-colors">
                <X className="w-4 h-4 text-on-surface-variant" />
              </button>
            </div>

            <div className="space-y-4">
              <div className="space-y-1.5">
                <label className="text-xs font-medium text-on-surface-variant">节点名称</label>
                <input
                  type="text"
                  value={editForm.name}
                  onChange={(e) => setEditForm({ ...editForm, name: e.target.value })}
                  placeholder="节点显示名称"
                  className="w-full px-4 py-3 bg-surface-container-highest rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary/30 transition-all placeholder:text-outline-variant"
                />
              </div>
              <div className="space-y-1.5">
                <label className="text-xs font-medium text-on-surface-variant">描述</label>
                <textarea
                  value={editForm.description}
                  onChange={(e) => setEditForm({ ...editForm, description: e.target.value })}
                  placeholder="节点描述信息"
                  rows={2}
                  className="w-full px-4 py-3 bg-surface-container-highest rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary/30 transition-all resize-none placeholder:text-outline-variant"
                />
              </div>
              <div className="space-y-1.5">
                <label className="text-xs font-medium text-on-surface-variant">最大并发 Runner 数</label>
                <input
                  type="number"
                  min={1}
                  max={64}
                  value={editForm.max_runners}
                  onChange={(e) => setEditForm({ ...editForm, max_runners: Number(e.target.value) })}
                  className="w-full px-4 py-3 bg-surface-container-highest rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary/30 transition-all"
                />
              </div>

              {/* Read-only info */}
              <div className="p-3 rounded-lg bg-surface-container-low space-y-1.5">
                <div className="flex justify-between text-[11px]">
                  <span className="text-on-surface-variant">Key</span>
                  <span className="font-mono text-on-surface">{editingNode.key}</span>
                </div>
                <div className="flex justify-between text-[11px]">
                  <span className="text-on-surface-variant">IP</span>
                  <span className="font-mono text-on-surface">{editingNode.ip || '-'}</span>
                </div>
                <div className="flex justify-between text-[11px]">
                  <span className="text-on-surface-variant">状态</span>
                  <span className="text-on-surface">{statusMap[editingNode.status]?.label ?? editingNode.status}</span>
                </div>
                <div className="flex justify-between text-[11px]">
                  <span className="text-on-surface-variant">注册时间</span>
                  <span className="text-on-surface">{new Date(editingNode.created_at).toLocaleString('zh-CN')}</span>
                </div>
              </div>
            </div>

            <div className="flex items-center justify-end gap-2 mt-6 pt-4">
              <button
                onClick={() => setEditingNode(null)}
                className="px-4 py-2 text-xs text-on-surface-variant bg-surface-container-high rounded-lg hover:bg-surface-container-highest transition-colors"
              >
                取消
              </button>
              <button
                onClick={handleUpdate}
                disabled={updateMut.isPending}
                className="flex items-center gap-1.5 px-5 py-2 text-xs font-medium text-on-primary bg-gradient-to-br from-primary to-primary-container rounded-lg hover:opacity-90 active:scale-95 transition-all disabled:opacity-60 tonal-depth"
              >
                {updateMut.isPending && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
                保存更改
              </button>
            </div>
          </div>
        </div>
      )}

      {/* ===== Delete Confirmation ===== */}
      {deletingNode && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-on-surface/30" onClick={() => setDeletingNode(null)} />
          <div className="relative bg-surface-container-lowest rounded-2xl shadow-editorial w-full max-w-sm mx-4 p-6">
            <div className="flex flex-col items-center text-center">
              <div className="w-12 h-12 rounded-full bg-error-container/30 flex items-center justify-center mb-4">
                <AlertTriangle className="w-6 h-6 text-error" />
              </div>
              <h3 className="text-sm font-bold text-on-surface">确认移除节点</h3>
              <p className="text-xs text-on-surface-variant mt-2 leading-relaxed">
                即将移除节点 <span className="font-medium text-on-surface">{deletingNode.name || deletingNode.key}</span>，
                节点记录将被删除。如该节点仍在运行，它会在下次心跳时重新注册。
              </p>
            </div>
            <div className="flex items-center gap-2 mt-6">
              <button
                onClick={() => setDeletingNode(null)}
                className="flex-1 px-4 py-2 text-xs text-on-surface-variant bg-surface-container-high rounded-lg hover:bg-surface-container-highest transition-colors"
              >
                取消
              </button>
              <button
                onClick={() => deleteMut.mutate(deletingNode._id)}
                disabled={deleteMut.isPending}
                className="flex-1 flex items-center justify-center gap-1.5 px-4 py-2 text-xs font-medium text-on-primary bg-error rounded-lg hover:opacity-90 transition-all disabled:opacity-60"
              >
                {deleteMut.isPending && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
                确认移除
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
