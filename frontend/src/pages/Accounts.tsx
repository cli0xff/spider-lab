import { useState, useEffect, useRef } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  getXueqiuCookies, deleteXueqiuCookie, updateXueqiuCookieStatus,
  checkXueqiuCookie, startQRLogin, getQRLoginStatus, cancelQRLogin,
  getTargetAccounts, addTargetAccount, deleteTargetAccount,
  searchXueqiuUser, searchWeiboUser, lookupXhsUser, searchWechatUser,
  getWechatCookies, deleteWechatCookie, updateWechatCookieStatus,
  checkWechatCookie, startWechatQRLogin, getWechatQRLoginStatus, cancelWechatQRLogin,
  addWechatCookieManual,
} from '../lib/api';
import type { XueqiuCookie, QRLoginSession, TargetAccount, WechatCookie } from '../types';
import {
  Plus, Trash2, RefreshCw, ShieldCheck, ShieldOff, Loader2,
  X, QrCode, CheckCircle, AlertTriangle, Clock, Ban,
  Info, Search, BadgeCheck, UserPlus,
} from 'lucide-react';

type Platform = 'xueqiu' | 'weibo' | 'xhs' | 'wechat';

const platforms: { id: Platform; label: string }[] = [
  { id: 'xueqiu', label: '雪球' },
  { id: 'weibo', label: '微博' },
  { id: 'xhs', label: '小红书' },
  { id: 'wechat', label: '微信' },
];

const statusConfig: Record<string, { label: string; color: string; icon: typeof ShieldCheck }> = {
  active: { label: '正常', color: 'bg-primary text-on-primary', icon: ShieldCheck },
  cooldown: { label: '冷却中', color: 'bg-tertiary text-on-tertiary', icon: Clock },
  expired: { label: '已过期', color: 'bg-error text-on-error', icon: AlertTriangle },
  disabled: { label: '已禁用', color: 'bg-outline text-surface', icon: Ban },
};

