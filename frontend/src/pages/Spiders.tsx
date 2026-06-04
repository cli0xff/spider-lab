import { useState, useEffect, useRef } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  getSpiders, createSpider, deleteSpider, runSpider,
  getNodes, getSpiderFiles, getSpiderFileContent, saveSpiderFileContent,
  uploadSpiderFile, getSpiderTemplates, updateSpiderConfig,
  searchWeiboUser, searchXueqiuUser, lookupXhsUser,
  lookupXueqiuUser, lookupWeiboUser, getTargetAccounts, searchWechatUser,
} from '../lib/api';
import type { Spider, Node, SpiderTemplate, ConfigField, WeiboUserResult, XueqiuUserResult, XhsUserResult, TargetAccount, WechatUserResult } from '../types';
import {
  Bug, Plus, Play, Trash2, Search, X, Loader2,
  AlertTriangle, Clock, FileCode, Save, Upload,
  Flame, Search as SearchIcon, ChevronRight, LayoutTemplate, Sliders,
  User, CheckCircle,
} from 'lucide-react';

const modeLabel: Record<string, string> = {
  all_nodes: '全部节点',
  selected_nodes: '指定节点',
  random: '随机分配',
};

const platformLabel: Record<string, string> = {
  weibo: '微博',
  xhs: '小红书',
  xueqiu: '雪球',
  wechat: '微信',
  douyin: '抖音',
  bilibili: 'B站',
};

const templateIconMap: Record<string, typeof Flame> = {
  flame: Flame,
  search: SearchIcon,
  user: User,
};

// ===== Config Field Renderer =====

