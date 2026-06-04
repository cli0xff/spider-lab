import { Outlet, NavLink, useNavigate, Link } from 'react-router-dom';
import {
  LayoutDashboard, ListOrdered, Bug, Server, Clock,
  Database, ScrollText, Settings, LogOut, Search,
  Bell, Plus, Activity, Users,
} from 'lucide-react';
import { clsx } from 'clsx';

const navItems = [
  { to: '/dashboard', icon: LayoutDashboard, label: '控制面板' },
  { to: '/tasks', icon: ListOrdered, label: '任务队列' },
  { to: '/spiders', icon: Bug, label: '采集爬虫' },
  { to: '/nodes', icon: Server, label: '节点管理' },
  { to: '/schedules', icon: Clock, label: '定时任务' },
  { to: '/accounts', icon: Users, label: '账号管理' },
  { to: '/data', icon: Database, label: '数据中心' },
  { to: '/logs', icon: ScrollText, label: '系统日志' },
  { to: '/settings', icon: Settings, label: '设置' },
];

export default function Layout() {
  const navigate = useNavigate();

  const handleLogout = () => {
    localStorage.removeItem('token');
    navigate('/login');
  };

  return (
    <div className="min-h-screen bg-surface">
      {/* Sidebar */}
      <aside className="fixed inset-y-0 left-0 w-64 bg-surface-container-low z-40 flex flex-col">
        {/* Logo */}
        <div className="px-6 py-6">
          <div className="flex items-center gap-3">
            <div className="w-9 h-9 rounded-lg bg-gradient-to-br from-primary to-primary-container flex items-center justify-center">
              <Activity className="w-5 h-5 text-white" />
            </div>
            <div>
              <h1 className="text-sm font-bold text-on-surface tracking-tight">观察中心</h1>
              <p className="text-[11px] text-on-surface-variant">分布式数据采集系统</p>
            </div>
          </div>
        </div>

        {/* Navigation */}
        <nav className="flex-1 px-3 space-y-0.5">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) =>
                clsx(
                  'flex items-center gap-3 px-3 py-2.5 rounded-lg text-[13px] transition-all duration-150',
                  isActive
                    ? 'bg-surface-container-lowest text-primary font-semibold tonal-depth'
                    : 'text-on-surface-variant hover:bg-surface-container-high'
                )
              }
            >
              <item.icon className="w-[18px] h-[18px]" />
              <span>{item.label}</span>
            </NavLink>
          ))}
        </nav>

        {/* Bottom */}
        <div className="px-4 pb-5 space-y-3">
          <div className="flex items-center gap-2 px-3 py-2 rounded-lg bg-surface-container">
            <span className="relative flex h-2 w-2">
              <span className="pulse-live absolute inline-flex h-full w-full rounded-full bg-primary-fixed opacity-75" />
              <span className="relative inline-flex rounded-full h-2 w-2 bg-primary" />
            </span>
            <span className="text-xs text-on-surface-variant">系统运行中</span>
          </div>
          <button
            onClick={handleLogout}
            className="flex items-center gap-2 px-3 py-2 text-xs text-error hover:bg-error-container/30 rounded-lg w-full transition-colors"
          >
            <LogOut className="w-4 h-4" />
            <span>安全退出</span>
          </button>
        </div>
      </aside>

      {/* Top Header */}
      <header className="fixed top-0 left-64 right-0 h-16 z-30 glass-header bg-surface-container-lowest/80 flex items-center justify-between px-6">
        <h2 className="text-base font-bold bg-gradient-to-r from-primary to-primary-container bg-clip-text text-transparent">
          精准观察者 <span className="font-normal text-sm">Precision Observer</span>
        </h2>

        <div className="flex items-center gap-3">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-on-surface-variant" />
            <input
              type="text"
              placeholder="搜索任务或节点..."
              className="pl-9 pr-4 py-2 text-xs bg-surface-container-high rounded-full w-56 outline-none focus:ring-2 focus:ring-primary/20 transition-all placeholder:text-outline-variant"
            />
          </div>
          <button className="relative p-2 rounded-lg hover:bg-surface-container-high transition-colors">
            <Bell className="w-[18px] h-[18px] text-on-surface-variant" />
            <span className="absolute top-1.5 right-1.5 w-1.5 h-1.5 bg-error rounded-full" />
          </button>
          <Link
            to="/tasks/create"
            className="flex items-center gap-1.5 px-4 py-2 text-xs font-medium text-on-primary bg-gradient-to-br from-primary to-primary-container rounded-lg hover:opacity-90 active:scale-95 transition-all tonal-depth"
          >
            <Plus className="w-4 h-4" />
            新建采集任务
          </Link>
        </div>
      </header>

      {/* Main Content */}
      <main className="ml-64 pt-16 min-h-screen">
        <div className="p-6">
          <Outlet />
        </div>
      </main>
    </div>
  );
}