function timeAgo(dateStr: string) {
  if (!dateStr) return '-';
  const d = new Date(dateStr);
  if (d.getTime() === 0 || isNaN(d.getTime())) return '-';
  const now = Date.now();
  const diff = now - d.getTime();
  if (diff < 60000) return '刚刚';
  if (diff < 3600000) return `${Math.floor(diff / 60000)}分钟前`;
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}小时前`;
  return `${Math.floor(diff / 86400000)}天前`;
}

function formatCooldown(seconds: number) {
  if (seconds <= 0) return '';
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return m > 0 ? `${m}分${s > 0 ? s + '秒' : ''}` : `${s}秒`;
}

function formatFollowers(n: number) {
  if (!n) return '';
  if (n >= 10000) return `${(n / 10000).toFixed(1)}万`;
  return String(n);
}

// ==================== Unified Target Accounts (all platforms) ====================

function UnifiedTargetAccounts({ showToast }: { showToast: (msg: string, type?: 'success' | 'error') => void }) {
  const queryClient = useQueryClient();
  const [showAddModal, setShowAddModal] = useState(false);
  const [addPlatform, setAddPlatform] = useState<Platform>('xueqiu');
  const [searchQ, setSearchQ] = useState('');
  const [searchResults, setSearchResults] = useState<any[]>([]);
  const [searching, setSearching] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const composingRef = useRef(false);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const { data: allAccounts = [], isLoading } = useQuery({
    queryKey: ['target-accounts'],
    queryFn: () => getTargetAccounts(),
  });

  const grouped = platforms.reduce((acc, p) => {
    acc[p.id] = allAccounts.filter((a: TargetAccount) => a.platform === p.id);
    return acc;
  }, {} as Record<Platform, TargetAccount[]>);

  const addMut = useMutation({
    mutationFn: addTargetAccount,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['target-accounts'] });
      showToast('目标账号已添加');
    },
    onError: (e: Error) => showToast(e.message || '添加失败', 'error'),
  });

  const deleteMut = useMutation({
    mutationFn: deleteTargetAccount,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['target-accounts'] });
      setDeletingId(null);
      showToast('目标账号已移除');
    },
    onError: (e: Error) => showToast(e.message || '删除失败', 'error'),
  });

  const doSearch = async (q: string, platform: Platform) => {
    if (!q.trim()) { setSearchResults([]); return; }
    setSearching(true);
    try {
      if (platform === 'xueqiu') {
        const results = await searchXueqiuUser(q);
        setSearchResults((results || []).map((r: any) => ({
          user_id: String(r.id), nickname: r.screen_name, avatar_url: r.avatar_url,
          description: r.description, followers_count: r.followers_count,
          verified: r.verified,
        })));
      } else if (platform === 'weibo') {
        const results = await searchWeiboUser(q);
        setSearchResults((results || []).map((r: any) => ({
          user_id: r.uid, nickname: r.screen_name, avatar_url: r.avatar_url,
          description: r.description, followers_count: r.followers_count,
          verified: r.verified,
        })));
      } else if (platform === 'xhs') {
        const r = await lookupXhsUser(q);
        if (r && (r.nickname || r.user_id)) {
          setSearchResults([{
            user_id: r.user_id, nickname: r.nickname, avatar_url: r.avatar,
            description: r.desc, followers_count: r.followers_count ?? 0,
            verified: r.verified ?? false,
          }]);
        } else {
          setSearchResults([]);
        }
      } else if (platform === 'wechat') {
        const results = await searchWechatUser(q);
        setSearchResults((results || []).map((r: any) => ({
          user_id: r.id, nickname: r.nickname, avatar_url: r.avatar_url,
          description: r.description, followers_count: r.followers_count ?? 0,
          verified: r.verified ?? true,
        })));
      }
    } catch {
      setSearchResults([]);
    } finally {
      setSearching(false);
    }
  };

  const handleSearchInput = (val: string) => {
    setSearchQ(val);
    if (composingRef.current) return;
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => doSearch(val, addPlatform), 500);
  };

  const handlePlatformSwitch = (p: Platform) => {
    setAddPlatform(p);
    setSearchQ('');
    setSearchResults([]);
  };

  const handleAdd = (item: any) => {
    addMut.mutate({
      platform: addPlatform,
      user_id: item.user_id,
      nickname: item.nickname,
      avatar_url: item.avatar_url,
      description: item.description,
      followers_count: item.followers_count,
      verified: item.verified,
    });
    setSearchResults((prev) => prev.filter((r) => r.user_id !== item.user_id));
  };

  const existingIds = new Set(allAccounts.map((a: TargetAccount) => `${a.platform}:${a.user_id}`));

  const openAddModal = (p?: Platform) => {
    if (p) setAddPlatform(p);
    setShowAddModal(true);
    setSearchQ('');
    setSearchResults([]);
  };

  const closeAddModal = () => {
    setShowAddModal(false);
    setSearchQ('');
    setSearchResults([]);
  };

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h2 className="text-base font-bold text-on-surface">目标账号</h2>
        <button
          onClick={() => openAddModal()}
          className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-primary bg-primary/10 rounded-lg hover:bg-primary/20 transition-colors"
        >
          <UserPlus className="w-3.5 h-3.5" />
          添加目标
        </button>
      </div>

      {/* Platform card areas — side by side */}
      {isLoading ? (
        <div className="flex items-center justify-center py-8">
          <Loader2 className="w-5 h-5 animate-spin text-primary" />
        </div>
      ) : (
        <div className="grid grid-cols-3 gap-3">
          {platforms.map((p) => {
            const accounts = grouped[p.id];
            return (
              <div key={p.id} className="bg-surface-container-low rounded-xl flex flex-col min-h-[160px]">
                {/* Platform header */}
                <div className="flex items-center justify-between px-3 py-2.5">
                  <div className="flex items-center gap-1.5">
                    <span className="text-sm font-semibold text-on-surface">{p.label}</span>
                    <span className="text-[10px] text-on-surface-variant bg-surface-container-high px-1.5 py-0.5 rounded-full">
                      {accounts.length}
                    </span>
                  </div>
                  <button
                    onClick={() => openAddModal(p.id)}
                    className="p-1 rounded-lg hover:bg-surface-container-high transition-colors text-on-surface-variant"
                    title={`添加${p.label}目标`}
                  >
                    <Plus className="w-3.5 h-3.5" />
                  </button>
                </div>

                {/* Account list */}
                {accounts.length === 0 ? (
                  <div className="flex-1 flex items-center justify-center px-3 pb-3">
                    <p className="text-[10px] text-on-surface-variant/50">暂无目标</p>
                  </div>
                ) : (
                  <div className="px-2 pb-2 space-y-1 flex-1 overflow-y-auto max-h-[320px]">
                    {accounts.map((acc) => (
                      <div key={acc._id} className="group/acc flex items-center gap-2 px-2 py-1.5 rounded-lg bg-surface-container-lowest/60 hover:bg-surface-container-lowest transition-colors">
                        <div className="w-7 h-7 rounded-full bg-surface-container-high flex items-center justify-center overflow-hidden shrink-0">
                          {acc.avatar_url ? (
                            <img src={acc.avatar_url} alt="" className="w-full h-full object-cover" />
                          ) : (
                            <span className="text-[10px] font-bold text-on-surface-variant">{(acc.nickname || '?')[0]}</span>
                          )}
                        </div>
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-1">
                            <span className="text-xs font-semibold text-on-surface truncate">{acc.nickname || acc.user_id}</span>
                            {acc.verified && <BadgeCheck className="w-3 h-3 text-amber-500 shrink-0" />}
                          </div>
                          <div className="flex items-center gap-1.5 text-[10px] text-on-surface-variant leading-tight">
                            {acc.followers_count > 0 && <span>{formatFollowers(acc.followers_count)}粉丝</span>}
                          </div>
                        </div>
                        <button
                          onClick={() => setDeletingId(acc._id)}
                          className="p-1 rounded-lg opacity-0 group-hover/acc:opacity-100 hover:bg-error-container/30 transition-all text-error/70 shrink-0"
                          title="移除"
                        >
                          <Trash2 className="w-3 h-3" />
                        </button>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}

      {/* Add Target Modal (multi-platform) */}
      {showAddModal && (
        <div className="fixed inset-0 bg-black/40 z-50 flex items-center justify-center" onClick={closeAddModal}>
          <div className="bg-surface-container-lowest rounded-2xl w-[460px] max-w-[90vw] max-h-[80vh] flex flex-col" onClick={e => e.stopPropagation()}>
            {/* Modal header */}
            <div className="flex items-center justify-between p-5 pb-0">
              <h2 className="text-base font-bold text-on-surface">添加目标账号</h2>
              <button onClick={closeAddModal} className="p-1 rounded-lg hover:bg-surface-container-high">
                <X className="w-5 h-5 text-on-surface-variant" />
              </button>
            </div>

            {/* Platform selector */}
            <div className="flex gap-1 mx-5 mt-4 bg-surface-container-low rounded-lg p-1">
              {platforms.map((p) => (
                <button
                  key={p.id}
                  onClick={() => handlePlatformSwitch(p.id)}
                  className={`flex-1 px-3 py-2 rounded-md text-xs font-medium transition-all ${
                    addPlatform === p.id
                      ? 'bg-surface-container-lowest text-primary tonal-depth'
                      : 'text-on-surface-variant hover:bg-surface-container-high'
                  }`}
                >
                  {p.label}
                </button>
              ))}
            </div>

            {/* Search input */}
            <div className="px-5 mt-3">
              <div className="relative">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-on-surface-variant" />
                <input
                  type="text"
                  value={searchQ}
                  onChange={(e) => handleSearchInput(e.target.value)}
                  onCompositionStart={() => { composingRef.current = true; }}
                  onCompositionEnd={(e) => { composingRef.current = false; handleSearchInput((e.target as HTMLInputElement).value); }}
                  placeholder={addPlatform === 'xhs' ? '输入小红书用户ID（如 5fa7...）' : addPlatform === 'wechat' ? '搜索公众号名称...' : '搜索用户昵称...'}
                  className="w-full pl-9 pr-4 py-2.5 text-sm bg-surface-container-high rounded-lg outline-none focus:ring-2 focus:ring-primary/20 transition-all placeholder:text-outline-variant"
                  autoFocus
                />
                {searching && <Loader2 className="absolute right-3 top-1/2 -translate-y-1/2 w-4 h-4 animate-spin text-primary" />}
              </div>
            </div>

            {/* Search results */}
            <div className="flex-1 overflow-y-auto px-5 py-3 space-y-2">
              {searchResults.map((item) => {
                const alreadyAdded = existingIds.has(`${addPlatform}:${item.user_id}`);
                return (
                  <div key={item.user_id} className="flex items-center gap-3 p-3 rounded-lg bg-surface-container-low">
                    <div className="w-9 h-9 rounded-full bg-surface-container-high flex items-center justify-center overflow-hidden shrink-0">
                      {item.avatar_url ? (
                        <img src={item.avatar_url} alt="" className="w-full h-full object-cover" />
                      ) : (
                        <span className="text-sm font-bold text-on-surface-variant">{(item.nickname || '?')[0]}</span>
                      )}
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-1.5">
                        <span className="text-sm font-semibold text-on-surface truncate">{item.nickname || item.user_id}</span>
                        {item.verified && <BadgeCheck className="w-3.5 h-3.5 text-amber-500 shrink-0" />}
                        {item.followers_count > 0 && (
                          <span className="text-[10px] text-on-surface-variant ml-1">{formatFollowers(item.followers_count)}粉丝</span>
                        )}
                      </div>
                      {item.description && (
                        <p className="text-xs text-on-surface-variant truncate mt-0.5">{item.description}</p>
                      )}
                    </div>
                    <button
                      onClick={() => handleAdd(item)}
                      disabled={alreadyAdded || addMut.isPending}
                      className={`px-3 py-1.5 text-xs font-medium rounded-lg transition-colors shrink-0 ${
                        alreadyAdded
                          ? 'bg-surface-container-high text-on-surface-variant cursor-default'
                          : 'bg-primary/10 text-primary hover:bg-primary/20'
                      }`}
                    >
                      {alreadyAdded ? '已添加' : '添加'}
                    </button>
                  </div>
                );
              })}
              {searchQ && !searching && searchResults.length === 0 && (
                <p className="text-xs text-on-surface-variant text-center py-4">
                  {addPlatform === 'wechat' ? '未找到匹配的公众号' : '未找到匹配用户'}
                </p>
              )}
              {!searchQ && (
                <p className="text-xs text-on-surface-variant text-center py-4">
                  {addPlatform === 'xhs' ? '输入小红书用户ID查找' : addPlatform === 'wechat' ? '输入公众号名称开始搜索' : '输入用户昵称开始搜索'}
                </p>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Delete Confirm Modal */}
      {deletingId && (
        <div className="fixed inset-0 bg-black/40 z-50 flex items-center justify-center" onClick={() => setDeletingId(null)}>
          <div className="bg-surface-container-lowest rounded-2xl p-6 w-[360px] max-w-[90vw]" onClick={e => e.stopPropagation()}>
            <h2 className="text-base font-bold text-on-surface mb-2">确认移除</h2>
            <p className="text-sm text-on-surface-variant mb-4">移除后该用户将不再出现在目标账号列表中。</p>
            <div className="flex justify-end gap-2">
              <button
                onClick={() => setDeletingId(null)}
                className="px-4 py-2 text-sm text-on-surface-variant bg-surface-container-high rounded-lg hover:bg-surface-container-highest transition-colors"
              >
                取消
              </button>
              <button
                onClick={() => deleteMut.mutate(deletingId)}
                disabled={deleteMut.isPending}
                className="px-4 py-2 text-sm text-on-error bg-error rounded-lg hover:opacity-90 transition-all"
              >
                {deleteMut.isPending ? '移除中...' : '确认移除'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// ==================== Xueqiu Working Accounts ====================

function XueqiuWorkingAccounts({ showToast }: { showToast: (msg: string, type?: 'success' | 'error') => void }) {
  const queryClient = useQueryClient();
  const [showQR, setShowQR] = useState(false);
  const [qrSessionId, setQrSessionId] = useState<string | null>(null);
  const [qrSession, setQrSession] = useState<QRLoginSession | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [checkingId, setCheckingId] = useState<string | null>(null);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const { data: cookies = [], isLoading } = useQuery({
    queryKey: ['xueqiu-cookies'],
    queryFn: getXueqiuCookies,
    refetchInterval: (query) => {
      const items: XueqiuCookie[] = (query.state.data as any) ?? [];
      return items.some((c) => c.effective_status === 'cooldown') ? 10000 : 30000;
    },
  });

  const deleteMut = useMutation({
    mutationFn: deleteXueqiuCookie,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['xueqiu-cookies'] });
      setDeletingId(null);
      showToast('账号已移除');
    },
    onError: (e: Error) => showToast(e.message || '删除失败', 'error'),
  });

  const toggleMut = useMutation({
    mutationFn: ({ id, status }: { id: string; status: string }) =>
      updateXueqiuCookieStatus(id, status),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['xueqiu-cookies'] });
      showToast('状态已更新');
    },
    onError: (e: Error) => showToast(e.message || '操作失败', 'error'),
  });

  const checkMut = useMutation({
    mutationFn: checkXueqiuCookie,
    onSuccess: (data: any) => {
      queryClient.invalidateQueries({ queryKey: ['xueqiu-cookies'] });
      setCheckingId(null);
      if (data?.valid) {
        showToast('Cookie有效');
      } else {
        showToast(`Cookie已失效${data?.error ? ': ' + data.error : ''}`, 'error');
      }
    },
    onError: (e: Error) => {
      setCheckingId(null);
      showToast(e.message || '检查失败', 'error');
    },
  });

  // QR Login flow
  const handleStartQR = async () => {
    setShowQR(true);
    setQrSession(null);
    setQrSessionId(null);
    try {
      const result = await startQRLogin();
      const sid = result?.session_id;
      if (!sid) throw new Error('No session_id returned');
      setQrSessionId(sid);
      setQrSession({ id: sid, status: 'pending' });
    } catch (e: any) {
      setQrSession({ id: '', status: 'error', error: e.message || '启动失败' });
    }
  };

  // Poll QR status
  useEffect(() => {
    if (!qrSessionId || !showQR) return;

    const poll = async () => {
      try {
        const status = await getQRLoginStatus(qrSessionId);
        setQrSession(status);

        if (status.status === 'success') {
          if (pollRef.current) clearInterval(pollRef.current);
          queryClient.invalidateQueries({ queryKey: ['xueqiu-cookies'] });
          showToast(`账号 ${status.nickname || status.uid} 添加成功`);
          setTimeout(() => setShowQR(false), 2000);
        } else if (status.status === 'expired' || status.status === 'error') {
          if (pollRef.current) clearInterval(pollRef.current);
        }
      } catch {
        // ignore transient poll errors
      }
    };

    pollRef.current = setInterval(poll, 2000);
    poll(); // immediate first poll

    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, [qrSessionId, showQR, queryClient]);

  const handleCancelQR = () => {
    if (qrSessionId) cancelQRLogin(qrSessionId).catch((err) => console.warn('Failed to cancel QR login:', err));
    if (pollRef.current) clearInterval(pollRef.current);
    setShowQR(false);
    setQrSession(null);
    setQrSessionId(null);
  };

  const activeCount = cookies.filter((c: XueqiuCookie) => c.effective_status === 'active').length;
  const totalTasks = cookies.reduce((acc: number, c: XueqiuCookie) => acc + (c.total_tasks || 0), 0);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold text-on-surface">工作账号</h3>
        <button
          onClick={handleStartQR}
          className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-on-primary bg-gradient-to-br from-primary to-primary-container rounded-lg hover:opacity-90 active:scale-95 transition-all tonal-depth"
        >
          <Plus className="w-3.5 h-3.5" />
          扫码添加
        </button>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-3 gap-3">
        <div className="bg-surface-container-low rounded-xl p-3">
          <div className="text-xl font-bold text-on-surface">{cookies.length}</div>
          <div className="text-[10px] text-on-surface-variant mt-0.5">总账号</div>
        </div>
        <div className="bg-surface-container-low rounded-xl p-3">
          <div className="text-xl font-bold text-primary">{activeCount}</div>
          <div className="text-[10px] text-on-surface-variant mt-0.5">可用</div>
        </div>
        <div className="bg-surface-container-low rounded-xl p-3">
          <div className="text-xl font-bold text-on-surface">{totalTasks}</div>
          <div className="text-[10px] text-on-surface-variant mt-0.5">累计任务</div>
        </div>
      </div>

      {/* Cookie List */}
      {isLoading ? (
        <div className="flex items-center justify-center py-10">
          <Loader2 className="w-5 h-5 animate-spin text-primary" />
        </div>
      ) : cookies.length === 0 ? (
        <div className="text-center py-10 text-on-surface-variant">
          <QrCode className="w-10 h-10 mx-auto mb-3 opacity-30" />
          <p className="text-xs">暂无工作账号，点击「扫码添加」通过雪球APP扫码登录</p>
        </div>
      ) : (
        <div className="space-y-2">
          {cookies.map((ck: XueqiuCookie) => {
            const displayStatus = ck.effective_status || ck.status;
            const cfg = statusConfig[displayStatus] || statusConfig.disabled;
            const StatusIcon = cfg.icon;
            return (
              <div key={ck._id} className="bg-surface-container-low rounded-xl p-3 flex items-center gap-3">
                <div className="w-9 h-9 rounded-full bg-surface-container-high flex items-center justify-center overflow-hidden shrink-0">
                  {ck.avatar_url ? (
                    <img src={ck.avatar_url} alt="" className="w-full h-full object-cover" />
                  ) : (
                    <span className="text-sm font-bold text-on-surface-variant">{(ck.nickname || '?')[0]}</span>
                  )}
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="font-semibold text-sm text-on-surface truncate">
                      {ck.nickname || ck.xueqiu_uid || '未知用户'}
                    </span>
                    <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-medium ${cfg.color}`}>
                      <StatusIcon className="w-3 h-3" />
                      {cfg.label}
                      {displayStatus === 'cooldown' && ck.cooldown_left > 0 && (
                        <span className="ml-0.5">{formatCooldown(ck.cooldown_left)}</span>
                      )}
                    </span>
                  </div>
                  <div className="flex items-center gap-3 mt-0.5 text-xs text-on-surface-variant">
                    <span>UID: {ck.xueqiu_uid || '-'}</span>
                    <span>今日: {ck.daily_tasks || 0}次</span>
                    <span>累计: {ck.total_tasks || 0}次</span>
                    <span>最近: {timeAgo(ck.last_used_at)}</span>
                    {ck.error_count > 0 && (
                      <span className="text-error">错误: {ck.error_count}次</span>
                    )}
                  </div>
                </div>
                <div className="flex items-center gap-0.5 shrink-0">
                  {displayStatus === 'expired' && (
                    <button onClick={handleStartQR} className="flex items-center gap-1 px-2 py-1 rounded-lg bg-primary/10 hover:bg-primary/20 transition-colors text-primary text-[10px] font-medium" title="刷新登录">
                      <QrCode className="w-3 h-3" />刷新
                    </button>
                  )}
                  <button onClick={() => { setCheckingId(ck._id); checkMut.mutate(ck._id); }} disabled={checkingId === ck._id} className="p-1.5 rounded-lg hover:bg-surface-container-high transition-colors text-on-surface-variant" title="检查">
                    {checkingId === ck._id ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <RefreshCw className="w-3.5 h-3.5" />}
                  </button>
                  <button onClick={() => toggleMut.mutate({ id: ck._id, status: ck.status === 'disabled' ? 'active' : 'disabled' })} className="p-1.5 rounded-lg hover:bg-surface-container-high transition-colors text-on-surface-variant" title={ck.status === 'disabled' ? '启用' : '禁用'}>
                    {ck.status === 'disabled' ? <ShieldCheck className="w-3.5 h-3.5" /> : <ShieldOff className="w-3.5 h-3.5" />}
                  </button>
                  <button onClick={() => setDeletingId(ck._id)} className="p-1.5 rounded-lg hover:bg-error-container/30 transition-colors text-error/70" title="删除">
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* Anti-bot Info */}
      <div className="bg-surface-container-low rounded-xl p-3">
        <h4 className="text-xs font-semibold text-on-surface mb-1.5">防封策略</h4>
        <div className="grid grid-cols-2 gap-2 text-[10px] text-on-surface-variant">
          <div className="flex items-center gap-1.5"><Clock className="w-3 h-3 text-primary" /><span>使用后自动冷却</span></div>
          <div className="flex items-center gap-1.5"><ShieldCheck className="w-3 h-3 text-primary" /><span>每天最多 20 个任务</span></div>
          <div className="flex items-center gap-1.5"><RefreshCw className="w-3 h-3 text-primary" /><span>自动轮换最久未用</span></div>
          <div className="flex items-center gap-1.5"><AlertTriangle className="w-3 h-3 text-primary" /><span>3 次错误自动过期</span></div>
        </div>
      </div>

      {/* QR Login Modal */}
      {showQR && (
        <div className="fixed inset-0 bg-black/40 z-50 flex items-center justify-center" onClick={handleCancelQR}>
          <div className="bg-surface-container-lowest rounded-2xl p-6 w-[380px] max-w-[90vw]" onClick={e => e.stopPropagation()}>
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-base font-bold text-on-surface">扫码登录雪球</h2>
              <button onClick={handleCancelQR} className="p-1 rounded-lg hover:bg-surface-container-high">
                <X className="w-5 h-5 text-on-surface-variant" />
              </button>
            </div>
            <div className="flex flex-col items-center py-4">
              {(!qrSession || qrSession.status === 'pending') && (
                <>
                  <Loader2 className="w-8 h-8 animate-spin text-primary mb-3" />
                  <p className="text-sm text-on-surface-variant">正在启动浏览器...</p>
                  <p className="text-xs text-on-surface-variant/60 mt-1">首次可能需要 10-15 秒</p>
                </>
              )}
              {qrSession?.status === 'qr_ready' && qrSession.qr_image && (
                <>
                  <div className="w-48 h-48 bg-white rounded-xl p-2 mb-3">
                    <img src={`data:image/png;base64,${qrSession.qr_image}`} alt="QR Code" className="w-full h-full" />
                  </div>
                  <p className="text-sm font-medium text-on-surface">打开雪球APP扫描二维码</p>
                  <p className="text-xs text-on-surface-variant mt-1">二维码有效期约 2 分钟</p>
                </>
              )}
              {qrSession?.status === 'success' && (
                <>
                  <CheckCircle className="w-12 h-12 text-primary mb-3" />
                  <p className="text-sm font-medium text-on-surface">{qrSession.nickname || qrSession.uid} 登录成功</p>
                </>
              )}
              {qrSession?.status === 'expired' && (
                <>
                  <AlertTriangle className="w-12 h-12 text-error/60 mb-3" />
                  <p className="text-sm text-on-surface-variant">二维码已过期</p>
                  <button onClick={handleStartQR} className="mt-3 px-4 py-2 text-sm text-primary bg-primary/10 rounded-lg hover:bg-primary/20 transition-colors">重新获取</button>
                </>
              )}
              {qrSession?.status === 'error' && (
                <>
                  <AlertTriangle className="w-12 h-12 text-error/60 mb-3" />
                  <p className="text-sm text-error">{qrSession.error || '发生错误'}</p>
                  <button onClick={handleStartQR} className="mt-3 px-4 py-2 text-sm text-primary bg-primary/10 rounded-lg hover:bg-primary/20 transition-colors">重试</button>
                </>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Delete Confirm Modal */}
      {deletingId && (
        <div className="fixed inset-0 bg-black/40 z-50 flex items-center justify-center" onClick={() => setDeletingId(null)}>
          <div className="bg-surface-container-lowest rounded-2xl p-6 w-[360px] max-w-[90vw]" onClick={e => e.stopPropagation()}>
            <h2 className="text-base font-bold text-on-surface mb-2">确认删除</h2>
            <p className="text-sm text-on-surface-variant mb-4">删除后该账号的Cookie将不再用于爬虫任务。</p>
            <div className="flex justify-end gap-2">
              <button onClick={() => setDeletingId(null)} className="px-4 py-2 text-sm text-on-surface-variant bg-surface-container-high rounded-lg hover:bg-surface-container-highest transition-colors">取消</button>
              <button onClick={() => deleteMut.mutate(deletingId)} disabled={deleteMut.isPending} className="px-4 py-2 text-sm text-on-error bg-error rounded-lg hover:opacity-90 transition-all">
                {deleteMut.isPending ? '删除中...' : '确认删除'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// ==================== Working Accounts: Weibo / XHS info ====================

function WechatWorkingAccounts({ showToast }: { showToast: (msg: string, type?: 'success' | 'error') => void }) {
  const queryClient = useQueryClient();
  const [showQR, setShowQR] = useState(false);
  const [showManual, setShowManual] = useState(false);
  const [qrSessionId, setQrSessionId] = useState<string | null>(null);
  const [qrSession, setQrSession] = useState<QRLoginSession | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [checkingId, setCheckingId] = useState<string | null>(null);
  const [manualForm, setManualForm] = useState({ nickname: '', cookie: '', token: '', mp_account: '', notes: '' });
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const { data: cookies = [], isLoading } = useQuery({
    queryKey: ['wechat-cookies'],
    queryFn: getWechatCookies,
    refetchInterval: (query) => {
      const items: WechatCookie[] = (query.state.data as any) ?? [];
      return items.some((c) => c.effective_status === 'cooldown') ? 10000 : 30000;
    },
  });

  const deleteMut = useMutation({
    mutationFn: deleteWechatCookie,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['wechat-cookies'] });
      setDeletingId(null);
      showToast('账号已移除');
    },
    onError: (e: Error) => showToast(e.message || '删除失败', 'error'),
  });

  const toggleMut = useMutation({
    mutationFn: ({ id, status }: { id: string; status: string }) =>
      updateWechatCookieStatus(id, status),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['wechat-cookies'] });
      showToast('状态已更新');
    },
    onError: (e: Error) => showToast(e.message || '操作失败', 'error'),
  });

  const checkMut = useMutation({
    mutationFn: checkWechatCookie,
    onSuccess: (data: any) => {
      queryClient.invalidateQueries({ queryKey: ['wechat-cookies'] });
      setCheckingId(null);
      if (data?.valid) {
        showToast('Cookie有效');
      } else {
        showToast(`Cookie已失效${data?.error ? ': ' + data.error : ''}`, 'error');
      }
    },
    onError: (e: Error) => {
      setCheckingId(null);
      showToast(e.message || '检查失败', 'error');
    },
  });

  const manualAddMut = useMutation({
    mutationFn: addWechatCookieManual,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['wechat-cookies'] });
      setShowManual(false);
      setManualForm({ nickname: '', cookie: '', token: '', mp_account: '', notes: '' });
      showToast('账号已添加');
    },
    onError: (e: Error) => showToast(e.message || '添加失败', 'error'),
  });

  // QR Login flow
  const handleStartQR = async () => {
    setShowQR(true);
    setQrSession(null);
    setQrSessionId(null);
    try {
      const result = await startWechatQRLogin();
      const sid = result?.session_id;
      if (!sid) throw new Error('No session_id returned');
      setQrSessionId(sid);
      setQrSession({ id: sid, status: 'pending' });
    } catch (e: any) {
      setQrSession({ id: '', status: 'error', error: e.message || '启动失败' });
    }
  };

  // Poll QR status
  useEffect(() => {
    if (!qrSessionId || !showQR) return;

    const poll = async () => {
      try {
        const status = await getWechatQRLoginStatus(qrSessionId);
        setQrSession(status);

        if (status.status === 'success') {
          if (pollRef.current) clearInterval(pollRef.current);
          queryClient.invalidateQueries({ queryKey: ['wechat-cookies'] });
          showToast(`账号 ${status.nickname || ''} 添加成功`);
          setTimeout(() => setShowQR(false), 2000);
        } else if (status.status === 'expired' || status.status === 'error') {
          if (pollRef.current) clearInterval(pollRef.current);
        }
      } catch {
        // ignore transient poll errors
      }
    };

    pollRef.current = setInterval(poll, 2000);
    poll(); // immediate first poll

    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, [qrSessionId, showQR, queryClient]);

  const handleCancelQR = () => {
    if (qrSessionId) cancelWechatQRLogin(qrSessionId).catch((err) => console.warn('Failed to cancel WeChat QR login:', err));
    if (pollRef.current) clearInterval(pollRef.current);
    setShowQR(false);
    setQrSession(null);
    setQrSessionId(null);
  };

  const handleManualAdd = () => {
    if (!manualForm.nickname || !manualForm.cookie) {
      showToast('公众号名称和Cookie不能为空', 'error');
      return;
    }
    manualAddMut.mutate(manualForm);
  };

  const activeCount = cookies.filter((c: WechatCookie) => c.effective_status === 'active').length;
  const totalTasks = cookies.reduce((acc: number, c: WechatCookie) => acc + (c.total_tasks || 0), 0);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold text-on-surface">工作账号</h3>
        <div className="flex gap-2">
          <button
            onClick={() => setShowManual(true)}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-on-primary bg-surface-container-high rounded-lg hover:bg-surface-container-highest transition-colors"
          >
            <Plus className="w-3.5 h-3.5" />
            手动添加
          </button>
          <button
            onClick={handleStartQR}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-on-primary bg-gradient-to-br from-primary to-primary-container rounded-lg hover:opacity-90 active:scale-95 transition-all tonal-depth"
          >
            <QrCode className="w-3.5 h-3.5" />
            扫码添加
          </button>
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-3 gap-3">
        <div className="bg-surface-container-low rounded-xl p-3">
          <div className="text-xl font-bold text-on-surface">{cookies.length}</div>
          <div className="text-[10px] text-on-surface-variant mt-0.5">总账号</div>
        </div>
        <div className="bg-surface-container-low rounded-xl p-3">
          <div className="text-xl font-bold text-primary">{activeCount}</div>
          <div className="text-[10px] text-on-surface-variant mt-0.5">可用</div>
        </div>
        <div className="bg-surface-container-low rounded-xl p-3">
          <div className="text-xl font-bold text-on-surface">{totalTasks}</div>
          <div className="text-[10px] text-on-surface-variant mt-0.5">累计任务</div>
        </div>
      </div>

      {/* Cookie List */}
      {isLoading ? (
        <div className="flex items-center justify-center py-10">
          <Loader2 className="w-5 h-5 animate-spin text-primary" />
        </div>
      ) : cookies.length === 0 ? (
        <div className="text-center py-10 text-on-surface-variant">
          <QrCode className="w-10 h-10 mx-auto mb-3 opacity-30" />
          <p className="text-xs">暂无工作账号，点击「扫码添加」或「手动添加」</p>
        </div>
      ) : (
        <div className="space-y-2">
          {cookies.map((ck: WechatCookie) => {
            const displayStatus = ck.effective_status || ck.status;
            const cfg = statusConfig[displayStatus] || statusConfig.disabled;
            const StatusIcon = cfg.icon;
            return (
              <div key={ck._id} className="bg-surface-container-low rounded-xl p-3 flex items-center gap-3">
                <div className="w-9 h-9 rounded-full bg-surface-container-high flex items-center justify-center overflow-hidden shrink-0">
                  <span className="text-sm font-bold text-on-surface-variant">{(ck.nickname || '?')[0]}</span>
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="font-semibold text-sm text-on-surface truncate">
                      {ck.nickname || ck.mp_account || '未知账号'}
                    </span>
                    <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-medium ${cfg.color}`}>
                      <StatusIcon className="w-3 h-3" />
                      {cfg.label}
                      {displayStatus === 'cooldown' && ck.cooldown_left > 0 && (
                        <span className="ml-0.5">{formatCooldown(ck.cooldown_left)}</span>
                      )}
                    </span>
                  </div>
                  <div className="flex items-center gap-3 mt-0.5 text-xs text-on-surface-variant">
                    <span>账号: {ck.mp_account || '-'}</span>
                    <span>今日: {ck.daily_tasks || 0}次</span>
                    <span>累计: {ck.total_tasks || 0}次</span>
                    <span>最近: {timeAgo(ck.last_used_at)}</span>
                    {ck.error_count > 0 && (
                      <span className="text-error">错误: {ck.error_count}次</span>
                    )}
                  </div>
                </div>
                <div className="flex items-center gap-0.5 shrink-0">
                  {displayStatus === 'expired' && (
                    <button onClick={handleStartQR} className="flex items-center gap-1 px-2 py-1 rounded-lg bg-primary/10 hover:bg-primary/20 transition-colors text-primary text-[10px] font-medium" title="刷新登录">
                      <QrCode className="w-3 h-3" />刷新
                    </button>
                  )}
                  <button onClick={() => { setCheckingId(ck._id); checkMut.mutate(ck._id); }} disabled={checkingId === ck._id} className="p-1.5 rounded-lg hover:bg-surface-container-high transition-colors text-on-surface-variant" title="检查">
                    {checkingId === ck._id ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <RefreshCw className="w-3.5 h-3.5" />}
                  </button>
                  <button onClick={() => toggleMut.mutate({ id: ck._id, status: ck.status === 'disabled' ? 'active' : 'disabled' })} className="p-1.5 rounded-lg hover:bg-surface-container-high transition-colors text-on-surface-variant" title={ck.status === 'disabled' ? '启用' : '禁用'}>
                    {ck.status === 'disabled' ? <ShieldCheck className="w-3.5 h-3.5" /> : <ShieldOff className="w-3.5 h-3.5" />}
                  </button>
                  <button onClick={() => setDeletingId(ck._id)} className="p-1.5 rounded-lg hover:bg-error-container/30 transition-colors text-error/70" title="删除">
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* Anti-bot Info */}
      <div className="bg-surface-container-low rounded-xl p-3">
        <h4 className="text-xs font-semibold text-on-surface mb-1.5">防封策略</h4>
        <div className="grid grid-cols-2 gap-2 text-[10px] text-on-surface-variant">
          <div className="flex items-center gap-1.5"><Clock className="w-3 h-3 text-primary" /><span>使用后自动冷却</span></div>
          <div className="flex items-center gap-1.5"><ShieldCheck className="w-3 h-3 text-primary" /><span>每天最多 15 个任务</span></div>
          <div className="flex items-center gap-1.5"><RefreshCw className="w-3 h-3 text-primary" /><span>自动轮换最久未用</span></div>
          <div className="flex items-center gap-1.5"><AlertTriangle className="w-3 h-3 text-primary" /><span>3 次错误自动过期</span></div>
        </div>
      </div>

      {/* QR Login Modal */}
      {showQR && (
        <div className="fixed inset-0 bg-black/40 z-50 flex items-center justify-center" onClick={handleCancelQR}>
          <div className="bg-surface-container-lowest rounded-2xl p-6 w-[380px] max-w-[90vw]" onClick={e => e.stopPropagation()}>
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-base font-bold text-on-surface">扫码登录微信公众号</h2>
              <button onClick={handleCancelQR} className="p-1 rounded-lg hover:bg-surface-container-high">
                <X className="w-5 h-5 text-on-surface-variant" />
              </button>
            </div>
            <div className="flex flex-col items-center py-4">
              {(!qrSession || qrSession.status === 'pending') && (
                <>
                  <Loader2 className="w-8 h-8 animate-spin text-primary mb-3" />
                  <p className="text-sm text-on-surface-variant">正在启动浏览器...</p>
                  <p className="text-xs text-on-surface-variant/60 mt-1">首次可能需要 10-15 秒</p>
                </>
              )}
              {qrSession?.status === 'qr_ready' && qrSession.qr_image && (
                <>
                  <div className="w-48 h-48 bg-white rounded-xl p-2 mb-3">
                    <img src={`data:image/png;base64,${qrSession.qr_image}`} alt="QR Code" className="w-full h-full" />
                  </div>
                  <p className="text-sm font-medium text-on-surface">打开微信APP扫描二维码</p>
                  <p className="text-xs text-on-surface-variant mt-1">二维码有效期约 2 分钟</p>
                </>
              )}
              {qrSession?.status === 'success' && (
                <>
                  <CheckCircle className="w-12 h-12 text-primary mb-3" />
                  <p className="text-sm font-medium text-on-surface">{qrSession.nickname || ''} 登录成功</p>
                </>
              )}
              {qrSession?.status === 'expired' && (
                <>
                  <AlertTriangle className="w-12 h-12 text-error/60 mb-3" />
                  <p className="text-sm text-on-surface-variant">二维码已过期</p>
                  <button onClick={handleStartQR} className="mt-3 px-4 py-2 text-sm text-primary bg-primary/10 rounded-lg hover:bg-primary/20 transition-colors">重新获取</button>
                </>
              )}
              {qrSession?.status === 'error' && (
                <>
                  <AlertTriangle className="w-12 h-12 text-error/60 mb-3" />
                  <p className="text-sm text-error">{qrSession.error || '发生错误'}</p>
                  <button onClick={handleStartQR} className="mt-3 px-4 py-2 text-sm text-primary bg-primary/10 rounded-lg hover:bg-primary/20 transition-colors">重试</button>
                </>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Manual Add Modal */}
      {showManual && (
        <div className="fixed inset-0 bg-black/40 z-50 flex items-center justify-center" onClick={() => setShowManual(false)}>
          <div className="bg-surface-container-lowest rounded-2xl p-6 w-[460px] max-w-[90vw]" onClick={e => e.stopPropagation()}>
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-base font-bold text-on-surface">手动添加微信公众号账号</h2>
              <button onClick={() => setShowManual(false)} className="p-1 rounded-lg hover:bg-surface-container-high">
                <X className="w-5 h-5 text-on-surface-variant" />
              </button>
            </div>
            <div className="space-y-3">
              <div>
                <label className="text-xs font-medium text-on-surface mb-1 block">公众号名称 *</label>
                <input
                  type="text"
                  value={manualForm.nickname}
                  onChange={(e) => setManualForm({ ...manualForm, nickname: e.target.value })}
                  placeholder="公众号名称"
                  className="w-full px-3 py-2 text-sm bg-surface-container-high rounded-lg outline-none focus:ring-2 focus:ring-primary/20 transition-all"
                />
              </div>
              <div>
                <label className="text-xs font-medium text-on-surface mb-1 block">Cookie *</label>
                <textarea
                  value={manualForm.cookie}
                  onChange={(e) => setManualForm({ ...manualForm, cookie: e.target.value })}
                  placeholder="mp.weixin.qq.com Cookie 字符串"
                  rows={3}
                  className="w-full px-3 py-2 text-sm bg-surface-container-high rounded-lg outline-none focus:ring-2 focus:ring-primary/20 transition-all resize-none"
                />
              </div>
              <div>
                <label className="text-xs font-medium text-on-surface mb-1 block">Token (可选)</label>
                <input
                  type="text"
                  value={manualForm.token}
                  onChange={(e) => setManualForm({ ...manualForm, token: e.target.value })}
                  placeholder="MP Token (从URL中获取)"
                  className="w-full px-3 py-2 text-sm bg-surface-container-high rounded-lg outline-none focus:ring-2 focus:ring-primary/20 transition-all"
                />
              </div>
              <div>
                <label className="text-xs font-medium text-on-surface mb-1 block">公众号账号 (可选)</label>
                <input
                  type="text"
                  value={manualForm.mp_account}
                  onChange={(e) => setManualForm({ ...manualForm, mp_account: e.target.value })}
                  placeholder="微信公众号账号"
                  className="w-full px-3 py-2 text-sm bg-surface-container-high rounded-lg outline-none focus:ring-2 focus:ring-primary/20 transition-all"
                />
              </div>
              <div>
                <label className="text-xs font-medium text-on-surface mb-1 block">备注 (可选)</label>
                <input
                  type="text"
                  value={manualForm.notes}
                  onChange={(e) => setManualForm({ ...manualForm, notes: e.target.value })}
                  placeholder="备注信息"
                  className="w-full px-3 py-2 text-sm bg-surface-container-high rounded-lg outline-none focus:ring-2 focus:ring-primary/20 transition-all"
                />
              </div>
            </div>
            <div className="flex justify-end gap-2 mt-4">
              <button onClick={() => setShowManual(false)} className="px-4 py-2 text-sm text-on-surface-variant bg-surface-container-high rounded-lg hover:bg-surface-container-highest transition-colors">
                取消
              </button>
              <button onClick={handleManualAdd} disabled={manualAddMut.isPending} className="px-4 py-2 text-sm text-on-primary bg-primary rounded-lg hover:opacity-90 transition-all">
                {manualAddMut.isPending ? '添加中...' : '确认添加'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Delete Confirm Modal */}
      {deletingId && (
        <div className="fixed inset-0 bg-black/40 z-50 flex items-center justify-center" onClick={() => setDeletingId(null)}>
          <div className="bg-surface-container-lowest rounded-2xl p-6 w-[360px] max-w-[90vw]" onClick={e => e.stopPropagation()}>
            <h2 className="text-base font-bold text-on-surface mb-2">确认删除</h2>
            <p className="text-sm text-on-surface-variant mb-4">删除后该账号的Cookie将不再用于爬虫任务。</p>
            <div className="flex justify-end gap-2">
              <button onClick={() => setDeletingId(null)} className="px-4 py-2 text-sm text-on-surface-variant bg-surface-container-high rounded-lg hover:bg-surface-container-highest transition-colors">取消</button>
              <button onClick={() => deleteMut.mutate(deletingId)} disabled={deleteMut.isPending} className="px-4 py-2 text-sm text-on-error bg-error rounded-lg hover:opacity-90 transition-all">
                {deleteMut.isPending ? '删除中...' : '确认删除'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function WeiboWorkingInfo() {
  return (
    <div className="bg-surface-container-low rounded-xl p-4">
      <div className="flex items-start gap-3">
        <Info className="w-4 h-4 text-primary mt-0.5 shrink-0" />
        <div className="text-xs text-on-surface-variant leading-relaxed">
          <p className="font-medium text-on-surface mb-1">微博Cookie池即将上线</p>
          <p>
            目前微博爬虫支持在创建任务时手动填写Cookie。
            不填写时以未登录状态运行（部分功能受限）。
          </p>
        </div>
      </div>
    </div>
  );
}

function XhsWorkingInfo() {
  return (
    <div className="bg-surface-container-low rounded-xl p-4">
      <div className="flex items-start gap-3">
        <CheckCircle className="w-4 h-4 text-primary mt-0.5 shrink-0" />
        <div className="text-xs text-on-surface-variant leading-relaxed">
          <p className="font-medium text-on-surface mb-1">无需登录</p>
          <p>小红书爬虫通过 Playwright 访问公开页面，无需登录即可采集。</p>
        </div>
      </div>
    </div>
  );
}

// ==================== Main Accounts Page ====================

export default function Accounts() {
  const [workingTab, setWorkingTab] = useState<Platform>('xueqiu');
  const [toast, setToast] = useState<{ msg: string; type: 'success' | 'error' } | null>(null);

  const showToast = (msg: string, type: 'success' | 'error' = 'success') => {
    setToast({ msg, type });
    setTimeout(() => setToast(null), 3000);
  };

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div>
        <h1 className="text-xl font-bold text-on-surface">账号管理</h1>
        <p className="text-sm text-on-surface-variant mt-1">
          管理各平台的工作账号和采集目标用户
        </p>
      </div>

      {/* Working Accounts Section */}
      <div className="space-y-4">
        <h2 className="text-base font-bold text-on-surface">工作账号</h2>
        <div className="flex gap-1 bg-surface-container-low rounded-xl p-1">
          {platforms.map((p) => (
            <button
              key={p.id}
              onClick={() => setWorkingTab(p.id)}
              className={`flex-1 flex items-center justify-center px-4 py-2.5 rounded-lg text-sm font-medium transition-all duration-150 ${
                workingTab === p.id
                  ? 'bg-surface-container-lowest text-primary tonal-depth'
                  : 'text-on-surface-variant hover:bg-surface-container-high'
              }`}
            >
              {p.label}
            </button>
          ))}
        </div>
        {workingTab === 'xueqiu' && <XueqiuWorkingAccounts showToast={showToast} />}
        {workingTab === 'weibo' && <WeiboWorkingInfo />}
        {workingTab === 'xhs' && <XhsWorkingInfo />}
        {workingTab === 'wechat' && <WechatWorkingAccounts showToast={showToast} />}
      </div>

      {/* Divider */}
      <div className="h-px bg-outline-variant/20" />

      {/* Target Accounts — unified, grouped by platform */}
      <UnifiedTargetAccounts showToast={showToast} />

      {/* Toast */}
      {toast && (
        <div className={`fixed bottom-6 left-1/2 -translate-x-1/2 z-50 px-4 py-2.5 rounded-lg text-sm font-medium ${
          toast.type === 'success'
            ? 'bg-primary text-on-primary'
            : 'bg-error text-on-error'
        } tonal-depth animate-in`}>
          {toast.msg}
        </div>
      )}
    </div>
  );
}
