export interface User {
  id: string;
  username: string;
  email: string;
  role: string;
  avatar?: string;
  created_at: string;
  updated_at: string;
}

export interface Node {
  _id: string;
  name: string;
  key: string;
  ip: string;
  port: number;
  description?: string;
  is_master: boolean;
  status: 'online' | 'offline';
  enabled: boolean;
  active_runners: number;
  max_runners: number;
  available_runners: number;
  cpu_usage: number;
  memory_usage: number;
  disk_usage: number;
  last_heartbeat: string;
  created_at: string;
  updated_at: string;
}

export interface ConfigField {
  key: string;
  label: string;
  type: 'string' | 'number' | 'boolean' | 'select';
  default: any;
  required: boolean;
  description: string;
  options?: string[];
  min?: number;
  max?: number;
  placeholder?: string;
}

export interface SpiderTemplate {
  id: string;
  name: string;
  platform: string;
  description: string;
  icon: string;
  tags: string[];
  config_fields: ConfigField[];
  cmd: string;
}

export interface Spider {
  _id: string;
  name: string;
  type: string;
  template_id?: string;
  config?: Record<string, any>;
  description: string;
  cmd: string;
  param: string;
  mode: string;
  node_ids?: string[];
  col_name: string;
  priority: number;
  project: string;
  tags: string[];
  stat?: {
    total_tasks: number;
    total_results: number;
    average_duration: number;
    success_rate: number;
    last_run_at: string;
  };
  created_at: string;
  updated_at: string;
  created_by: string;
}

export interface Task {
  _id: string;
  spider_id: string;
  node_id: string;
  status: 'pending' | 'running' | 'finished' | 'error' | 'cancelled' | 'waiting';
  cmd: string;
  param?: string;
  error?: string;
  pid?: number;
  schedule_id?: string;
  type: string;
  priority: number;
  result_count?: number;
  run_ts?: string;
  start_ts?: string;
  end_ts?: string;
  spider?: Spider;
  node?: Node;
  created_at: string;
  updated_at: string;
  created_by?: string;
}

export interface Schedule {
  id: string;
  name: string;
  spider_id: string;
  spider_name?: string;
  cron: string;
  cmd?: string;
  param?: string;
  mode: 'all-nodes' | 'selected-nodes' | 'random';
  node_ids?: string[];
  enabled: boolean;
  description?: string;
  created_at: string;
  updated_at: string;
}

export interface LogEntry {
  _id: string;
  task_id: string;
  content: string;
  level: 'info' | 'warn' | 'error' | 'debug';
  timestamp: string;
}

export interface Setting {
  id: string;
  key: string;
  value: any;
  description?: string;
  category: string;
}

export interface LoginRequest {
  username: string;
  password: string;
}

export interface RegisterRequest {
  username: string;
  email: string;
  password: string;
}

export interface AuthResponse {
  token: string;
  user: User;
}

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  size: number;
}

export interface StatsOverview {
  nodes: { total: number; online: number };
  tasks: { total: number; running: number; pending: number; finished: number; error: number };
  spiders: number;
  active_schedules: number;
}

export interface DailyTaskStat {
  date: string;
  total: number;
  finished: number;
  error: number;
}

export interface DashboardStats {
  total_tasks: number;
  running_tasks: number;
  total_nodes: number;
  online_nodes: number;
  total_spiders: number;
  total_results: number;
  daily_tasks: { date: string; count: number }[];
  task_by_status: { status: string; count: number }[];
}

export interface SystemLogEntry {
  _id: string;
  level: 'info' | 'warn' | 'error' | 'debug';
  module: string;
  message: string;
  detail?: string;
  timestamp: string;
}

export interface ResultItem {
  _id: string;
  task_id: string;
  spider_id: string;
  item: Record<string, any>;
  published_at?: string;
  timestamp: string;
}

export interface ResultStats {
  total: number;
  today_count: number;
}

export interface WeiboUserResult {
  uid: string;
  screen_name: string;
  avatar_url: string;
  description: string;
  followers_count: number;
  verified: boolean;
  verified_type: number;
  verified_reason: string;
}

export interface XueqiuUserResult {
  id: string;
  screen_name: string;
  avatar_url: string;
  description: string;
  followers_count: number;
  verified: boolean;
  verified_description: string;
}

export interface XhsUserResult {
  user_id: string;
  nickname: string;
  avatar: string;
  desc: string;
  red_id: string;
  followers_count?: number;
  verified?: boolean;
}

export interface WechatUserResult {
  id: string;
  nickname: string;
  avatar_url: string;
  description: string;
  followers_count: number;
  verified: boolean;
}

export interface XueqiuCookie {
  _id: string;
  nickname: string;
  xueqiu_uid: string;
  avatar_url: string;
  has_cookie: boolean;
  status: 'active' | 'cooldown' | 'expired' | 'disabled';
  effective_status: 'active' | 'cooldown' | 'expired' | 'disabled';
  cooldown_left: number; // seconds remaining, 0 if not in cooldown
  created_at: string;
  last_used_at: string;
  last_check_at: string;
  cooldown_till: string;
  total_tasks: number;
  daily_tasks: number;
  error_count: number;
  last_error: string;
  notes: string;
}

export interface QRLoginSession {
  id: string;
  status: 'pending' | 'qr_ready' | 'success' | 'expired' | 'error';
  qr_image?: string;
  nickname?: string;
  uid?: string;
  avatar_url?: string;
  error?: string;
}

export interface TargetAccount {
  _id: string;
  platform: 'xueqiu' | 'weibo' | 'xhs' | 'wechat';
  user_id: string;
  nickname: string;
  avatar_url: string;
  description: string;
  followers_count: number;
  verified: boolean;
  created_at: string;
  notes: string;
}

export interface WechatCookie {
  _id: string;
  nickname: string;
  mp_account: string;
  has_cookie: boolean;
  status: 'active' | 'cooldown' | 'expired' | 'disabled';
  effective_status: 'active' | 'cooldown' | 'expired' | 'disabled';
  cooldown_left: number; // seconds remaining, 0 if not in cooldown
  created_at: string;
  last_used_at: string;
  last_check_at: string;
  cooldown_till: string;
  total_tasks: number;
  daily_tasks: number;
  error_count: number;
  last_error: string;
  notes: string;
}
