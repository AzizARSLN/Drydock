import { useState } from 'react'
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { api, ApiError } from '../api'
import { useAuth } from '../context/auth'
import { useToast } from '../context/toast'
import { useI18n, type Key, type Lang } from '../i18n'
import { Modal } from './Modal'
import { ThemeToggle } from './ThemeToggle'
import { Alert, Badge } from './ui'

type IconName = 'dashboard' | 'users' | 'audit' | 'logout' | 'collapse'

function Icon({ name }: { name: IconName }) {
  const props = {
    width: 15,
    height: 15,
    viewBox: '0 0 16 16',
    fill: 'none',
    stroke: 'currentColor',
    strokeWidth: 1.4,
    strokeLinecap: 'round' as const,
    strokeLinejoin: 'round' as const,
  }
  switch (name) {
    case 'dashboard':
      return (
        <svg {...props}>
          <rect x="2" y="2" width="5" height="5" rx="1" />
          <rect x="9" y="2" width="5" height="5" rx="1" />
          <rect x="2" y="9" width="5" height="5" rx="1" />
          <rect x="9" y="9" width="5" height="5" rx="1" />
        </svg>
      )
    case 'users':
      return (
        <svg {...props}>
          <circle cx="6" cy="5.5" r="2.4" />
          <path d="M1.6 13.4c0-2.2 2-3.6 4.4-3.6s4.4 1.4 4.4 3.6" />
          <path d="M11 4.2a2.2 2.2 0 0 1 0 4.3M12 13.4c0-1.6-.6-2.7-1.6-3.4" />
        </svg>
      )
    case 'audit':
      return (
        <svg {...props}>
          <path d="M3 2.5h10v11H3z" />
          <path d="M5.5 5.5h5M5.5 8h5M5.5 10.5h3" />
        </svg>
      )
    case 'logout':
      return (
        <svg {...props}>
          <path d="M6.5 2.5H3.5v11h3M10 5.5 12.5 8 10 10.5M12.5 8H6.5" />
        </svg>
      )
    case 'collapse':
      return (
        <svg {...props}>
          <path d="M10 3.5 6 8l4 4.5" />
        </svg>
      )
  }
}

const roleKey: Record<string, Key> = {
  admin: 'users.role.admin',
  operator: 'users.role.operator',
  viewer: 'users.role.viewer',
}

export default function Layout() {
  const { t, lang, setLang } = useI18n()
  const { user, logout } = useAuth()
  const toast = useToast()
  const nav = useNavigate()
  const loc = useLocation()
  const [collapsed, setCollapsed] = useState(false)
  const [pwOpen, setPwOpen] = useState(false)

  const crumb = () => {
    if (loc.pathname.startsWith('/servers/')) return t('server.title')
    if (loc.pathname.startsWith('/users')) return t('nav.users')
    if (loc.pathname.startsWith('/audit')) return t('nav.audit')
    return t('nav.dashboard')
  }

  const doLogout = async () => {
    await logout()
    nav('/login', { replace: true })
  }

  const navItem = (to: string, end: boolean, icon: IconName, label: string) => (
    <NavLink to={to} end={end} className={({ isActive }) => `nav-item${isActive ? ' active' : ''}`}>
      <span className="icon">
        <Icon name={icon} />
      </span>
      {!collapsed ? label : null}
    </NavLink>
  )

  return (
    <div className="app-shell">
      <aside className={`sidebar${collapsed ? ' collapsed' : ''}`}>
        <div className="brand">
          <div className="brand-mark">DD</div>
          {!collapsed ? (
            <div className="brand-text">
              <strong>Drydock</strong>
              <span>{t('login.subtitle')}</span>
            </div>
          ) : null}
        </div>

        <nav className="nav">
          {navItem('/', true, 'dashboard', t('nav.dashboard'))}
          {user?.role === 'admin' ? (
            <>
              <div className="nav-section">{t('users.title')}</div>
              {navItem('/users', false, 'users', t('nav.users'))}
            </>
          ) : null}
          <div className="nav-section">{t('audit.title')}</div>
          {navItem('/audit', false, 'audit', t('nav.audit'))}
        </nav>

        <div className="sidebar-foot">
          <button className="btn btn-ghost btn-sm" onClick={() => setCollapsed((c) => !c)}>
            <Icon name="collapse" />
            {!collapsed ? 'Dock' : null}
          </button>
        </div>
      </aside>

      <div className="main">
        <header className="topbar">
          <div className="crumbs">
            <strong>{crumb()}</strong>
          </div>
          <div className="spacer" />
          <ThemeToggle />
          <div className="toggle">
            {(['tr', 'en'] as Lang[]).map((l) => (
              <button key={l} className={lang === l ? 'active' : ''} onClick={() => setLang(l)}>
                {l.toUpperCase()}
              </button>
            ))}
          </div>
          <div className="flex">
            <span className="fs-12 dim">{user?.full_name || user?.username}</span>
            <Badge kind={user?.role === 'admin' ? 'info' : ''}>{t(roleKey[user?.role ?? 'viewer'])}</Badge>
          </div>
          <button className="btn btn-ghost btn-sm" onClick={() => setPwOpen(true)}>
            {t('nav.password')}
          </button>
          <button className="btn btn-ghost btn-sm" onClick={doLogout}>
            <Icon name="logout" />
            {t('nav.logout')}
          </button>
        </header>

        <main className="content">
          <Outlet />
        </main>
      </div>

      {pwOpen ? (
        <PasswordModal
          onClose={() => setPwOpen(false)}
          onDone={() => toast.ok(t('auth.pwChanged'))}
        />
      ) : null}
    </div>
  )
}

function PasswordModal({ onClose, onDone }: { onClose: () => void; onDone: () => void }) {
  const { t } = useI18n()
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [confirm, setConfirm] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const submit = async () => {
    if (next.length < 8) {
      setError(t('setup.minLength'))
      return
    }
    if (next !== confirm) {
      setError(t('setup.passwordMismatch'))
      return
    }
    setBusy(true)
    setError('')
    try {
      await api.changePassword(current, next)
      onDone()
      onClose()
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t('err.network'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      title={t('nav.password')}
      onClose={onClose}
      footer={
        <>
          <button className="btn" onClick={onClose} disabled={busy}>
            {t('common.cancel')}
          </button>
          <button className="btn btn-primary" onClick={submit} disabled={busy}>
            {t('common.save')}
          </button>
        </>
      }
    >
      {error ? <Alert kind="err">{error}</Alert> : null}
      <div className="field">
        <label>{t('users.currentPassword')}</label>
        <input className="input" type="password" value={current} onChange={(e) => setCurrent(e.target.value)} />
      </div>
      <div className="field">
        <label>{t('users.newPassword')}</label>
        <input className="input" type="password" value={next} onChange={(e) => setNext(e.target.value)} />
      </div>
      <div className="field">
        <label>{t('setup.confirmPassword')}</label>
        <input className="input" type="password" value={confirm} onChange={(e) => setConfirm(e.target.value)} />
      </div>
    </Modal>
  )
}
