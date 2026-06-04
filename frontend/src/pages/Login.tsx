import { useState } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { Activity, Mail, Lock, Eye, EyeOff, ArrowRight } from 'lucide-react';
import { login } from '../lib/api';

export default function Login() {
  const navigate = useNavigate();
  const [form, setForm] = useState({ username: '', password: '' });
  const [showPwd, setShowPwd] = useState(false);
  const [remember, setRemember] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError('');
    try {
      const res = await login(form);
      localStorage.setItem('token', res.token);
      navigate('/dashboard');
    } catch {
      setError('用户名或密码错误');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex">
      {/* Left Brand Panel */}
      <section className="hidden lg:flex lg:w-1/2 bg-gradient-to-br from-primary to-primary-container relative overflow-hidden">
        <div className="relative z-10 flex flex-col justify-between p-12 text-white w-full">
          <div className="flex items-center gap-2">
            <Activity className="w-6 h-6" />
            <span className="text-sm font-semibold tracking-wide">Veridian Metric</span>
          </div>

          <div className="space-y-6">
            <h1 className="text-4xl font-bold tracking-tight leading-tight">
              Precision<br />Observer
            </h1>
            <p className="text-primary-fixed text-lg font-medium">分布式数据采集专家</p>
            <p className="text-white/70 text-sm leading-relaxed max-w-md">
              利用行业领先的指标分析与实时流处理技术，为您的全球数据节点提供无与伦比的可视化洞察。
            </p>
            <div className="flex gap-8 pt-4">
              <div>
                <p className="text-2xl font-bold">99.9%</p>
                <p className="text-xs text-white/60 mt-1">系统可用性</p>
              </div>
              <div>
                <p className="text-2xl font-bold">250ms</p>
                <p className="text-xs text-white/60 mt-1">实时延迟</p>
              </div>
            </div>
          </div>

          <p className="text-xs text-white/40">加入全球 2,000+ 专业数据团队</p>
        </div>

        {/* Decorative */}
        <div className="absolute top-20 right-20 w-64 h-64 rounded-full bg-primary-fixed/10 blur-3xl" />
        <div className="absolute bottom-10 left-10 w-48 h-48 rounded-full bg-white/5 blur-2xl" />
      </section>

      {/* Right Login Form */}
      <main className="flex-1 flex items-center justify-center bg-surface-container-lowest p-8">
        <div className="w-full max-w-md space-y-8">
          {/* Mobile logo */}
          <div className="lg:hidden flex items-center gap-2 mb-6">
            <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-primary to-primary-container flex items-center justify-center">
              <Activity className="w-4 h-4 text-white" />
            </div>
            <span className="text-sm font-bold text-on-surface">精准观察者</span>
          </div>

          <div>
            <h2 className="text-2xl font-bold text-on-surface">用户登录</h2>
            <p className="text-sm text-on-surface-variant mt-2">
              欢迎回来。请在下方输入您的凭据以访问控制面板。
            </p>
          </div>

          {error && (
            <div className="px-4 py-3 rounded-lg bg-error-container/30 text-sm text-on-error-container">
              {error}
            </div>
          )}

          <form onSubmit={handleSubmit} className="space-y-5">
            <div className="space-y-1.5">
              <label className="text-xs font-medium text-on-surface-variant">用户名 / 邮箱地址</label>
              <div className="relative">
                <Mail className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-outline" />
                <input
                  type="text"
                  value={form.username}
                  onChange={(e) => setForm({ ...form, username: e.target.value })}
                  placeholder="name@company.com"
                  className="w-full pl-10 pr-4 py-3 bg-surface-container-highest rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary/30 transition-all placeholder:text-outline-variant"
                  required
                />
              </div>
            </div>

            <div className="space-y-1.5">
              <label className="text-xs font-medium text-on-surface-variant">密码</label>
              <div className="relative">
                <Lock className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-outline" />
                <input
                  type={showPwd ? 'text' : 'password'}
                  value={form.password}
                  onChange={(e) => setForm({ ...form, password: e.target.value })}
                  placeholder="••••••••"
                  className="w-full pl-10 pr-12 py-3 bg-surface-container-highest rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary/30 transition-all placeholder:text-outline-variant"
                  required
                />
                <button
                  type="button"
                  onClick={() => setShowPwd(!showPwd)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-outline hover:text-on-surface transition-colors"
                >
                  {showPwd ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                </button>
              </div>
            </div>

            <div className="flex items-center justify-between">
              <label className="flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  checked={remember}
                  onChange={(e) => setRemember(e.target.checked)}
                  className="w-4 h-4 rounded accent-primary"
                />
                <span className="text-xs text-on-surface-variant">记住我</span>
              </label>
              <button type="button" className="text-xs text-primary font-medium hover:underline">
                忘记密码？
              </button>
            </div>

            <button
              type="submit"
              disabled={loading}
              className="w-full flex items-center justify-center gap-2 py-3 text-sm font-medium text-on-primary bg-gradient-to-br from-primary to-primary-container rounded-lg hover:opacity-90 active:scale-[0.98] transition-all disabled:opacity-60 tonal-depth"
            >
              {loading ? '登录中...' : '立即登录'}
              {!loading && <ArrowRight className="w-4 h-4" />}
            </button>
          </form>

          <p className="text-center text-sm text-on-surface-variant">
            还没有账号？{' '}
            <Link to="/register" className="text-primary font-medium hover:underline">
              立即注册
            </Link>
          </p>

          <div className="flex justify-center gap-6 pt-4">
            <button className="text-xs text-outline-variant hover:text-on-surface-variant">隐私政策</button>
            <button className="text-xs text-outline-variant hover:text-on-surface-variant">服务条款</button>
            <button className="text-xs text-outline-variant hover:text-on-surface-variant">帮助中心</button>
          </div>
        </div>
      </main>
    </div>
  );
}
