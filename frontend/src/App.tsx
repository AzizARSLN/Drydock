import type { ReactNode } from 'react'
import { BrowserRouter, Navigate, Route, Routes, useLocation } from 'react-router-dom'
import Layout from './components/Layout'
import { Loading } from './components/ui'
import { AuthProvider, useAuth } from './context/auth'
import { ToastProvider } from './context/toast'
import Audit from './pages/Audit'
import Dashboard from './pages/Dashboard'
import Login from './pages/Login'
import ServerDetail from './pages/ServerDetail'
import Setup from './pages/Setup'
import Users from './pages/Users'

function Guard() {
  const { ready, user, needsSetup } = useAuth()
  const loc = useLocation()
  if (!ready) {
    return (
      <div className="center">
        <Loading />
      </div>
    )
  }
  if (needsSetup) return <Navigate to="/setup" replace />
  if (!user) return <Navigate to="/login" replace state={{ from: loc.pathname }} />
  return <Layout />
}

function AdminOnly({ children }: { children: ReactNode }) {
  const { user } = useAuth()
  if (user?.role !== 'admin') return <Navigate to="/" replace />
  return <>{children}</>
}

export default function App() {
  return (
    <ToastProvider>
      <AuthProvider>
        <BrowserRouter>
          <Routes>
            <Route path="/login" element={<Login />} />
            <Route path="/setup" element={<Setup />} />
            <Route element={<Guard />}>
              <Route path="/" element={<Dashboard />} />
              <Route path="/servers/:id" element={<ServerDetail />} />
              <Route
                path="/users"
                element={
                  <AdminOnly>
                    <Users />
                  </AdminOnly>
                }
              />
              <Route path="/audit" element={<Audit />} />
              <Route path="*" element={<Navigate to="/" replace />} />
            </Route>
          </Routes>
        </BrowserRouter>
      </AuthProvider>
    </ToastProvider>
  )
}
