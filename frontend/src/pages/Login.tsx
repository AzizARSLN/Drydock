import { useEffect, useState, type FormEvent } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { ApiError } from '../api'
import { useAuth } from '../context/auth'
import { useI18n } from '../i18n'
import type { Lang } from '../i18n'
import { Alert } from '../components/ui'

export default function Login() {
  const { t, lang, setLang } = useI18n()
  const { ready, user, needsSetup, login } = useAuth()
  const nav = useNavigate()
  const loc = useLocation()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!ready) return
    if (needsSetup) nav('/setup', { replace: true })
    else if (user) nav('/', { replace: true })
  }, [ready, needsSetup, user, nav])

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      await login(username.trim(), password)
      const from = (loc.state as { from?: string } | null)?.from ?? '/'
      nav(from, { replace: true })
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) setError(t('login.invalid'))
      else if (err instanceof ApiError && err.status === 0) setError(t('err.network'))
      else setError(err instanceof ApiError ? err.message : t('err.network'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="auth-page">
      <div className="auth-bg" />
      <form className="auth-card" onSubmit={submit}>
        <div className="auth-lang">
          {(['tr', 'en'] as Lang[]).map((l) => (
            <button
              key={l}
              type="button"
              className={`lang-pill${lang === l ? ' active' : ''}`}
              onClick={() => setLang(l)}
            >
              {l === 'tr' ? 'Türkçe' : 'English'}
            </button>
          ))}
        </div>

        <div className="auth-head">
          <div className="brand-mark">DD</div>
          <h1>{t('login.title')}</h1>
          <p>{t('login.subtitle')}</p>
        </div>

        {error ? (
          <div className="mb-12">
            <Alert kind="err">{error}</Alert>
          </div>
        ) : null}

        <div className="field">
          <label>{t('login.username')}</label>
          <input
            className="input"
            value={username}
            autoComplete="username"
            autoFocus
            onChange={(e) => setUsername(e.target.value)}
          />
        </div>
        <div className="field">
          <label>{t('login.password')}</label>
          <input
            className="input"
            type="password"
            value={password}
            autoComplete="current-password"
            onChange={(e) => setPassword(e.target.value)}
          />
        </div>

        <button className="btn btn-primary" style={{ width: '100%' }} disabled={busy || !username || !password}>
          {busy ? t('login.signingIn') : t('login.signIn')}
        </button>

        <div className="auth-foot">{t('login.serverNote')}</div>
      </form>
    </div>
  )
}
