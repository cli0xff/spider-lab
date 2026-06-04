import axios from 'axios';
import type {
  LoginRequest, RegisterRequest, AuthResponse,
  User, Node, Spider, SpiderTemplate, Task, Schedule, LogEntry, Setting,
  PaginatedResponse, StatsOverview, DailyTaskStat, SystemLogEntry, ResultItem,
  WeiboUserResult, XueqiuUserResult, XhsUserResult, WechatUserResult,
  XueqiuCookie, QRLoginSession, TargetAccount, WechatCookie,
} from '../types';

const api = axios.create({
  baseURL: '/api',
  headers: { 'Content-Type': 'application/json' },
});

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

api.interceptors.response.use(
  (res) => {
    const body = res.data;
    if (body && typeof body === 'object' && 'code' in body) {
      if ('total' in body) {
        res.data = { data: body.data ?? [], total: body.total, page: body.page, size: body.size };
      } else {
        res.data = body.data;
      }
    }
    return res;
  },
  (err) => {
    if (err.response?.status === 401) {
      localStorage.removeItem('token');
      window.location.href = '/login';
    }
    const msg = err.response?.data?.message || err.message;
    return Promise.reject(new Error(msg));
  }
);

// Auth
export const login = (data: LoginRequest) =>
  api.post<AuthResponse>('/login', data).then(r => r.data);

export const register = (data: RegisterRequest) =>
  api.post<AuthResponse>('/register', data).then(r => r.data);

export const getMe = () =>
  api.get<User>('/users/me').then(r => r.data);

// Nodes
export const getNodes = (params?: Record<string, any>) =>
  api.get<PaginatedResponse<Node>>('/nodes', { params }).then(r => r.data);

export const getNode = (id: string) =>
  api.get<Node>(`/nodes/${id}`).then(r => r.data);

export const updateNode = (id: string, data: Partial<Node>) =>
  api.put<Node>(`/nodes/${id}`, data).then(r => r.data);

export const deleteNode = (id: string) =>
  api.delete(`/nodes/${id}`);

export const enableNode = (id: string) =>
  api.post(`/nodes/${id}/enable`);

export const disableNode = (id: string) =>
  api.post(`/nodes/${id}/disable`);

// Spiders
export const getSpiders = (params?: Record<string, any>) =>
  api.get<PaginatedResponse<Spider>>('/spiders', { params }).then(r => r.data);

export const getSpider = (id: string) =>
  api.get<Spider>(`/spiders/${id}`).then(r => r.data);

export const createSpider = (data: Partial<Spider>) =>
  api.post<Spider>('/spiders', data).then(r => r.data);

export const updateSpider = (id: string, data: Partial<Spider>) =>
  api.put<Spider>(`/spiders/${id}`, data).then(r => r.data);

export const deleteSpider = (id: string) =>
  api.delete(`/spiders/${id}`);

export const runSpider = (id: string, param?: string) =>
  api.put<Task>(`/spiders/${id}/run`, { param }).then(r => r.data);

export const getSpiderFiles = (id: string) =>
  api.get<{ name: string; path: string; size: number; is_dir: boolean }[]>(`/spiders/${id}/files`).then(r => r.data);

export const getSpiderFileContent = (id: string, path: string) =>
  api.get<{ path: string; content: string }>(`/spiders/${id}/file`, { params: { path } }).then(r => r.data);

export const saveSpiderFileContent = (id: string, path: string, content: string) =>
  api.put(`/spiders/${id}/file`, { path, content }).then(r => r.data);

export const uploadSpiderFile = (id: string, file: File) => {
  const formData = new FormData();
  formData.append('file', file);
  return api.post(`/spiders/${id}/upload`, formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  }).then(r => r.data);
};

export const getSpiderTemplates = () =>
  api.get<SpiderTemplate[]>('/spiders/templates').then(r => r.data);

export const updateSpiderConfig = (id: string, config: Record<string, any>) =>
  api.put(`/spiders/${id}/config`, { config }).then(r => r.data);

export const searchWeiboUser = (q: string) =>
  api.get<WeiboUserResult[]>('/spiders/weibo/search-user', { params: { q } }).then(r => r.data);

export const searchXueqiuUser = (q: string) =>
  api.get<XueqiuUserResult[]>('/spiders/xueqiu/search-user', { params: { q } }).then(r => r.data);

export const lookupXhsUser = (id: string) =>
  api.get<XhsUserResult>('/spiders/xhs/lookup-user', { params: { id } }).then(r => r.data);

export const lookupXueqiuUser = (id: string) =>
  api.get<{ id: string; screen_name: string; avatar_url: string }>('/spiders/xueqiu/lookup-user', { params: { id } }).then(r => r.data);

export const lookupWeiboUser = (uid: string) =>
  api.get<{ uid: string; screen_name: string; avatar_url: string }>('/spiders/weibo/lookup-user', { params: { uid } }).then(r => r.data);

export const searchWechatUser = (q: string) =>
  api.get<WechatUserResult[]>('/spiders/wechat/search-user', { params: { q } }).then(r => r.data);

// Tasks
export const getTasks = (params?: Record<string, any>) =>
  api.get<PaginatedResponse<Task>>('/tasks', { params }).then(r => r.data);

export const getTask = (id: string) =>
  api.get<Task>(`/tasks/${id}`).then(r => r.data);

export const createTask = (data: { spider_id: string; node_ids?: string[]; cmd?: string; param?: string; mode?: string; priority?: number }) =>
  api.post<Task>('/tasks', data).then(r => r.data);

export const cancelTask = (id: string) =>
  api.post(`/tasks/${id}/cancel`);

