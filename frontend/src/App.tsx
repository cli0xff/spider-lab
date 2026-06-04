import { Routes, Route, Navigate } from 'react-router-dom'
import { lazy, Suspense } from 'react'
import Layout from './components/Layout'

const Login = lazy(() => import('./pages/Login'))
const Register = lazy(() => import('./pages/Register'))
const Dashboard = lazy(() => import('./pages/Dashboard'))
const Nodes = lazy(() => import('./pages/Nodes'))
const Spiders = lazy(() => import('./pages/Spiders'))
const Tasks = lazy(() => import('./pages/Tasks'))
const CreateTask = lazy(() => import('./pages/CreateTask'))
const Schedules = lazy(() => import('./pages/Schedules'))
const DataCenter = lazy(() => import('./pages/DataCenter'))
const Accounts = lazy(() => import('./pages/Accounts'))
const SystemLogs = lazy(() => import('./pages/SystemLogs'))
const Settings = lazy(() => import('./pages/Settings'))

function Loading() {
  return (
    <div className="min-h-screen flex items-center justify-center bg-surface">
      <div className="flex flex-col items-center gap-4">
        <div className="w-10 h-10 rounded-full bg-gradient-to-br from-primary to-primary-container animate-pulse" />
        <span className="text-sm text-on-surface-variant tracking-wide">加载中...</span>
      </div>
    </div>
  )
}

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const token = localStorage.getItem('token')
  if (!token) return <Navigate to="/login" replace />
  return <>{children}</>
}

export default function App() {
  return (
    <Suspense fallback={<Loading />}>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="/register" element={<Register />} />
        <Route
          path="/"
          element={
            <ProtectedRoute>
              <Layout />
            </ProtectedRoute>
          }
        >
          <Route index element={<Navigate to="/dashboard" replace />} />
          <Route path="dashboard" element={<Dashboard />} />
          <Route path="nodes" element={<Nodes />} />
          <Route path="spiders" element={<Spiders />} />
          <Route path="tasks" element={<Tasks />} />
          <Route path="tasks/create" element={<CreateTask />} />
          <Route path="schedules" element={<Schedules />} />
          <Route path="accounts" element={<Accounts />} />
          <Route path="data" element={<DataCenter />} />
          <Route path="logs" element={<SystemLogs />} />
          <Route path="settings" element={<Settings />} />
        </Route>
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </Suspense>
  )
}
