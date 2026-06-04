import { useState, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Save, Eye, EyeOff, Copy, Camera, Bell, Webhook, Mail, Loader2, AlertTriangle } from 'lucide-react';
import { getSettings, updateSetting, getMe, updateMe, getStatsOverview } from '../lib/api';
import type { Setting, User, StatsOverview } from '../types';

/* ------------------------------------------------------------------ */
/*  Crawl-slider metadata for known setting keys                      */
/* ------------------------------------------------------------------ */
const CRAWL_META: Record<
  string,
  { label: string; min: number; max: number; step: number; minLabel: string; maxLabel: string; fmt: (v: number) => string }
> = {
  crawl_frequency:          { label: '采集频率',     min: 0.5, max: 5,   step: 0.1, minLabel: '0.5s', maxLabel: '5.0s', fmt: v => `${v}s`  },
  max_concurrency:          { label: '并发任务上限', min: 8,   max: 256, step: 8,   minLabel: '8',    maxLabel: '256',  fmt: v => `${v}`   },
  proxy_rotation_interval:  { label: '代理切换间隔', min: 0,   max: 60,  step: 5,   minLabel: '立即', maxLabel: '60m',  fmt: v => v === 0 ? '立即' : `${v}m` },
};

/* ------------------------------------------------------------------ */
/*  Component                                                         */
/* ------------------------------------------------------------------ */
export default function Settings() {
  const queryClient = useQueryClient();

  /* ---- toast ---- */
  const [toast, setToast] = useState<{ msg: string; type: 'success' | 'error' } | null>(null);
  const showToast = (msg: string, type: 'success' | 'error' = 'success') => {
    setToast({ msg, type });
    setTimeout(() => setToast(null), 3000);
  };

  /* ---- show / hide api keys ---- */
  const [showKeys, setShowKeys] = useState<Record<string, boolean>>({});

  /* ================================================================ */
  /*  Queries                                                         */
  /* ================================================================ */
  const {
    data: settingsRaw,
    isLoading: settingsLoading,
    error: settingsError,
  } = useQuery({ queryKey: ['settings'], queryFn: getSettings });

  const {
    data: userRaw,
    isLoading: userLoading,
    error: userError,
  } = useQuery({ queryKey: ['me'], queryFn: getMe });

  const { data: statsRaw } = useQuery({
    queryKey: ['stats-overview'],
    queryFn: getStatsOverview,
  });

  /* ---- derived data ---- */
  const allSettings: Setting[] = Array.isArray(settingsRaw) ? settingsRaw : [];
  const apiKeySettings     = allSettings.filter(s => s.category === 'api_key');
  const notificationSettings = allSettings.filter(s => s.category === 'notification');
  const crawlSettings      = allSettings.filter(s => s.category === 'crawl');

  const user  = userRaw  as User | undefined;
  const stats = statsRaw as StatsOverview | undefined;

  /* ================================================================ */
  /*  Profile form                                                    */
  /* ================================================================ */
  const [profileForm, setProfileForm] = useState({ username: '', email: '' });
  const [profileDirty, setProfileDirty] = useState(false);

  useEffect(() => {
    if (user) {
      setProfileForm({ username: user.username ?? '', email: user.email ?? '' });
      setProfileDirty(false);
    }
  }, [user]);

  const handleProfileChange = (field: 'username' | 'email', value: string) => {
    setProfileForm(prev => ({ ...prev, [field]: value }));
    setProfileDirty(true);
  };

  /* ================================================================ */
  /*  Local setting edits (saved on "保存所有更改")                    */
  /* ================================================================ */
  const [modifiedSettings, setModifiedSettings] = useState<Record<string, any>>({});

  const getVal = (s: Setting) =>
    modifiedSettings[s.key] !== undefined ? modifiedSettings[s.key] : s.value;

  const setVal = (key: string, value: any) =>
    setModifiedSettings(prev => ({ ...prev, [key]: value }));

  /* ================================================================ */
  /*  Mutations                                                       */
  /* ================================================================ */
  const updateSettingMut = useMutation({
    mutationFn: ({ key, value }: { key: string; value: any }) =>
      updateSetting(key, { value }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['settings'] }),
    onError: (e: Error) => showToast(e.message || '保存设置失败', 'error'),
  });

  const updateMeMut = useMutation({
    mutationFn: (data: Partial<User>) => updateMe(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['me'] });
      setProfileDirty(false);
    },
    onError: (e: Error) => showToast(e.message || '更新资料失败', 'error'),
  });

  /* ---- save everything ---- */
  const [saving, setSaving] = useState(false);

  const handleSaveAll = async () => {
    const promises: Promise<any>[] = [];

    for (const [key, value] of Object.entries(modifiedSettings)) {
      promises.push(updateSettingMut.mutateAsync({ key, value }));
    }
    if (profileDirty) {
      promises.push(updateMeMut.mutateAsync(profileForm));
    }

    if (promises.length === 0) {
      showToast('没有需要保存的更改');
      return;
    }

    setSaving(true);
    try {
      await Promise.all(promises);
      setModifiedSettings({});
      showToast('所有更改已保存');
    } catch {
      /* individual mutation onError already fires */
    } finally {
      setSaving(false);
    }
  };

  /* ================================================================ */
  /*  Notification helpers                                            */
  /* ================================================================ */
  const isNotifEnabled = (s: Setting): boolean => {
    const v = getVal(s);
    return typeof v === 'object' && v !== null ? !!v.enabled : !!v;
  };

  const toggleNotif = (s: Setting) => {
    const v = getVal(s);
    if (typeof v === 'object' && v !== null) {
      setVal(s.key, { ...v, enabled: !v.enabled });
    } else {
      setVal(s.key, !v);
    }
  };

  const notifIcon = (key: string) => {
    if (key.includes('webhook'))                return Webhook;
    if (key.includes('ding') || key.includes('dingtalk')) return Bell;
    if (key.includes('email') || key.includes('mail'))    return Mail;
    return Bell;
  };

  const notifName = (s: Setting) => {
    const v = s.value;
    if (typeof v === 'object' && v?.name) return v.name as string;
    return s.description || s.key;
  };

  const notifDetail = (s: Setting) => {
    const v = getVal(s);
    if (typeof v === 'object' && v !== null) {
      const parts: string[] = [];
      if (v.status) parts.push(v.status);
      if (v.detail) parts.push(v.detail);
      if (v.count)  parts.push(String(v.count));
      if (parts.length) return parts.join(' · ');
    }
    return isNotifEnabled(s) ? 'Active' : 'Paused';
  };

  /* ================================================================ */
  /*  Misc helpers                                                    */
  /* ================================================================ */
  const hasChanges = Object.keys(modifiedSettings).length > 0 || profileDirty;
  const isLoading  = settingsLoading || userLoading;
  const loadError  = settingsError || userError;

  const initials = user
    ? (user.username || '').slice(0, 2).toUpperCase() || '??'
    : '??';

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text).then(
      () => showToast('已复制到剪贴板'),
      () => showToast('复制失败', 'error'),
    );
  };

  /* ================================================================ */
  /*  Render                                                          */
  /* ================================================================ */

  /* ---- loading ---- */
  if (isLoading) {
    return (
      <div className="space-y-6">
        <div className="bg-surface-container-lowest rounded-xl p-12 tonal-depth text-center">
          <Loader2 className="w-8 h-8 text-primary animate-spin mx-auto" />
          <p className="text-sm text-on-surface-variant mt-3">加载系统配置...</p>
        </div>
      </div>
    );
  }

  /* ---- error ---- */
  if (loadError) {
    return (
      <div className="space-y-6">
        <div className="bg-surface-container-lowest rounded-xl p-8 tonal-depth text-center">
          <AlertTriangle className="w-10 h-10 text-tertiary mx-auto mb-3" />
          <p className="text-sm font-medium text-on-surface">加载失败</p>
          <p className="text-xs text-on-surface-variant mt-1">
            {(loadError as Error).message}
          </p>
          <button
            onClick={() => {
              queryClient.invalidateQueries({ queryKey: ['settings'] });
              queryClient.invalidateQueries({ queryKey: ['me'] });
            }}
            className="mt-4 px-4 py-2 text-xs font-medium text-primary bg-primary-fixed/20 rounded-lg hover:bg-primary-fixed/30 transition-colors"
          >
            重试
          </button>
        </div>
      </div>
    );
  }

  /* ---- main content ---- */
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
          <h1 className="text-xl font-bold text-on-surface">系统配置</h1>
          <p className="text-sm text-on-surface-variant mt-1">
            管理分布式数据采集集群的全局参数、集成凭据以及通知策略
          </p>
        </div>
        <button
          onClick={handleSaveAll}
          disabled={saving || !hasChanges}
          className="flex items-center gap-1.5 px-4 py-2 text-xs font-medium text-on-primary bg-gradient-to-br from-primary to-primary-container rounded-lg hover:opacity-90 active:scale-95 transition-all tonal-depth disabled:opacity-60"
        >
          {saving ? <Loader2 className="w-4 h-4 animate-spin" /> : <Save className="w-4 h-4" />}
          保存所有更改
        </button>
      </div>

      <div className="grid grid-cols-12 gap-6">
        {/* ============================================================ */}
        {/*  API Keys                                                     */}
        {/* ============================================================ */}
        <div className="col-span-8 bg-surface-container-lowest rounded-xl p-6 tonal-depth">
          <h3 className="text-sm font-semibold text-on-surface mb-4">API 密钥与集成</h3>
          <div className="space-y-4">
            {apiKeySettings.length === 0 && (
              <p className="text-xs text-on-surface-variant py-4 text-center">暂无 API 密钥配置</p>
            )}
            {apiKeySettings.map((s) => {
              const keyStr = typeof getVal(s) === 'string' ? getVal(s) : JSON.stringify(getVal(s));
              return (
                <div
                  key={s.id || s.key}
                  className="flex items-center justify-between p-4 rounded-lg bg-surface-container-low"
                >
                  <div>
                    <p className="text-xs font-medium text-on-surface">
                      {s.description || s.key}
                    </p>
                    <p className="text-[11px] font-mono text-on-surface-variant mt-1">
                      {showKeys[s.key] ? keyStr : '•'.repeat(Math.min(keyStr.length, 32))}
                    </p>
                  </div>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() =>
                        setShowKeys(prev => ({ ...prev, [s.key]: !prev[s.key] }))
                      }
                      className="p-1.5 rounded-lg hover:bg-surface-container-high transition-colors"
                    >
                      {showKeys[s.key] ? (
                        <EyeOff className="w-4 h-4 text-on-surface-variant" />
                      ) : (
                        <Eye className="w-4 h-4 text-on-surface-variant" />
                      )}
                    </button>
                    <button
                      onClick={() => copyToClipboard(keyStr)}
                      className="p-1.5 rounded-lg hover:bg-surface-container-high transition-colors"
                    >
                      <Copy className="w-4 h-4 text-on-surface-variant" />
                    </button>
                    <button className="px-2 py-1 text-[11px] font-medium text-primary bg-primary-fixed/20 rounded-lg hover:bg-primary-fixed/30 transition-colors">
                      重新生成
                    </button>
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        {/* ============================================================ */}
        {/*  Profile                                                      */}
        {/* ============================================================ */}
        <div className="col-span-4 bg-surface-container-lowest rounded-xl p-6 tonal-depth">
          <h3 className="text-sm font-semibold text-on-surface mb-4">个人资料</h3>
          <div className="flex flex-col items-center mb-6">
            <div className="relative group">
              <div className="w-20 h-20 rounded-full bg-gradient-to-br from-primary to-primary-container flex items-center justify-center text-white text-2xl font-bold">
                {initials}
              </div>
              <button className="absolute inset-0 flex items-center justify-center rounded-full bg-black/40 opacity-0 group-hover:opacity-100 transition-opacity">
                <Camera className="w-5 h-5 text-white" />
              </button>
            </div>
            {user?.role && (
              <span className="mt-2 text-[10px] font-medium text-on-surface-variant uppercase tracking-wider">
                {user.role}
              </span>
            )}
          </div>
          <div className="space-y-4">
            <div className="space-y-1.5">
              <label className="text-xs font-medium text-on-surface-variant">显示名称</label>
              <input
                type="text"
                value={profileForm.username}
                onChange={e => handleProfileChange('username', e.target.value)}
                className="w-full px-3 py-2 text-sm bg-surface-container-highest rounded-lg outline-none focus:ring-2 focus:ring-primary/30 transition-all"
              />
            </div>
            <div className="space-y-1.5">
              <label className="text-xs font-medium text-on-surface-variant">电子邮箱</label>
              <input
                type="email"
                value={profileForm.email}
                onChange={e => handleProfileChange('email', e.target.value)}
                className="w-full px-3 py-2 text-sm bg-surface-container-highest rounded-lg outline-none focus:ring-2 focus:ring-primary/30 transition-all"
              />
            </div>
          </div>
        </div>

        {/* ============================================================ */}
        {/*  Crawl Strategy                                               */}
        {/* ============================================================ */}
        <div className="col-span-8 bg-surface-container-lowest rounded-xl p-6 tonal-depth">
          <h3 className="text-sm font-semibold text-on-surface mb-4">全局采集策略</h3>
          <div className="grid grid-cols-3 gap-6">
            {crawlSettings.length === 0 && (
              <p className="col-span-3 text-xs text-on-surface-variant py-4 text-center">
                暂无采集策略配置
              </p>
            )}
            {crawlSettings.map(s => {
              const meta = CRAWL_META[s.key];
              const numVal = Number(getVal(s)) || 0;

              if (meta) {
                return (
                  <div key={s.key} className="space-y-2">
                    <label className="text-xs font-medium text-on-surface-variant">
                      {meta.label}
                    </label>
                    <input
                      type="range"
                      min={meta.min}
                      max={meta.max}
                      step={meta.step}
                      value={numVal}
                      onChange={e => setVal(s.key, Number(e.target.value))}
                      className="w-full accent-primary"
                    />
                    <div className="flex justify-between text-[10px] text-outline">
                      <span>{meta.minLabel}</span>
                      <span className="font-medium text-on-surface">{meta.fmt(numVal)}</span>
                      <span>{meta.maxLabel}</span>
                    </div>
                  </div>
                );
              }

              /* fallback: generic number input for unknown crawl keys */
              return (
                <div key={s.key} className="space-y-2">
                  <label className="text-xs font-medium text-on-surface-variant">
                    {s.description || s.key}
                  </label>
                  <input
                    type="number"
                    value={numVal}
                    onChange={e => setVal(s.key, Number(e.target.value))}
                    className="w-full px-3 py-2 text-sm bg-surface-container-highest rounded-lg outline-none focus:ring-2 focus:ring-primary/30 transition-all"
                  />
                </div>
              );
            })}
          </div>
          <div className="mt-4 p-3 rounded-lg bg-primary-fixed/10">
            <p className="text-[11px] text-primary">
              采集频率过高可能导致代理池快速消耗或触发目标站点的 WAF
              拦截。建议根据目标站点的防护等级合理配置。
            </p>
          </div>
        </div>

        {/* ============================================================ */}
        {/*  Notifications                                                */}
        {/* ============================================================ */}
        <div className="col-span-4 bg-surface-container-lowest rounded-xl p-6 tonal-depth">
          <h3 className="text-sm font-semibold text-on-surface mb-4">通知渠道</h3>
          <div className="space-y-3">
            {notificationSettings.length === 0 && (
              <p className="text-xs text-on-surface-variant py-4 text-center">暂无通知渠道</p>
            )}
            {notificationSettings.map(s => {
              const Icon = notifIcon(s.key);
              const enabled = isNotifEnabled(s);
              return (
                <div
                  key={s.id || s.key}
                  className="flex items-center justify-between p-3 rounded-lg bg-surface-container-low"
                >
                  <div className="flex items-center gap-3">
                    <Icon className="w-4 h-4 text-on-surface-variant" />
                    <div>
                      <p className="text-xs font-medium text-on-surface">{notifName(s)}</p>
                      <p className="text-[10px] text-on-surface-variant">{notifDetail(s)}</p>
                    </div>
                  </div>
                  <label className="relative inline-flex items-center cursor-pointer">
                    <input
                      type="checkbox"
                      checked={enabled}
                      onChange={() => toggleNotif(s)}
                      className="sr-only peer"
                    />
                    <div className="w-8 h-4.5 bg-surface-container-highest rounded-full peer peer-checked:bg-primary peer-checked:after:translate-x-full after:content-[''] after:absolute after:top-0.5 after:left-0.5 after:bg-white after:rounded-full after:h-3.5 after:w-3.5 after:transition-all" />
                  </label>
                </div>
              );
            })}
          </div>
          <button className="w-full mt-3 py-2 text-xs font-medium text-primary bg-primary-fixed/10 rounded-lg hover:bg-primary-fixed/20 transition-colors">
            配置新渠道
          </button>
        </div>
      </div>

      {/* ============================================================== */}
      {/*  System Status                                                  */}
      {/* ============================================================== */}
      <div className="bg-surface-container-lowest rounded-xl p-5 tonal-depth flex items-center justify-between">
        <div>
          <p className="text-xs font-semibold text-on-surface">System Infrastructure</p>
          <p className="text-[11px] text-on-surface-variant mt-0.5">
            {stats
              ? `${stats.nodes.online} / ${stats.nodes.total} nodes online · ${stats.tasks.running} tasks running`
              : '加载系统状态...'}
          </p>
        </div>
        <div className="flex items-center gap-4 text-[11px] text-on-surface-variant">
          <span>
            Nodes Active:{' '}
            <span className="font-medium text-on-surface">
              {stats ? `${stats.nodes.online} / ${stats.nodes.total}` : '—'}
            </span>
          </span>
          <span>
            Tasks Running:{' '}
            <span className="font-medium text-on-surface">
              {stats ? stats.tasks.running : '—'}
            </span>
          </span>
          <span>
            Spiders:{' '}
            <span className="font-medium text-on-surface">
              {stats ? stats.spiders : '—'}
            </span>
          </span>
        </div>
      </div>
    </div>
  );
}