export const restartTask = (id: string) =>
  api.post(`/tasks/${id}/restart`);

export const deleteTask = (id: string) =>
  api.delete(`/tasks/${id}`);

// Schedules
export const getSchedules = (params?: Record<string, any>) =>
  api.get<PaginatedResponse<Schedule>>('/schedules', { params }).then(r => r.data);

export const getSchedule = (id: string) =>
  api.get<Schedule>(`/schedules/${id}`).then(r => r.data);

export const createSchedule = (data: Partial<Schedule>) =>
  api.post<Schedule>('/schedules', data).then(r => r.data);

export const updateSchedule = (id: string, data: Partial<Schedule>) =>
  api.put<Schedule>(`/schedules/${id}`, data).then(r => r.data);

export const deleteSchedule = (id: string) =>
  api.delete(`/schedules/${id}`);

export const enableSchedule = (id: string) =>
  api.post(`/schedules/${id}/enable`);

export const disableSchedule = (id: string) =>
  api.post(`/schedules/${id}/disable`);

// Logs
export const getLogs = (taskId: string, params?: Record<string, any>) =>
  api.get<PaginatedResponse<LogEntry>>(`/tasks/${taskId}/logs`, { params }).then(r => r.data);

// Task Results
export const getTaskResults = (taskId: string, params?: Record<string, any>) =>
  api.get<PaginatedResponse<ResultItem>>(`/tasks/${taskId}/results`, { params }).then(r => r.data);

// Global Results (all spiders)
export const getResults = (params?: Record<string, any>) =>
  api.get<PaginatedResponse<ResultItem>>('/results', { params }).then(r => r.data);

export const getResultStats = () =>
  api.get<{ total: number; today_count: number }>('/results/stats').then(r => r.data);

// Settings
export const getSettings = () =>
  api.get<Setting[]>('/settings').then(r => r.data);

export const updateSetting = (key: string, data: { value: any }) =>
  api.put<Setting>(`/settings/${key}`, data).then(r => r.data);

// Dashboard / Stats
export const getStatsOverview = () =>
  api.get<StatsOverview>('/stats/overview').then(r => r.data);

export const getTaskStats = () =>
  api.get<DailyTaskStat[]>('/stats/tasks').then(r => r.data);

// System Logs
export const getSystemLogs = (params?: Record<string, any>) =>
  api.get<PaginatedResponse<SystemLogEntry>>('/logs/system', { params }).then(r => r.data);

// User
export const updateMe = (data: Partial<User>) =>
  api.put<User>('/users/me', data).then(r => r.data);

// Xueqiu Cookie Pool
export const getXueqiuCookies = () =>
  api.get<XueqiuCookie[]>('/xueqiu/cookies').then(r => r.data);

export const deleteXueqiuCookie = (id: string) =>
  api.delete(`/xueqiu/cookies/${id}`);

export const updateXueqiuCookieStatus = (id: string, status: string) =>
  api.put(`/xueqiu/cookies/${id}/status`, { status }).then(r => r.data);

export const checkXueqiuCookie = (id: string) =>
  api.post<{ valid: boolean; error?: string }>(`/xueqiu/cookies/${id}/check`).then(r => r.data);

export const startQRLogin = () =>
  api.post<{ session_id: string }>('/xueqiu/cookies/qr-start').then(r => r.data);

export const getQRLoginStatus = (sessionId: string) =>
  api.get<QRLoginSession>(`/xueqiu/cookies/qr/${sessionId}`).then(r => r.data);

export const cancelQRLogin = (sessionId: string) =>
  api.post(`/xueqiu/cookies/qr/${sessionId}/cancel`);

// Target Accounts
export const getTargetAccounts = (platform?: string) =>
  api.get<TargetAccount[]>('/target-accounts', { params: platform ? { platform } : {} }).then(r => r.data);

export const addTargetAccount = (data: { platform: string; user_id: string; nickname: string; avatar_url: string; description?: string; followers_count?: number; verified?: boolean }) =>
  api.post('/target-accounts', data).then(r => r.data);

export const deleteTargetAccount = (id: string) =>
  api.delete(`/target-accounts/${id}`);

export const getTargetAccountsByPlatform = (platform: string) =>
  api.get<{ value: string; label: string; user_id: string; nickname: string }[]>('/target-accounts/by-platform', { params: { platform } }).then(r => r.data);

// WeChat Cookie Pool
export const getWechatCookies = () =>
  api.get<WechatCookie[]>('/wechat/cookies').then(r => r.data);

export const deleteWechatCookie = (id: string) =>
  api.delete(`/wechat/cookies/${id}`);

export const updateWechatCookieStatus = (id: string, status: string) =>
  api.put(`/wechat/cookies/${id}/status`, { status }).then(r => r.data);

export const checkWechatCookie = (id: string) =>
  api.post<{ valid: boolean; error?: string }>(`/wechat/cookies/${id}/check`).then(r => r.data);

export const startWechatQRLogin = () =>
  api.post<{ session_id: string }>('/wechat/cookies/qr-start').then(r => r.data);

export const getWechatQRLoginStatus = (sessionId: string) =>
  api.get<QRLoginSession>(`/wechat/cookies/qr/${sessionId}`).then(r => r.data);

export const cancelWechatQRLogin = (sessionId: string) =>
  api.post(`/wechat/cookies/qr/${sessionId}/cancel`);

export const addWechatCookieManual = (data: { nickname: string; cookie: string; token?: string; mp_account?: string; notes?: string }) =>
  api.post('/wechat/cookies/add-manual', data).then(r => r.data);

export default api;