function ConfigFieldInput({
  field,
  value,
  onChange,
}: {
  field: ConfigField;
  value: any;
  onChange: (val: any) => void;
}) {
  const v = value ?? field.default ?? '';

  switch (field.type) {
    case 'boolean':
      return (
        <label className="flex items-center gap-2.5 cursor-pointer">
          <div
            onClick={() => onChange(!v)}
            className={`relative w-9 h-5 rounded-full transition-colors ${v ? 'bg-primary' : 'bg-surface-container-highest'}`}
          >
            <div className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white transition-transform ${v ? 'translate-x-4' : ''}`} />
          </div>
          <span className="text-xs text-on-surface">{v ? '开启' : '关闭'}</span>
        </label>
      );

    case 'select':
      return (
        <select
          value={v}
          onChange={(e) => onChange(e.target.value)}
          className="w-full px-3 py-2.5 bg-surface-container-highest rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary/30 transition-all"
        >
          {(field.options || []).map((opt) => (
            <option key={opt} value={opt}>{opt}</option>
          ))}
        </select>
      );

    case 'number':
      return (
        <input
          type="number"
          value={v}
          min={field.min}
          max={field.max}
          onChange={(e) => onChange(Number(e.target.value))}
          placeholder={field.placeholder}
          className="w-full px-3 py-2.5 bg-surface-container-highest rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary/30 transition-all placeholder:text-outline-variant"
        />
      );

    default:
      return (
        <input
          type="text"
          value={v}
          onChange={(e) => onChange(e.target.value)}
          placeholder={field.placeholder}
          className="w-full px-3 py-2.5 bg-surface-container-highest rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary/30 transition-all placeholder:text-outline-variant"
        />
      );
  }
}

// ===== Weibo User Search Widget =====

function WeiboUserSearch({
  value,
  onChange,
}: {
  value: string;
  onChange: (uid: string, meta?: { screen_name?: string }) => void;
}) {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<WeiboUserResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [showDropdown, setShowDropdown] = useState(false);
  const [selectedUser, setSelectedUser] = useState<WeiboUserResult | null>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const composingRef = useRef(false);

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as globalThis.Node)) {
        setShowDropdown(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const doSearch = (q: string) => {
    if (!q.trim()) {
      setResults([]);
      setShowDropdown(false);
      return;
    }
    setLoading(true);
    searchWeiboUser(q.trim())
      .then((data) => {
        const list = Array.isArray(data) ? data : [];
        setResults(list);
        setShowDropdown(list.length > 0);
      })
      .catch((err) => { console.warn('Weibo user search failed:', err); setResults([]); })
      .finally(() => setLoading(false));
  };

  const handleInputChange = (q: string) => {
    setQuery(q);
    if (debounceRef.current) clearTimeout(debounceRef.current);
    if (!composingRef.current) {
      debounceRef.current = setTimeout(() => doSearch(q), 500);
    }
  };

  const handleSelect = (u: WeiboUserResult) => {
    setSelectedUser(u);
    onChange(u.uid, { screen_name: u.screen_name });
    setShowDropdown(false);
    setQuery('');
  };

  const handleClear = () => {
    setSelectedUser(null);
    onChange('');
  };

  const formatFollowers = (n: number) => {
    if (n >= 10000) return (n / 10000).toFixed(1) + '万';
    return String(n);
  };

  return (
    <div className="space-y-2">
      {/* Manual UID input */}
      <div className="flex gap-2">
        <input
          type="text"
          value={value}
          onChange={(e) => { onChange(e.target.value); if (selectedUser && e.target.value !== selectedUser.uid) setSelectedUser(null); }}
          placeholder="输入用户UID，或通过下方搜索"
          className="flex-1 px-3 py-2.5 bg-surface-container-highest rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary/30 transition-all placeholder:text-outline-variant"
        />
      </div>

      {/* Selected user display */}
      {selectedUser && (
        <div className="flex items-center gap-3 p-2.5 rounded-lg bg-primary-fixed/10">
          {selectedUser.avatar_url ? (
            <img src={selectedUser.avatar_url} alt="" className="w-8 h-8 rounded-full object-cover shrink-0" />
          ) : (
            <div className="w-8 h-8 rounded-full bg-surface-container-highest flex items-center justify-center shrink-0">
              <User className="w-4 h-4 text-outline" />
            </div>
          )}
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-1.5">
              <span className="text-xs font-semibold text-on-surface truncate">{selectedUser.screen_name}</span>
              {selectedUser.verified && <CheckCircle className="w-3 h-3 text-primary shrink-0" />}
            </div>
            <span className="text-[10px] text-on-surface-variant">UID: {selectedUser.uid}</span>
          </div>
          <button type="button" onClick={handleClear} className="p-1 hover:bg-surface-container-high rounded transition-colors">
            <X className="w-3.5 h-3.5 text-outline" />
          </button>
        </div>
      )}

      {/* Search bar */}
      <div ref={containerRef} className="relative">
        <div className="flex items-center gap-2">
          <div className="relative flex-1">
            <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-outline" />
            <input
              type="text"
              value={query}
              onChange={(e) => handleInputChange(e.target.value)}
              onCompositionStart={() => { composingRef.current = true; }}
              onCompositionEnd={(e) => { composingRef.current = false; handleInputChange((e.target as HTMLInputElement).value); }}
              onFocus={() => { if (results.length > 0) setShowDropdown(true); }}
              placeholder="搜索微博用户昵称..."
              className="w-full pl-8 pr-3 py-2 bg-surface-container-high rounded-lg text-xs outline-none focus:ring-2 focus:ring-primary/30 transition-all placeholder:text-outline-variant"
            />
            {loading && <Loader2 className="absolute right-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-outline animate-spin" />}
          </div>
        </div>

        {/* Results dropdown */}
        {showDropdown && results.length > 0 && (
          <div className="absolute z-20 left-0 right-0 mt-1 bg-surface-container-lowest rounded-xl shadow-editorial max-h-64 overflow-y-auto">
            {results.map((u) => (
              <button
                key={u.uid}
                type="button"
                onClick={() => handleSelect(u)}
                className="w-full flex items-center gap-3 px-3 py-2.5 hover:bg-surface-container-low transition-colors text-left"
              >
                {u.avatar_url ? (
                  <img src={u.avatar_url} alt="" className="w-8 h-8 rounded-full object-cover shrink-0" />
                ) : (
                  <div className="w-8 h-8 rounded-full bg-surface-container-highest flex items-center justify-center shrink-0">
                    <User className="w-4 h-4 text-outline" />
                  </div>
                )}
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-1.5">
                    <span className="text-xs font-medium text-on-surface truncate">{u.screen_name}</span>
                    {u.verified && <CheckCircle className="w-3 h-3 text-primary shrink-0" />}
                    <span className="text-[10px] text-outline ml-auto shrink-0">{formatFollowers(u.followers_count)} 粉丝</span>
                  </div>
                  <p className="text-[10px] text-on-surface-variant truncate mt-0.5">
                    {u.verified_reason || u.description || `UID: ${u.uid}`}
                  </p>
                </div>
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

// ===== Xueqiu User Search Widget =====

function XueqiuUserSearch({
  value,
  onChange,
}: {
  value: string;
  onChange: (id: string, meta?: { screen_name?: string }) => void;
}) {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<XueqiuUserResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [showDropdown, setShowDropdown] = useState(false);
  const [selectedUser, setSelectedUser] = useState<XueqiuUserResult | null>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const composingRef = useRef(false);

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as globalThis.Node)) {
        setShowDropdown(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const doSearch = (q: string) => {
    if (!q.trim()) { setResults([]); setShowDropdown(false); return; }
    setLoading(true);
    searchXueqiuUser(q.trim())
      .then((data) => {
        const list = Array.isArray(data) ? data : [];
        setResults(list);
        setShowDropdown(list.length > 0);
      })
      .catch((err) => { console.warn('Xueqiu user search failed:', err); setResults([]); })
      .finally(() => setLoading(false));
  };

  const handleInputChange = (q: string) => {
    setQuery(q);
    if (debounceRef.current) clearTimeout(debounceRef.current);
    if (!composingRef.current) {
      debounceRef.current = setTimeout(() => doSearch(q), 500);
    }
  };

  const handleSelect = (u: XueqiuUserResult) => {
    setSelectedUser(u);
    onChange(u.id, { screen_name: u.screen_name });
    setShowDropdown(false);
    setQuery('');
  };

  const handleClear = () => {
    setSelectedUser(null);
    onChange('');
  };

  const formatFollowers = (n: number) => {
    if (n >= 10000) return (n / 10000).toFixed(1) + '万';
    return String(n);
  };

  return (
    <div className="space-y-2">
      <div className="flex gap-2">
        <input
          type="text"
          value={value}
          onChange={(e) => { onChange(e.target.value); if (selectedUser && e.target.value !== selectedUser.id) setSelectedUser(null); }}
          placeholder="输入雪球用户ID，或通过下方搜索"
          className="flex-1 px-3 py-2.5 bg-surface-container-highest rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary/30 transition-all placeholder:text-outline-variant"
        />
      </div>

      {selectedUser && (
        <div className="flex items-center gap-3 p-2.5 rounded-lg bg-primary-fixed/10">
          {selectedUser.avatar_url ? (
            <img src={selectedUser.avatar_url} alt="" className="w-8 h-8 rounded-full object-cover shrink-0" />
          ) : (
            <div className="w-8 h-8 rounded-full bg-surface-container-highest flex items-center justify-center shrink-0">
              <User className="w-4 h-4 text-outline" />
            </div>
          )}
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-1.5">
              <span className="text-xs font-semibold text-on-surface truncate">{selectedUser.screen_name}</span>
              {selectedUser.verified && <CheckCircle className="w-3 h-3 text-primary shrink-0" />}
            </div>
            <span className="text-[10px] text-on-surface-variant">ID: {selectedUser.id}</span>
          </div>
          <button type="button" onClick={handleClear} className="p-1 hover:bg-surface-container-high rounded transition-colors">
            <X className="w-3.5 h-3.5 text-outline" />
          </button>
        </div>
      )}

      <div ref={containerRef} className="relative">
        <div className="flex items-center gap-2">
          <div className="relative flex-1">
            <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-outline" />
            <input
              type="text"
              value={query}
              onChange={(e) => handleInputChange(e.target.value)}
              onCompositionStart={() => { composingRef.current = true; }}
              onCompositionEnd={(e) => { composingRef.current = false; handleInputChange((e.target as HTMLInputElement).value); }}
              onFocus={() => { if (results.length > 0) setShowDropdown(true); }}
              placeholder="搜索雪球用户昵称..."
              className="w-full pl-8 pr-3 py-2 bg-surface-container-high rounded-lg text-xs outline-none focus:ring-2 focus:ring-primary/30 transition-all placeholder:text-outline-variant"
            />
            {loading && <Loader2 className="absolute right-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-outline animate-spin" />}
          </div>
        </div>

        {showDropdown && results.length > 0 && (
          <div className="absolute z-20 left-0 right-0 mt-1 bg-surface-container-lowest rounded-xl shadow-editorial max-h-64 overflow-y-auto">
            {results.map((u) => (
              <button
                key={u.id}
                type="button"
                onClick={() => handleSelect(u)}
                className="w-full flex items-center gap-3 px-3 py-2.5 hover:bg-surface-container-low transition-colors text-left"
              >
                {u.avatar_url ? (
                  <img src={u.avatar_url} alt="" className="w-8 h-8 rounded-full object-cover shrink-0" />
                ) : (
                  <div className="w-8 h-8 rounded-full bg-surface-container-highest flex items-center justify-center shrink-0">
                    <User className="w-4 h-4 text-outline" />
                  </div>
                )}
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-1.5">
                    <span className="text-xs font-medium text-on-surface truncate">{u.screen_name}</span>
                    {u.verified && <CheckCircle className="w-3 h-3 text-primary shrink-0" />}
                    <span className="text-[10px] text-outline ml-auto shrink-0">{formatFollowers(u.followers_count)} 粉丝</span>
                  </div>
                  <p className="text-[10px] text-on-surface-variant truncate mt-0.5">
                    {u.verified_description || u.description || `ID: ${u.id}`}
                  </p>
                </div>
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

// ===== XHS User Lookup Widget =====

function XhsUserSearch({
  value,
  onChange,
}: {
  value: string;
  onChange: (id: string, meta?: { screen_name?: string }) => void;
}) {
  const [lookupResult, setLookupResult] = useState<XhsUserResult | null>(null);
  const [loading, setLoading] = useState(false);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  const doLookup = (id: string) => {
    if (!id || id.length < 10) {
      setLookupResult(null);
      return;
    }
    setLoading(true);
    lookupXhsUser(id)
      .then((data) => {
        if (data && data.nickname) {
          setLookupResult(data);
          onChange(id, { screen_name: data.nickname });
        } else {
          setLookupResult(null);
        }
      })
      .catch((err) => { console.warn('XHS user lookup failed:', err); setLookupResult(null); })
      .finally(() => setLoading(false));
  };

  const handleChange = (id: string) => {
    onChange(id);
    setLookupResult(null);
    if (debounceRef.current) clearTimeout(debounceRef.current);
    if (id.length >= 10) {
      debounceRef.current = setTimeout(() => doLookup(id), 800);
    }
  };

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-2">
        <input
          type="text"
          value={value}
          onChange={(e) => handleChange(e.target.value)}
          placeholder="输入小红书用户ID（如 5fa7492e00000000010049ef）"
          className="flex-1 px-3 py-2.5 bg-surface-container-highest rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary/30 transition-all placeholder:text-outline-variant"
        />
        {loading && <Loader2 className="w-4 h-4 text-outline animate-spin shrink-0" />}
      </div>

      {lookupResult && lookupResult.nickname && (
        <div className="flex items-center gap-3 p-2.5 rounded-lg bg-primary-fixed/10">
          {lookupResult.avatar ? (
            <img src={lookupResult.avatar} alt="" className="w-8 h-8 rounded-full object-cover shrink-0" />
          ) : (
            <div className="w-8 h-8 rounded-full bg-surface-container-highest flex items-center justify-center shrink-0">
              <User className="w-4 h-4 text-outline" />
            </div>
          )}
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-1.5">
              <span className="text-xs font-semibold text-on-surface truncate">{lookupResult.nickname}</span>
              <CheckCircle className="w-3 h-3 text-primary shrink-0" />
            </div>
            <span className="text-[10px] text-on-surface-variant">
              {lookupResult.red_id ? `小红书号: ${lookupResult.red_id}` : `ID: ${lookupResult.user_id}`}
            </span>
          </div>
          {lookupResult.desc && (
            <span className="text-[10px] text-on-surface-variant max-w-[40%] truncate">{lookupResult.desc}</span>
          )}
        </div>
      )}
    </div>
  );
}

// ===== WeChat User Search Widget =====

function WechatUserSearch({
  value,
  onChange,
}: {
  value: string;
  onChange: (id: string, meta?: { nickname?: string }) => void;
}) {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<WechatUserResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [showDropdown, setShowDropdown] = useState(false);
  const [selectedUser, setSelectedUser] = useState<WechatUserResult | null>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const composingRef = useRef(false);

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as globalThis.Node)) {
        setShowDropdown(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const doSearch = (q: string) => {
    if (!q.trim()) {
      setResults([]);
      setShowDropdown(false);
      return;
    }
    setLoading(true);
    searchWechatUser(q.trim())
      .then((data) => {
        const list = Array.isArray(data) ? data : [];
        setResults(list);
        setShowDropdown(list.length > 0);
      })
      .catch((err) => { console.warn('WeChat user search failed:', err); setResults([]); })
      .finally(() => setLoading(false));
  };

  const handleInputChange = (q: string) => {
    setQuery(q);
    if (debounceRef.current) clearTimeout(debounceRef.current);
    if (!composingRef.current) {
      debounceRef.current = setTimeout(() => doSearch(q), 500);
    }
  };

  const handleSelect = (u: WechatUserResult) => {
    setSelectedUser(u);
    onChange(u.id, { nickname: u.nickname });
    setShowDropdown(false);
    setQuery('');
  };

  const handleClear = () => {
    setSelectedUser(null);
    onChange('');
  };

  return (
    <div className="space-y-2">
      {/* Manual ID input */}
      <div className="flex gap-2">
        <input
          type="text"
          value={value}
          onChange={(e) => { onChange(e.target.value); if (selectedUser && e.target.value !== selectedUser.id) setSelectedUser(null); }}
          placeholder="输入公众号fakeid，或通过下方搜索"
          className="flex-1 px-3 py-2.5 bg-surface-container-highest rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary/30 transition-all placeholder:text-outline-variant"
        />
      </div>

      {/* Selected user display */}
      {selectedUser && (
        <div className="flex items-center gap-3 p-2.5 rounded-lg bg-primary-fixed/10">
          <div className="w-8 h-8 rounded-full bg-surface-container-highest flex items-center justify-center shrink-0">
            <User className="w-4 h-4 text-outline" />
          </div>
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-1.5">
              <span className="text-xs font-semibold text-on-surface truncate">{selectedUser.nickname}</span>
              {selectedUser.verified && <CheckCircle className="w-3 h-3 text-primary shrink-0" />}
            </div>
            <span className="text-[10px] text-on-surface-variant">ID: {selectedUser.id}</span>
          </div>
          <button type="button" onClick={handleClear} className="p-1 hover:bg-surface-container-high rounded transition-colors">
            <X className="w-3.5 h-3.5 text-outline" />
          </button>
        </div>
      )}

      {/* Search bar */}
      <div ref={containerRef} className="relative">
        <div className="flex items-center gap-2">
          <div className="relative flex-1">
            <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-outline" />
            <input
              type="text"
              value={query}
              onChange={(e) => handleInputChange(e.target.value)}
              onCompositionStart={() => { composingRef.current = true; }}
              onCompositionEnd={(e) => { composingRef.current = false; handleInputChange((e.target as HTMLInputElement).value); }}
              onFocus={() => { if (results.length > 0) setShowDropdown(true); }}
              placeholder="搜索微信公众号名称..."
              className="w-full pl-8 pr-3 py-2 bg-surface-container-high rounded-lg text-xs outline-none focus:ring-2 focus:ring-primary/30 transition-all placeholder:text-outline-variant"
            />
            {loading && <Loader2 className="absolute right-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-outline animate-spin" />}
          </div>
        </div>

        {/* Results dropdown */}
        {showDropdown && results.length > 0 && (
          <div className="absolute z-20 left-0 right-0 mt-1 bg-surface-container-lowest rounded-xl shadow-editorial max-h-64 overflow-y-auto">
            {results.map((u) => (
              <button
                key={u.id}
                type="button"
                onClick={() => handleSelect(u)}
                className="w-full flex items-center gap-3 px-3 py-2.5 hover:bg-surface-container-low transition-colors text-left"
              >
                <div className="w-8 h-8 rounded-full bg-surface-container-highest flex items-center justify-center shrink-0">
                  <User className="w-4 h-4 text-outline" />
                </div>
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-1.5">
                    <span className="text-xs font-medium text-on-surface truncate">{u.nickname}</span>
                    {u.verified && <CheckCircle className="w-3 h-3 text-primary shrink-0" />}
                  </div>
                  <p className="text-[10px] text-on-surface-variant truncate mt-0.5">
                    ID: {u.id}
                  </p>
                </div>
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

// ===== Target Account Multi-Select Picker =====

function TargetAccountPicker({
  platform,
  selected,
  onChange,
}: {
  platform: 'xueqiu' | 'weibo' | 'xhs' | 'wechat';
  selected: TargetAccount[];
  onChange: (accounts: TargetAccount[]) => void;
}) {
  const { data: accounts, isLoading } = useQuery({
    queryKey: ['target-accounts', platform],
    queryFn: () => getTargetAccounts(platform),
  });

  const toggle = (account: TargetAccount) => {
    const isSelected = selected.some(a => a.user_id === account.user_id);
    if (isSelected) {
      onChange(selected.filter(a => a.user_id !== account.user_id));
    } else {
      onChange([...selected, account]);
    }
  };

  const selectAll = () => {
    if (!accounts) return;
    onChange(selected.length === accounts.length ? [] : [...accounts]);
  };

  const formatFollowers = (n: number) => {
    if (n >= 10000) return (n / 10000).toFixed(1) + '万';
    return String(n);
  };

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 py-3 text-xs text-on-surface-variant">
        <Loader2 className="w-3.5 h-3.5 animate-spin" />加载目标账号...
      </div>
    );
  }

  if (!accounts || accounts.length === 0) {
    return (
      <div className="p-4 rounded-lg bg-surface-container-high text-center">
        <User className="w-5 h-5 text-outline mx-auto mb-1.5" />
        <p className="text-xs text-on-surface-variant">暂无{platformLabel[platform]}目标账号</p>
        <p className="text-[10px] text-outline mt-1">请先在「账号管理」页面添加目标账号</p>
      </div>
    );
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <button type="button" onClick={selectAll} className="text-[10px] text-primary hover:underline">
          {selected.length === accounts.length ? '取消全选' : '全选'}
        </button>
        <span className="text-[10px] text-on-surface-variant">
          已选 {selected.length}/{accounts.length}
        </span>
      </div>
      <div className="max-h-48 overflow-y-auto rounded-lg bg-surface-container-highest">
        {accounts.map((account) => {
          const isSelected = selected.some(a => a.user_id === account.user_id);
          return (
            <label
              key={account.user_id}
              className={`flex items-center gap-3 px-3 py-2.5 cursor-pointer hover:bg-surface-container-low transition-colors ${isSelected ? 'bg-primary-fixed/10' : ''}`}
            >
              <input
                type="checkbox"
                checked={isSelected}
                onChange={() => toggle(account)}
                className="w-3.5 h-3.5 rounded accent-primary shrink-0"
              />
              {account.avatar_url ? (
                <img src={account.avatar_url} alt="" className="w-7 h-7 rounded-full object-cover shrink-0" />
              ) : (
                <div className="w-7 h-7 rounded-full bg-surface-container flex items-center justify-center shrink-0">
                  <User className="w-3.5 h-3.5 text-outline" />
                </div>
              )}
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-1.5">
                  <span className="text-xs font-medium text-on-surface truncate">{account.nickname}</span>
                  {account.verified && <CheckCircle className="w-3 h-3 text-primary shrink-0" />}
                  {account.followers_count > 0 && (
                    <span className="text-[10px] text-outline ml-auto shrink-0">{formatFollowers(account.followers_count)} 粉丝</span>
                  )}
                </div>
                <span className="text-[10px] text-on-surface-variant">ID: {account.user_id}</span>
              </div>
            </label>
          );
        })}
      </div>
    </div>
  );
}

// ===== Main Component =====

export default function Spiders() {
  const queryClient = useQueryClient();
  const [search, setSearch] = useState('');
  const [typeFilter, setTypeFilter] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const [deletingSpider, setDeletingSpider] = useState<Spider | null>(null);
  const [runningSpider, setRunningSpider] = useState<Spider | null>(null);
  const [runParam, setRunParam] = useState('');
  const [configSpider, setConfigSpider] = useState<Spider | null>(null);
  const [configValues, setConfigValues] = useState<Record<string, any>>({});
  const [toast, setToast] = useState<{ msg: string; type: 'success' | 'error' } | null>(null);

  // Create form state
  const [createStep, setCreateStep] = useState<'pick' | 'config'>('pick');
  const [selectedTemplate, setSelectedTemplate] = useState<SpiderTemplate | null>(null);
  const [createName, setCreateName] = useState('');
  const [createMode, setCreateMode] = useState('all_nodes');
  const [createPriority, setCreatePriority] = useState(5);
  const [createNodeIds, setCreateNodeIds] = useState<string[]>([]);
  const [createConfig, setCreateConfig] = useState<Record<string, any>>({});
  const [createProject, setCreateProject] = useState('');
  const [createTags, setCreateTags] = useState<string[]>([]);
  const [tagInput, setTagInput] = useState('');
  const [selectedTargetAccounts, setSelectedTargetAccounts] = useState<TargetAccount[]>([]);
  const [batchCreating, setBatchCreating] = useState(false);

  // File management state
  const [filesSpider, setFilesSpider] = useState<Spider | null>(null);
  const [selectedFile, setSelectedFile] = useState<string | null>(null);
  const [fileContent, setFileContent] = useState('');
  const [fileDirty, setFileDirty] = useState(false);
  const [fileSaving, setFileSaving] = useState(false);

  const showToast = (msg: string, type: 'success' | 'error' = 'success') => {
    setToast({ msg, type });
    setTimeout(() => setToast(null), 3000);
  };

  // Queries
  const { data, isLoading, error } = useQuery({
    queryKey: ['spiders', search, typeFilter],
    queryFn: () => getSpiders({ q: search || undefined, type: typeFilter || undefined, page: 1, size: 100 }),
  });
  const spiders: Spider[] = data?.data ?? [];

  const { data: nodesData } = useQuery({
    queryKey: ['nodes'],
    queryFn: () => getNodes({ page: 1, size: 200 }),
  });
  const nodes: Node[] = (nodesData as any)?.data ?? [];

  const { data: templates, isLoading: templatesLoading } = useQuery({
    queryKey: ['spider-templates'],
    queryFn: getSpiderTemplates,
  });

  // Auto-resolve missing screen_names for user-specific spiders
  const resolvingRef = useRef<Set<string>>(new Set());
  useEffect(() => {
    if (!spiders.length) return;
    const toResolve = spiders.filter((sp) => {
      if (resolvingRef.current.has(sp._id)) return false;
      if (sp.template_id === 'xueqiu_user_posts' && sp.config?.user_id && !sp.config?.xueqiu_screen_name) return true;
      if (sp.template_id === 'weibo_user_posts' && sp.config?.user_id && !sp.config?.weibo_screen_name) return true;
      return false;
    });
    if (!toResolve.length) return;

    for (const sp of toResolve) {
      resolvingRef.current.add(sp._id);
      const resolve = async () => {
        try {
          let screenName = '';
          if (sp.template_id === 'xueqiu_user_posts') {
            const info = await lookupXueqiuUser(sp.config!.user_id);
            screenName = info?.screen_name || '';
          } else if (sp.template_id === 'weibo_user_posts') {
            const info = await lookupWeiboUser(sp.config!.user_id);
            screenName = info?.screen_name || '';
          }
          if (screenName) {
            const key = sp.template_id === 'xueqiu_user_posts' ? 'xueqiu_screen_name' : 'weibo_screen_name';
            await updateSpiderConfig(sp._id, { ...sp.config, [key]: screenName });
            queryClient.invalidateQueries({ queryKey: ['spiders'] });
          }
        } catch {
          // Silently ignore — will retry on next page load
        }
      };
      resolve();
    }
  }, [spiders, queryClient]);

  // Mutations
  const createMut = useMutation({
    mutationFn: (data: Partial<Spider>) => createSpider(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['spiders'] });
      resetCreate();
      showToast('爬虫创建成功');
    },
    onError: (e: Error) => showToast(e.message || '创建失败', 'error'),
  });

  const deleteMut = useMutation({
    mutationFn: (id: string) => deleteSpider(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['spiders'] });
      setDeletingSpider(null);
      showToast('爬虫已删除');
    },
    onError: (e: Error) => showToast(e.message || '删除失败', 'error'),
  });

  const runMut = useMutation({
    mutationFn: ({ id, param }: { id: string; param?: string }) => runSpider(id, param),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['spiders'] });
      setRunningSpider(null);
      setRunParam('');
      showToast('任务已创建并开始执行');
    },
    onError: (e: Error) => showToast(e.message || '运行失败', 'error'),
  });

  const configMut = useMutation({
    mutationFn: ({ id, config }: { id: string; config: Record<string, any> }) => updateSpiderConfig(id, config),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['spiders'] });
      setConfigSpider(null);
      showToast('配置已保存');
    },
    onError: (e: Error) => showToast(e.message || '保存配置失败', 'error'),
  });

  const resetCreate = () => {
    setShowCreate(false);
    setCreateStep('pick');
    setSelectedTemplate(null);
    setCreateName('');
    setCreateMode('all_nodes');
    setCreatePriority(5);
    setCreateNodeIds([]);
    setCreateConfig({});
    setCreateProject('');
    setCreateTags([]);
    setTagInput('');
    setSelectedTargetAccounts([]);
    setBatchCreating(false);
  };

  const handleCreateFromTemplate = async () => {
    if (!selectedTemplate) return;
    if (createMode === 'selected_nodes' && createNodeIds.length === 0) return showToast('请选择至少一个节点', 'error');

    const isUserPostsTemplate = ['weibo_user_posts', 'xueqiu_user_posts', 'xhs_user_posts'].includes(selectedTemplate.id);

    // Batch creation: multiple target accounts selected
    if (isUserPostsTemplate && selectedTargetAccounts.length > 0) {
      // Build base config (excluding user_id which varies per account)
      const baseConfig: Record<string, any> = {};
      for (const field of selectedTemplate.config_fields) {
        if (field.key === 'user_id') continue;
        const val = createConfig[field.key] ?? field.default;
        if (field.required && (val === '' || val === null || val === undefined)) {
          return showToast(`请填写必填参数: ${field.label}`, 'error');
        }
        baseConfig[field.key] = val;
      }

      const platformSuffix: Record<string, string> = {
        weibo_user_posts: '微博', xueqiu_user_posts: '雪球', xhs_user_posts: '小红书',
      };
      const screenNameKey: Record<string, string> = {
        weibo_user_posts: 'weibo_screen_name', xueqiu_user_posts: 'xueqiu_screen_name', xhs_user_posts: 'xhs_screen_name',
      };
      const suffix = platformSuffix[selectedTemplate.id] ?? '';
      const metaKey = screenNameKey[selectedTemplate.id] ?? '';

      setBatchCreating(true);
      let successCount = 0;
      let failCount = 0;

      for (const account of selectedTargetAccounts) {
        try {
          await createSpider({
            name: `${account.nickname}的${suffix}`,
            type: 'template',
            template_id: selectedTemplate.id,
            config: { ...baseConfig, user_id: account.user_id, [metaKey]: account.nickname },
            mode: createMode,
            priority: createPriority,
            node_ids: createNodeIds,
            project: createProject,
            tags: createTags,
            cmd: selectedTemplate.cmd,
          } as unknown as Partial<Spider>);
          successCount++;
        } catch {
          failCount++;
        }
      }

      queryClient.invalidateQueries({ queryKey: ['spiders'] });
      resetCreate();

      if (failCount === 0) {
        showToast(`成功创建 ${successCount} 个爬虫`);
      } else {
        showToast(`创建完成：${successCount} 成功，${failCount} 失败`, 'error');
      }
      return;
    }

    // Single creation (original flow)
    if (!createName.trim()) return showToast('请输入爬虫名称', 'error');

    // Fill defaults for any missing config values
    const finalConfig: Record<string, any> = {};
    for (const field of selectedTemplate.config_fields) {
      const val = createConfig[field.key] ?? field.default;
      if (field.required && (val === '' || val === null || val === undefined)) {
        return showToast(`请填写必填参数: ${field.label}`, 'error');
      }
      finalConfig[field.key] = val;
    }
    // Preserve screen_name metadata from user search widgets
    for (const metaKey of ['weibo_screen_name', 'xueqiu_screen_name', 'xhs_screen_name']) {
      if (createConfig[metaKey]) finalConfig[metaKey] = createConfig[metaKey];
    }

    createMut.mutate({
      name: createName,
      type: 'template',
      template_id: selectedTemplate.id,
      config: finalConfig,
      mode: createMode,
      priority: createPriority,
      node_ids: createNodeIds,
      project: createProject,
      tags: createTags,
      cmd: selectedTemplate.cmd,
    } as unknown as Partial<Spider>);
  };

  const openConfig = (sp: Spider) => {
    setConfigSpider(sp);
    setConfigValues(sp.config ? { ...sp.config } : {});
  };

  const openRun = (sp: Spider) => {
    setRunParam(sp.param || '');
    setRunningSpider(sp);
  };

  // File management
  const { data: filesData, isLoading: filesLoading } = useQuery({
    queryKey: ['spider-files', filesSpider?._id],
    queryFn: () => getSpiderFiles(filesSpider!._id),
    enabled: !!filesSpider,
  });
  const files = (filesData as any) ?? [];

  const openFiles = (sp: Spider) => {
    setFilesSpider(sp);
    setSelectedFile(null);
    setFileContent('');
    setFileDirty(false);
  };

  const selectFile = async (path: string) => {
    if (fileDirty && !confirm('当前文件有未保存的更改，确定切换？')) return;
    try {
      const result = await getSpiderFileContent(filesSpider!._id, path);
      setSelectedFile(path);
      setFileContent((result as any)?.content ?? result ?? '');
      setFileDirty(false);
    } catch {
      showToast('读取文件失败', 'error');
    }
  };

  const handleSaveFile = async () => {
    if (!filesSpider || !selectedFile) return;
    setFileSaving(true);
    try {
      await saveSpiderFileContent(filesSpider._id, selectedFile, fileContent);
      setFileDirty(false);
      showToast('文件已保存');
    } catch {
      showToast('保存失败', 'error');
    } finally {
      setFileSaving(false);
    }
  };

  const handleUploadFile = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file || !filesSpider) return;
    try {
      await uploadSpiderFile(filesSpider._id, file);
      queryClient.invalidateQueries({ queryKey: ['spider-files', filesSpider._id] });
      showToast(`文件 ${file.name} 已上传`);
    } catch {
      showToast('上传失败', 'error');
    }
    e.target.value = '';
  };

  // Find template for a spider
  const getTemplateForSpider = (sp: Spider): SpiderTemplate | undefined => {
    if (sp.type !== 'template' || !sp.template_id) return undefined;
    return (templates as any as SpiderTemplate[] | undefined)?.find((t) => t.id === sp.template_id);
  };

  const getSpiderIcon = (sp: Spider) => {
    const tmpl = getTemplateForSpider(sp);
    if (tmpl) {
      const Icon = templateIconMap[tmpl.icon] || Flame;
      return <Icon className="w-5 h-5 text-primary" />;
    }
    return <Bug className="w-5 h-5 text-primary" />;
  };

  const getSpiderTypeLabel = (sp: Spider) => {
    const tmpl = getTemplateForSpider(sp);
    if (tmpl) return tmpl.name;
    return '自定义';
  };

  const templatesList = (templates as any as SpiderTemplate[] | undefined) ?? [];

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
          <h1 className="text-xl font-bold text-on-surface">采集爬虫管理</h1>
          <p className="text-sm text-on-surface-variant mt-1">
            管理和配置您的数据采集爬虫程序
            {spiders.length > 0 && <span className="ml-2 text-outline">({spiders.length} 个爬虫)</span>}
          </p>
        </div>
        <button
          onClick={() => { resetCreate(); setShowCreate(true); }}
          className="flex items-center gap-1.5 px-4 py-2 text-xs font-medium text-on-primary bg-gradient-to-br from-primary to-primary-container rounded-lg hover:opacity-90 active:scale-95 transition-all tonal-depth"
        >
          <Plus className="w-4 h-4" />
          新建爬虫
        </button>
      </div>

      {/* Search & Filter */}
      <div className="flex items-center gap-3">
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-outline" />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="搜索爬虫名称..."
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
            { value: 'template', label: '模板爬虫' },
            { value: 'custom', label: '自定义' },
          ].map((f) => (
            <button
              key={f.value}
              onClick={() => setTypeFilter(f.value)}
              className={`px-3 py-1.5 text-xs rounded-lg transition-colors ${
                typeFilter === f.value
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
        <div className="grid grid-cols-2 xl:grid-cols-3 gap-4">
          {[1, 2, 3].map((i) => (
            <div key={i} className="bg-surface-container-lowest rounded-xl p-5 tonal-depth animate-pulse">
              <div className="flex items-center gap-3 mb-4">
                <div className="w-10 h-10 rounded-lg bg-surface-container-high" />
                <div className="space-y-2 flex-1">
                  <div className="h-3 bg-surface-container-high rounded w-2/3" />
                  <div className="h-2.5 bg-surface-container-high rounded w-1/2" />
                </div>
              </div>
              <div className="space-y-2.5">
                <div className="h-2.5 bg-surface-container-high rounded" />
                <div className="h-2.5 bg-surface-container-high rounded w-3/4" />
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Error */}
      {error && !isLoading && (
        <div className="bg-surface-container-lowest rounded-xl p-8 tonal-depth text-center">
          <AlertTriangle className="w-10 h-10 text-tertiary mx-auto mb-3" />
          <p className="text-sm font-medium text-on-surface">加载失败</p>
          <p className="text-xs text-on-surface-variant mt-1">{(error as Error).message}</p>
          <button
            onClick={() => queryClient.invalidateQueries({ queryKey: ['spiders'] })}
            className="mt-4 px-4 py-2 text-xs font-medium text-primary bg-primary-fixed/20 rounded-lg hover:bg-primary-fixed/30 transition-colors"
          >
            重试
          </button>
        </div>
      )}

      {/* Empty */}
      {!isLoading && !error && spiders.length === 0 && (
        <div className="bg-surface-container-lowest rounded-xl p-12 tonal-depth text-center">
          <div className="w-16 h-16 rounded-2xl bg-primary-fixed/20 flex items-center justify-center mx-auto mb-4">
            <LayoutTemplate className="w-8 h-8 text-primary" />
          </div>
          <p className="text-sm font-semibold text-on-surface">
            {search || typeFilter ? '没有找到匹配的爬虫' : '还没有创建任何爬虫'}
          </p>
          <p className="text-xs text-on-surface-variant mt-1.5">
            {search || typeFilter ? '请尝试调整搜索条件' : '从模板快速创建一个真实可用的数据采集爬虫'}
          </p>
          {!search && !typeFilter && (
            <button
              onClick={() => { resetCreate(); setShowCreate(true); }}
              className="mt-4 px-4 py-2 text-xs font-medium text-on-primary bg-gradient-to-br from-primary to-primary-container rounded-lg hover:opacity-90 transition-all"
            >
              新建爬虫
            </button>
          )}
        </div>
      )}

      {/* Spider Grid */}
      {!isLoading && !error && spiders.length > 0 && (
        <div className="grid grid-cols-2 xl:grid-cols-3 gap-4">
          {spiders.map((spider) => {
            const tmpl = getTemplateForSpider(spider);
            return (
              <div key={spider._id} className="bg-surface-container-lowest rounded-xl p-5 tonal-depth group">
                {/* Header */}
                <div className="flex items-start justify-between mb-4">
                  <div className="flex items-center gap-3 min-w-0">
                    <div className="w-10 h-10 rounded-lg bg-primary-fixed/20 flex items-center justify-center shrink-0">
                      {getSpiderIcon(spider)}
                    </div>
                    <div className="min-w-0">
                      <h3 className="text-sm font-semibold text-on-surface truncate">
                        {spider.name}
                      </h3>
                      {spider.description && (
                        <p className="text-[11px] text-on-surface-variant mt-0.5 truncate">{spider.description}</p>
                      )}
                    </div>
                  </div>
                </div>

                {/* XHS user highlight for xhs_user_posts spiders */}
                {spider.template_id === 'xhs_user_posts' && spider.config?.user_id && (
                  <div className="flex items-center gap-2.5 mb-4 px-3 py-2.5 rounded-lg bg-primary-fixed/10">
                    <div className="w-7 h-7 rounded-full bg-primary/10 flex items-center justify-center shrink-0">
                      <User className="w-3.5 h-3.5 text-primary" />
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="text-[11px] text-on-surface-variant leading-none">监控用户</div>
                      <div className="text-xs font-semibold text-primary mt-0.5 truncate">
                        @{spider.config.xhs_screen_name || spider.config.user_id}
                      </div>
                    </div>
                    <span className="text-[10px] text-on-surface-variant shrink-0 font-mono">
                      {spider.config.xhs_screen_name ? `ID: ${spider.config.user_id}` : '小红书'}
                    </span>
                  </div>
                )}

                {/* Info */}
                <div className="space-y-2.5 mb-4">
                  <div className="flex items-center justify-between">
                    <span className="text-[11px] text-on-surface-variant">类型</span>
                    <span className="text-[11px] font-medium text-on-surface px-2 py-0.5 rounded bg-surface-container-high">
                      {getSpiderTypeLabel(spider)}
                    </span>
                  </div>
                  {tmpl && (
                    <div className="flex items-center justify-between">
                      <span className="text-[11px] text-on-surface-variant">平台</span>
                      <span className="text-[11px] text-on-surface">{platformLabel[tmpl.platform] || tmpl.platform}</span>
                    </div>
                  )}
                  <div className="flex items-center justify-between">
                    <span className="text-[11px] text-on-surface-variant">执行命令</span>
                    <span className="text-[11px] text-on-surface font-mono truncate max-w-[60%] text-right" title={spider.cmd}>
                      {spider.cmd || '-'}
                    </span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-[11px] text-on-surface-variant">运行模式</span>
                    <span className="text-[11px] text-on-surface">{modeLabel[spider.mode] || spider.mode}</span>
                  </div>
                  {spider.stat && (
                    <>
                      <div className="flex items-center justify-between">
                        <span className="text-[11px] text-on-surface-variant">累计任务</span>
                        <span className="text-[11px] font-medium text-on-surface">{spider.stat.total_tasks}</span>
                      </div>
                      {spider.stat.success_rate > 0 && (
                        <div className="flex items-center justify-between">
                          <span className="text-[11px] text-on-surface-variant">成功率</span>
                          <span className="text-[11px] font-medium text-primary">{(spider.stat.success_rate * 100).toFixed(1)}%</span>
                        </div>
                      )}
                    </>
                  )}
                  {spider.tags && spider.tags.length > 0 && (
                    <div className="flex items-center gap-1 flex-wrap pt-1">
                      {spider.tags.map((tag) => (
                        <span key={tag} className="text-[10px] px-1.5 py-0.5 rounded bg-primary-fixed/15 text-primary">
                          {tag}
                        </span>
                      ))}
                    </div>
                  )}
                </div>

                {/* Timestamp */}
                <div className="flex items-center gap-1 text-[10px] text-outline mb-3">
                  <Clock className="w-3 h-3" />
                  创建于 {new Date(spider.created_at).toLocaleDateString('zh-CN')}
                </div>

                {/* Actions */}
                <div className="flex items-center gap-2 pt-3">
                  <button
                    onClick={() => openRun(spider)}
                    className="flex-1 flex items-center justify-center gap-1 py-2 text-xs font-medium text-on-primary bg-gradient-to-br from-primary to-primary-container rounded-lg hover:opacity-90 transition-all"
                  >
                    <Play className="w-3.5 h-3.5" />
                    运行
                  </button>
                  {spider.type === 'template' && (
                    <button
                      onClick={() => openConfig(spider)}
                      className="p-2 rounded-lg bg-surface-container-high hover:bg-surface-container-highest transition-colors"
                      title="配置参数"
                    >
                      <Sliders className="w-3.5 h-3.5 text-on-surface-variant" />
                    </button>
                  )}
                  <button
                    onClick={() => openFiles(spider)}
                    className="p-2 rounded-lg bg-surface-container-high hover:bg-surface-container-highest transition-colors"
                    title="文件管理"
                  >
                    <FileCode className="w-3.5 h-3.5 text-on-surface-variant" />
                  </button>
                  <button
                    onClick={() => setDeletingSpider(spider)}
                    className="p-2 rounded-lg bg-surface-container-high hover:bg-error-container/30 transition-colors group/del"
                    title="删除"
                  >
                    <Trash2 className="w-3.5 h-3.5 text-on-surface-variant group-hover/del:text-error" />
                  </button>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* ===== Create Modal ===== */}
      {showCreate && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-on-surface/30" onClick={resetCreate} />
          <div className="relative bg-surface-container-lowest rounded-2xl shadow-editorial w-full max-w-2xl mx-4 max-h-[90vh] overflow-y-auto">
            <div className="p-6">
              <div className="flex items-center justify-between mb-6">
                <h2 className="text-base font-bold text-on-surface">
                  {createStep === 'pick' ? '选择爬虫模板' : `配置 — ${selectedTemplate?.name}`}
                </h2>
                <button onClick={resetCreate} className="p-1.5 rounded-lg hover:bg-surface-container-high transition-colors">
                  <X className="w-4 h-4 text-on-surface-variant" />
                </button>
              </div>

              {createStep === 'pick' ? (
                <div className="space-y-4">
                  {/* Template Cards */}
                  {templatesList.length > 0 ? (
                    <div className="space-y-3">
                      {templatesList.map((tmpl) => {
                        const Icon = templateIconMap[tmpl.icon] || Flame;
                        return (
                          <button
                            key={tmpl.id}
                            onClick={() => {
                              setSelectedTemplate(tmpl);
                              setCreateName(tmpl.name);
                              // Init config with defaults
                              const defaults: Record<string, any> = {};
                              tmpl.config_fields.forEach((f) => { defaults[f.key] = f.default; });
                              setCreateConfig(defaults);
                              setCreateTags([...tmpl.tags]);
                              setCreateStep('config');
                            }}
                            className="w-full p-4 rounded-xl bg-surface-container-low hover:bg-surface-container transition-all text-left group/tmpl"
                          >
                            <div className="flex items-center gap-4">
                              <div className="w-12 h-12 rounded-xl bg-primary-fixed/20 flex items-center justify-center shrink-0">
                                <Icon className="w-6 h-6 text-primary" />
                              </div>
                              <div className="flex-1 min-w-0">
                                <div className="flex items-center gap-2">
                                  <h3 className="text-sm font-semibold text-on-surface">{tmpl.name}</h3>
                                  <span className="text-[10px] px-1.5 py-0.5 rounded bg-primary-fixed/15 text-primary">
                                    {platformLabel[tmpl.platform] || tmpl.platform}
                                  </span>
                                </div>
                                <p className="text-[11px] text-on-surface-variant mt-1 line-clamp-2">{tmpl.description}</p>
                                <div className="flex items-center gap-1.5 mt-2">
                                  {tmpl.tags.slice(0, 4).map((tag) => (
                                    <span key={tag} className="text-[10px] px-1.5 py-0.5 rounded bg-surface-container-highest text-on-surface-variant">
                                      {tag}
                                    </span>
                                  ))}
                                </div>
                              </div>
                              <ChevronRight className="w-5 h-5 text-outline group-hover/tmpl:text-primary transition-colors" />
                            </div>
                          </button>
                        );
                      })}
                    </div>
                  ) : templatesLoading ? (
                    <div className="text-center py-8">
                      <Loader2 className="w-6 h-6 text-outline animate-spin mx-auto mb-2" />
                      <p className="text-xs text-on-surface-variant">加载模板中...</p>
                    </div>
                  ) : (
                    <div className="text-center py-8">
                      <AlertTriangle className="w-6 h-6 text-outline mx-auto mb-2" />
                      <p className="text-xs text-on-surface-variant">暂无可用模板</p>
                    </div>
                  )}
                </div>
              ) : selectedTemplate ? (
                <div className="space-y-4">
                  {/* Back button */}
                  <button
                    onClick={() => setCreateStep('pick')}
                    className="text-xs text-primary hover:underline"
                  >
                    ← 返回模板选择
                  </button>

                  {/* Name */}
                  <div className="space-y-1.5">
                    <label className="text-xs font-medium text-on-surface-variant">
                      爬虫名称 {selectedTargetAccounts.length <= 1 && <span className="text-error">*</span>}
                    </label>
                    {selectedTargetAccounts.length > 1 ? (
                      <div className="px-4 py-3 bg-primary-fixed/10 rounded-lg text-sm text-on-surface-variant">
                        将为 {selectedTargetAccounts.length} 个账号分别创建爬虫，自动命名
                      </div>
                    ) : (
                      <input
                        type="text"
                        value={createName}
                        onChange={(e) => setCreateName(e.target.value)}
                        placeholder="例如: 微博热搜监控"
                        className="w-full px-4 py-3 bg-surface-container-highest rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary/30 transition-all placeholder:text-outline-variant"
                      />
                    )}
                  </div>

                  {/* Config Fields */}
                  {selectedTemplate.config_fields.length > 0 && (
                    <div className="space-y-1.5">
                      <label className="text-xs font-medium text-on-surface-variant flex items-center gap-1.5">
                        <Sliders className="w-3.5 h-3.5" />
                        采集参数配置
                      </label>
                      <div className="space-y-3 p-4 rounded-xl bg-surface-container-low">
                        {selectedTemplate.config_fields.map((field) => (
                          <div key={field.key} className="space-y-1">
                            <div className="flex items-center justify-between">
                              <label className="text-[11px] font-medium text-on-surface">
                                {field.label}
                                {field.required && <span className="text-error ml-0.5">*</span>}
                              </label>
                              {field.description && (
                                <span className="text-[10px] text-on-surface-variant">{field.description}</span>
                              )}
                            </div>
                            {selectedTemplate.id === 'weibo_user_posts' && field.key === 'user_id' ? (
                              <TargetAccountPicker
                                platform="weibo"
                                selected={selectedTargetAccounts}
                                onChange={(accounts) => {
                                  setSelectedTargetAccounts(accounts);
                                  if (accounts.length === 1) setCreateName(`${accounts[0].nickname}的微博`);
                                  else if (accounts.length > 1) setCreateName('');
                                }}
                              />
                            ) : selectedTemplate.id === 'xueqiu_user_posts' && field.key === 'user_id' ? (
                              <TargetAccountPicker
                                platform="xueqiu"
                                selected={selectedTargetAccounts}
                                onChange={(accounts) => {
                                  setSelectedTargetAccounts(accounts);
                                  if (accounts.length === 1) setCreateName(`${accounts[0].nickname}的雪球`);
                                  else if (accounts.length > 1) setCreateName('');
                                }}
                              />
                            ) : selectedTemplate.id === 'xhs_user_posts' && field.key === 'user_id' ? (
                              <TargetAccountPicker
                                platform="xhs"
                                selected={selectedTargetAccounts}
                                onChange={(accounts) => {
                                  setSelectedTargetAccounts(accounts);
                                  if (accounts.length === 1) setCreateName(`${accounts[0].nickname}的小红书`);
                                  else if (accounts.length > 1) setCreateName('');
                                }}
                              />
                            ) : selectedTemplate.id === 'wechat_user_articles' && field.key === 'target_account_id' ? (
                              <TargetAccountPicker
                                platform="wechat"
                                selected={selectedTargetAccounts}
                                onChange={(accounts) => {
                                  setSelectedTargetAccounts(accounts);
                                  if (accounts.length === 1) {
                                    setCreateName(`${accounts[0].nickname}的微信`);
                                    setCreateConfig({ ...createConfig, account_name: accounts[0].nickname, biz: accounts[0].user_id });
                                  } else if (accounts.length > 1) {
                                    setCreateName('');
                                  }
                                }}
                              />
                            ) : selectedTemplate.id === 'wechat_user_articles' && field.key === 'biz' ? (
                              <WechatUserSearch
                                value={createConfig[field.key] || ''}
                                onChange={(id, meta) => {
                                  setCreateConfig({ ...createConfig, [field.key]: id });
                                  if (meta?.nickname) {
                                    setCreateConfig({ ...createConfig, [field.key]: id, account_name: meta.nickname });
                                  }
                                }}
                              />
                            ) : (
                              <ConfigFieldInput
                                field={field}
                                value={createConfig[field.key]}
                                onChange={(val) => setCreateConfig({ ...createConfig, [field.key]: val })}
                              />
                            )}
                          </div>
                        ))}
                      </div>
                    </div>
                  )}

                  {/* Mode & Priority */}
                  <div className="grid grid-cols-2 gap-4">
                    <div className="space-y-1.5">
                      <label className="text-xs font-medium text-on-surface-variant">运行模式</label>
                      <select
                        value={createMode}
                        onChange={(e) => { setCreateMode(e.target.value); if (e.target.value !== 'selected_nodes') setCreateNodeIds([]); }}
                        className="w-full px-3 py-3 bg-surface-container-highest rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary/30 transition-all"
                      >
                        <option value="all_nodes">全部节点</option>
                        <option value="selected_nodes">指定节点</option>
                        <option value="random">随机分配</option>
                      </select>
                    </div>
                    <div className="space-y-1.5">
                      <label className="text-xs font-medium text-on-surface-variant">优先级 (1-10)</label>
                      <input
                        type="number"
                        min={1}
                        max={10}
                        value={createPriority}
                        onChange={(e) => setCreatePriority(Number(e.target.value))}
                        className="w-full px-3 py-3 bg-surface-container-highest rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary/30 transition-all"
                      />
                    </div>
                  </div>

                  {/* Node selector */}
                  {createMode === 'selected_nodes' && (
                    <div className="space-y-1.5">
                      <label className="text-xs font-medium text-on-surface-variant">
                        选择节点 <span className="text-error">*</span>
                      </label>
                      <div className="space-y-1.5 max-h-40 overflow-y-auto rounded-lg bg-surface-container-highest p-2">
                        {nodes.map((node) => {
                          const checked = createNodeIds.includes(node._id);
                          return (
                            <label key={node._id} className={`flex items-center gap-2.5 px-3 py-2 rounded-lg cursor-pointer transition-colors ${checked ? 'bg-primary-fixed/20' : 'hover:bg-surface-container-low'}`}>
                              <input
                                type="checkbox"
                                checked={checked}
                                onChange={() => setCreateNodeIds(checked ? createNodeIds.filter((id) => id !== node._id) : [...createNodeIds, node._id])}
                                className="accent-primary w-3.5 h-3.5"
                              />
                              <span className="text-xs font-medium text-on-surface">{node.name || node.key}</span>
                              <span className={`text-[10px] px-1.5 py-0.5 rounded ${node.status === 'online' ? 'bg-primary-fixed/15 text-primary' : 'bg-surface-container-high text-on-surface-variant'}`}>
                                {node.status === 'online' ? '在线' : '离线'}
                              </span>
                            </label>
                          );
                        })}
                      </div>
                    </div>
                  )}

                  {/* Tags */}
                  <div className="space-y-1.5">
                    <label className="text-xs font-medium text-on-surface-variant">标签</label>
                    <div className="flex flex-wrap gap-1.5 min-h-[36px] p-2 bg-surface-container-highest rounded-lg">
                      {createTags.map((tag) => (
                        <span key={tag} className="inline-flex items-center gap-1 text-[11px] px-2 py-1 rounded bg-primary-fixed/15 text-primary">
                          {tag}
                          <button type="button" onClick={() => setCreateTags(createTags.filter((t) => t !== tag))} className="hover:text-error transition-colors">
                            <X className="w-3 h-3" />
                          </button>
                        </span>
                      ))}
                      <input
                        type="text"
                        value={tagInput}
                        onChange={(e) => setTagInput(e.target.value)}
                        onKeyDown={(e) => {
                          if (e.key === 'Enter' && tagInput.trim()) {
                            e.preventDefault();
                            if (!createTags.includes(tagInput.trim())) setCreateTags([...createTags, tagInput.trim()]);
                            setTagInput('');
                          }
                        }}
                        placeholder={createTags.length === 0 ? '输入标签后按回车' : ''}
                        className="flex-1 min-w-[100px] bg-transparent text-sm outline-none placeholder:text-outline-variant"
                      />
                    </div>
                  </div>

                  {/* Submit */}
                  <div className="flex items-center justify-end gap-2 pt-4">
                    <button onClick={resetCreate} className="px-4 py-2 text-xs text-on-surface-variant bg-surface-container-high rounded-lg hover:bg-surface-container-highest transition-colors">
                      取消
                    </button>
                    <button
                      onClick={handleCreateFromTemplate}
                      disabled={createMut.isPending || batchCreating}
                      className="flex items-center gap-1.5 px-5 py-2 text-xs font-medium text-on-primary bg-gradient-to-br from-primary to-primary-container rounded-lg hover:opacity-90 active:scale-95 transition-all disabled:opacity-60 tonal-depth"
                    >
                      {(createMut.isPending || batchCreating) && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
                      {batchCreating ? '创建中...' : selectedTargetAccounts.length > 1 ? `创建 ${selectedTargetAccounts.length} 个爬虫` : '创建爬虫'}
                    </button>
                  </div>
                </div>
              ) : null}
            </div>
          </div>
        </div>
      )}

      {/* ===== Config Modal (for existing template spiders) ===== */}
      {configSpider && (() => {
        const tmpl = getTemplateForSpider(configSpider);
        return (
          <div className="fixed inset-0 z-50 flex items-center justify-center">
            <div className="absolute inset-0 bg-on-surface/30" onClick={() => setConfigSpider(null)} />
            <div className="relative bg-surface-container-lowest rounded-2xl shadow-editorial w-full max-w-lg mx-4 max-h-[85vh] overflow-y-auto">
              <div className="p-6">
                <div className="flex items-center justify-between mb-6">
                  <div className="flex items-center gap-3">
                    <Sliders className="w-5 h-5 text-primary" />
                    <div>
                      <h2 className="text-base font-bold text-on-surface">{configSpider.name} — 配置</h2>
                      <p className="text-[11px] text-on-surface-variant mt-0.5">修改采集参数后保存生效</p>
                    </div>
                  </div>
                  <button onClick={() => setConfigSpider(null)} className="p-1.5 rounded-lg hover:bg-surface-container-high transition-colors">
                    <X className="w-4 h-4 text-on-surface-variant" />
                  </button>
                </div>

                {tmpl ? (
                  <div className="space-y-3">
                    {tmpl.config_fields.map((field) => (
                      <div key={field.key} className="space-y-1">
                        <div className="flex items-center justify-between">
                          <label className="text-[11px] font-medium text-on-surface">
                            {field.label}
                            {field.required && <span className="text-error ml-0.5">*</span>}
                          </label>
                          <span className="text-[10px] text-on-surface-variant">{field.description}</span>
                        </div>
                        {configSpider.template_id === 'weibo_user_posts' && field.key === 'user_id' ? (
                          <WeiboUserSearch
                            value={configValues[field.key] ?? ''}
                            onChange={(uid, meta) => setConfigValues({ ...configValues, [field.key]: uid, ...(meta?.screen_name ? { weibo_screen_name: meta.screen_name } : {}) })}
                          />
                        ) : configSpider.template_id === 'xueqiu_user_posts' && field.key === 'user_id' ? (
                          <XueqiuUserSearch
                            value={configValues[field.key] ?? ''}
                            onChange={(id, meta) => setConfigValues({ ...configValues, [field.key]: id, ...(meta?.screen_name ? { xueqiu_screen_name: meta.screen_name } : {}) })}
                          />
                        ) : configSpider.template_id === 'xhs_user_posts' && field.key === 'user_id' ? (
                          <XhsUserSearch
                            value={configValues[field.key] ?? ''}
                            onChange={(id, meta) => setConfigValues({ ...configValues, [field.key]: id, ...(meta?.screen_name ? { xhs_screen_name: meta.screen_name } : {}) })}
                          />
                        ) : (
                          <ConfigFieldInput
                            field={field}
                            value={configValues[field.key]}
                            onChange={(val) => setConfigValues({ ...configValues, [field.key]: val })}
                          />
                        )}
                      </div>
                    ))}
                  </div>
                ) : (
                  <p className="text-xs text-on-surface-variant">未找到模板配置信息</p>
                )}

                <div className="flex items-center justify-end gap-2 mt-6 pt-4">
                  <button onClick={() => setConfigSpider(null)} className="px-4 py-2 text-xs text-on-surface-variant bg-surface-container-high rounded-lg hover:bg-surface-container-highest transition-colors">
                    取消
                  </button>
                  <button
                    onClick={() => configMut.mutate({ id: configSpider._id, config: configValues })}
                    disabled={configMut.isPending}
                    className="flex items-center gap-1.5 px-5 py-2 text-xs font-medium text-on-primary bg-gradient-to-br from-primary to-primary-container rounded-lg hover:opacity-90 active:scale-95 transition-all disabled:opacity-60 tonal-depth"
                  >
                    {configMut.isPending && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
                    保存配置
                  </button>
                </div>
              </div>
            </div>
          </div>
        );
      })()}

      {/* ===== Delete Confirmation ===== */}
      {deletingSpider && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-on-surface/30" onClick={() => setDeletingSpider(null)} />
          <div className="relative bg-surface-container-lowest rounded-2xl shadow-editorial w-full max-w-sm mx-4 p-6">
            <div className="flex flex-col items-center text-center">
              <div className="w-12 h-12 rounded-full bg-error-container/30 flex items-center justify-center mb-4">
                <AlertTriangle className="w-6 h-6 text-error" />
              </div>
              <h3 className="text-sm font-bold text-on-surface">确认删除爬虫</h3>
              <p className="text-xs text-on-surface-variant mt-2 leading-relaxed">
                即将删除爬虫 <span className="font-medium text-on-surface">{deletingSpider.name}</span>，此操作不可恢复。
              </p>
            </div>
            <div className="flex items-center gap-2 mt-6">
              <button onClick={() => setDeletingSpider(null)} className="flex-1 px-4 py-2 text-xs text-on-surface-variant bg-surface-container-high rounded-lg hover:bg-surface-container-highest transition-colors">
                取消
              </button>
              <button
                onClick={() => deleteMut.mutate(deletingSpider._id)}
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

      {/* ===== Run Modal ===== */}
      {runningSpider && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-on-surface/30" onClick={() => setRunningSpider(null)} />
          <div className="relative bg-surface-container-lowest rounded-2xl shadow-editorial w-full max-w-md mx-4 p-6">
            <div className="flex items-center justify-between mb-5">
              <h3 className="text-sm font-bold text-on-surface">运行爬虫</h3>
              <button onClick={() => setRunningSpider(null)} className="p-1 rounded-lg hover:bg-surface-container-high transition-colors">
                <X className="w-4 h-4 text-on-surface-variant" />
              </button>
            </div>

            <div className="p-4 rounded-xl bg-surface-container-low mb-4">
              <div className="flex items-center gap-3">
                <div className="w-9 h-9 rounded-lg bg-primary-fixed/20 flex items-center justify-center">
                  {getSpiderIcon(runningSpider)}
                </div>
                <div>
                  <p className="text-sm font-medium text-on-surface">{runningSpider.name}</p>
                  <p className="text-[11px] text-on-surface-variant font-mono mt-0.5">{runningSpider.cmd}</p>
                </div>
              </div>
            </div>

            <div className="space-y-1.5 mb-5">
              <label className="text-xs font-medium text-on-surface-variant">运行参数 (可选)</label>
              <input
                type="text"
                value={runParam}
                onChange={(e) => setRunParam(e.target.value)}
                placeholder="传入额外参数，留空使用默认值"
                className="w-full px-4 py-3 bg-surface-container-highest rounded-lg text-sm font-mono outline-none focus:ring-2 focus:ring-primary/30 transition-all placeholder:text-outline-variant"
              />
            </div>

            <div className="flex items-center gap-2">
              <button onClick={() => setRunningSpider(null)} className="flex-1 px-4 py-2 text-xs text-on-surface-variant bg-surface-container-high rounded-lg hover:bg-surface-container-highest transition-colors">
                取消
              </button>
              <button
                onClick={() => runMut.mutate({ id: runningSpider._id, param: runParam || undefined })}
                disabled={runMut.isPending}
                className="flex-1 flex items-center justify-center gap-1.5 px-4 py-2.5 text-xs font-medium text-on-primary bg-gradient-to-br from-primary to-primary-container rounded-lg hover:opacity-90 active:scale-95 transition-all disabled:opacity-60 tonal-depth"
              >
                {runMut.isPending ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Play className="w-3.5 h-3.5" />}
                {runMut.isPending ? '创建任务中...' : '确认运行'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* ===== File Management Modal ===== */}
      {filesSpider && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-on-surface/30" onClick={() => { if (!fileDirty || confirm('有未保存的更改，确定关闭？')) { setFilesSpider(null); setSelectedFile(null); } }} />
          <div className="relative bg-surface-container-lowest rounded-2xl shadow-editorial w-full max-w-4xl mx-4 h-[80vh] flex flex-col overflow-hidden">
            {/* Header */}
            <div className="p-4 flex items-center justify-between shrink-0">
              <div className="flex items-center gap-3">
                <FileCode className="w-5 h-5 text-primary" />
                <div>
                  <h2 className="text-sm font-bold text-on-surface">{filesSpider.name} — 文件管理</h2>
                  <p className="text-[11px] text-on-surface-variant mt-0.5">查看和编辑爬虫源代码文件</p>
                </div>
              </div>
              <div className="flex items-center gap-2">
                <label className="flex items-center gap-1 px-3 py-1.5 text-xs text-on-surface-variant bg-surface-container-high rounded-lg hover:bg-surface-container-highest transition-colors cursor-pointer">
                  <Upload className="w-3 h-3" />
                  上传文件
                  <input type="file" className="hidden" onChange={handleUploadFile} />
                </label>
                <button onClick={() => { setFilesSpider(null); setSelectedFile(null); }} className="p-1.5 rounded-lg hover:bg-surface-container-high transition-colors">
                  <X className="w-4 h-4 text-on-surface-variant" />
                </button>
              </div>
            </div>

            {/* Body */}
            <div className="flex flex-1 overflow-hidden">
              {/* Sidebar */}
              <div className="w-56 shrink-0 bg-surface-container-low overflow-y-auto p-2 space-y-0.5">
                {filesLoading ? (
                  <div className="p-4 text-center"><Loader2 className="w-4 h-4 animate-spin mx-auto text-outline" /></div>
                ) : Array.isArray(files) && files.length > 0 ? (
                  files.filter((f: any) => !f.is_dir).map((f: any) => (
                    <button
                      key={f.path}
                      onClick={() => selectFile(f.path)}
                      className={`w-full text-left px-3 py-2 rounded-lg text-xs truncate transition-colors ${
                        selectedFile === f.path ? 'bg-primary-fixed/20 text-primary font-medium' : 'text-on-surface-variant hover:bg-surface-container'
                      }`}
                    >
                      {f.name}
                    </button>
                  ))
                ) : (
                  <p className="text-xs text-on-surface-variant p-4">暂无文件</p>
                )}
              </div>

              {/* Editor */}
              <div className="flex-1 flex flex-col overflow-hidden">
                {selectedFile ? (
                  <>
                    <div className="px-4 py-2 flex items-center justify-between shrink-0 bg-surface-container-low/50">
                      <span className="text-xs font-mono text-on-surface-variant">{selectedFile}</span>
                      <button
                        onClick={handleSaveFile}
                        disabled={!fileDirty || fileSaving}
                        className={`flex items-center gap-1 px-3 py-1 text-xs rounded-lg transition-all ${
                          fileDirty ? 'bg-primary text-on-primary hover:opacity-90' : 'bg-surface-container-high text-on-surface-variant'
                        }`}
                      >
                        {fileSaving ? <Loader2 className="w-3 h-3 animate-spin" /> : <Save className="w-3 h-3" />}
                        {fileDirty ? '保存' : '已保存'}
                      </button>
                    </div>
                    <textarea
                      value={fileContent}
                      onChange={(e) => { setFileContent(e.target.value); setFileDirty(true); }}
                      className="flex-1 w-full p-4 font-mono text-xs leading-relaxed bg-[#1a1a2e] text-[#e0e0e0] outline-none resize-none"
                      spellCheck={false}
                    />
                  </>
                ) : (
                  <div className="flex-1 flex items-center justify-center">
                    <p className="text-xs text-on-surface-variant">选择左侧文件查看内容</p>
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
