import { useState } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { Activity, User, Mail, Lock, Monitor, Clock, Sparkles } from 'lucide-react';
import { register } from '../lib/api';

export default function Register() {
  const navigate = useNavigate();
  const [form, setForm] = useState({ username: '', email: '', password: '', confirmPassword: '' });
  const [agreed, setAgreed] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (form.password !== form.confirmPassword) {
      setError('两次输入的密码不一致');
      return;
    }
    if (!agreed) {
      setError('请先同意服务条款和隐私政策');
      return;
    }
    setLoading(true);
    setError('');
    try {
      const res = await register({ username: form.username, email: form.email, password: form.password });
      localStorage.setItem('token', res.token);
      navigate('/dashboard');
    } catch {
      setError('注册失败，请稍后重试');
    } finally {
      setLoading(false);
    }
  };

  const features = [
    { icon: Monitor, title: '多平台支持', desc: '无论在何处，实时接入数据流。' },
    { icon: Clock, title: '实时监控', desc: '毫秒级延迟，掌控全局动态。' },
    { icon: Sparkles, title: '自动报告生成', desc: '数据可视化，一键转化为专业报告。' },
  ];

  return (
    <div className="min-h-screen flex flex-col md:flex-row">
      {/* Left Brand Panel */}
      <section className="hidden md:flex md:w-2/5 bg-gradient-to-br from-primary to-primary-container relative overflow-hidden">
        <div className="relative z-10 flex flex-col justify-between p-12 text-white w-full">
          <div className="flex items-center gap-2">
            <Activity className="w-6 h-6" />
            <span className="text-sm font-semibold tracking-wide">Veridian Metric</span>
          </div>

          <div className="space-y-8">
            <div>
              <h1 className="text-3xl font-bold leading-tight">开启您的<br />精准观察之旅</h1>
              <p className="text-white/70 text-sm mt-3">
                Precision Observer 为分布式系统提供最前沿的洞察力。
              </p>
            </div>

            <div className="space-y-5">
              {features.map((f) => (
                <div key={f.title} className="flex items-start gap-3">
                  <div className="w-8 h-8 rounded-lg bg-white/10 flex items-center justify-center shrink-0 mt-0.5">
                    <f.icon className="w-4 h-4" />
                  </div>
                  <div>
                    <p className="text-sm font-medium">{f.title}</p>
                    <p className="text-xs text-white/60 mt-0.5">{f.desc}</p>
                  </div>
                </div>
              ))}
            </div>
          </div>

          <p className="text-xs text-white/30">Precision Observer — Distributed Intelligence Unit</p>
        </div>
        <div className="absolute top-10 right-10 w-56 h-56 rounded-full bg-primary-fixed/10 blur-3xl" />
      </section>

      {/* Right Registration Form */}
      <main className="flex-1 flex items-center justify-center bg-surface-container-lowest p-8">
        <div className="w-full max-w-md space-y-8">
          <div className="md:hidden flex items-center gap-2 mb-6">
            <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-primary to-primary-container flex items-center justify-center">
              <Activity className="w-4 h-4 text-white" />
            </div>
            <span className="text-sm font-bold text-on-surface">精准观察者</span>
          </div>

          <div>
            <h2 className="text-2xl font-bold text-on-surface">注册账号</h2>
            <p className="text-sm text-on-surface-variant mt-2">
              请填写以下信息以创建您的访问凭据。
            </p>
          </div>

          {error && (
            <div className="px-4 py-3 rounded-lg bg-error-container/30 text-sm text-on-error-container">
              {error}
            </div>
          )}

          <form onSubmit={handleSubmit} className="space-y-5">
            <div className="space-y-1.5">
              <label className="text-xs font-medium text-on-surface-variant">用户名</label>
              <div className="relative">
                <User className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-outline" />
                <input
                  type="text"
                  value={form.username}
                  onChange={(e) => setForm({ ...form, username: e.target.value })}
                  placeholder="请输入您的用户名"
                  className="w-full pl-10 pr-4 py-3 bg-surface-container-highest rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary/30 transition-all placeholder:text-outline-variant"
                  required
                />
              </div>
            </div>

            <div className="space-y-1.5">
              <label className="text-xs font-medium text-on-surface-variant">电子邮箱</label>
              <div className="relative">
                <Mail className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-outline" />
                <input
                  type="email"
                  value={form.email}
                  onChange={(e) => setForm({ ...form, email: e.target.value })}
                  placeholder="example@precision.com"
                  className="w-full pl-10 pr-4 py-3 bg-surface-container-highest rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary/30 transition-all placeholder:text-outline-variant"
                  required
                />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-1.5">
                <label className="text-xs font-medium text-on-surface-variant">设置密码</label>
                <div className="relative">
                  <Lock className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-outline" />
                  <input
                    type="password"
                    value={form.password}
                    onChange={(e) => setForm({ ...form, password: e.target.value })}
                    placeholder="至少8位字符"
                    className="w-full pl-10 pr-4 py-3 bg-surface-container-highest rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary/30 transition-all placeholder:text-outline-variant"
                    required
                    minLength={8}
                  />
                </div>
              </div>
              <div className="space-y-1.5">
                <label className="text-xs font-medium text-on-surface-variant">确认密码</label>
                <div className="relative">
                  <Lock className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-outline" />
                  <input
                    type="password"
                    value={form.confirmPassword}
                    onChange={(e) => setForm({ ...form, confirmPassword: e.target.value })}
                    placeholder="请再次输入"
                    className="w-full pl-10 pr-4 py-3 bg-surface-container-highest rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary/30 transition-all placeholder:text-outline-variant"
                    required
                    minLength={8}
                  />
                </div>
              </div>
            </div>

            <label className="flex items-start gap-2 cursor-pointer">
              <input
                type="checkbox"
                checked={agreed}
                onChange={(e) => setAgreed(e.target.checked)}
                className="w-4 h-4 rounded accent-primary mt-0.5"
              />
              <span className="text-xs text-on-surface-variant leading-relaxed">
                注册即表示您同意我们的{' '}
                <button type="button" className="text-primary font-medium hover:underline">服务条款</button>
                {' '}和{' '}
                <button type="button" className="text-primary font-medium hover:underline">隐私政策</button>
              </span>
            </label>

            <button
              type="submit"
              disabled={loading}
              className="w-full py-3 text-sm font-medium text-on-primary bg-gradient-to-br from-primary to-primary-container rounded-lg hover:opacity-90 active:scale-[0.98] transition-all disabled:opacity-60 tonal-depth"
            >
              {loading ? '注册中...' : '立即注册'}
            </button>
          </form>

          <p className="text-center text-sm text-on-surface-variant">
            已有账号？{' '}
            <Link to="/login" className="text-primary font-medium hover:underline">
              立即登录
            </Link>
          </p>
        </div>
      </main>
    </div>
  );
}
