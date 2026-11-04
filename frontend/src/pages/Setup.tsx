import { useEffect, useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, ApiError } from '../api'
import { useAuth } from '../context/auth'
import { useI18n, type Lang } from '../i18n'
import { Alert } from '../components/ui'

export default function Setup() {
  const { t, lang, setLang } = useI18n()
  const { ready, needsSetup, completeSetup } = useAuth()
  const nav = useNavigate()
  const [username, setUsername] = useState('')
  const [fullName, setFullName] = useState('')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (ready && !needsSetup) nav('/login', { replace: true })
  }, [ready, needsSetup, nav])

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (password.length < 8) {
      setError(t('setup.minLength'))
      return
    }
    if (password !== confirm) {
      setError(t('setup.passwordMismatch'))
      return
    }
    setBusy(true)
    setError('')
    try {
      await api.setup(username.trim(), password, fullName.trim())
      completeSetup()
      nav('/login', { replace: true })
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t('err.network'))
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
          <h1>{t('setup.title')}</h1>
          <p>{t('setup.subtitle')}</p>
        </div>

        {error ? (
          <div className="mb-12">
            <Alert kind="err">{error}</Alert>
          </div>
        ) : null}

        <div className="field">
          <label>{t('setup.fullName')}</label>
          <input className="input" value={fullName} autoFocus onChange={(e) => setFullName(e.target.value)} />
        </div>
        <div className="field">
          <label>{t('login.username')}</label>
          <input
            className="input"
            value={username}
            autoComplete="username"
            onChange={(e) => setUsername(e.target.value)}
          />
        </div>
        <div className="field">
          <label>{t('login.password')}</label>
          <input
            className="input"
            type="password"
            value={password}
            autoComplete="new-password"
            onChange={(e) => setPassword(e.target.value)}
          />
        </div>
        <div className="field">
          <label>{t('setup.confirmPassword')}</label>
          <input
            className="input"
            type="password"
            value={confirm}
            autoComplete="new-password"
            onChange={(e) => setConfirm(e.target.value)}
          />
        </div>

        <button
          className="btn btn-primary"
          style={{ width: '100%' }}
          disabled={busy || !username || !password}
        >
          {busy ? t('common.loadingShort') : t('setup.createAdmin')}
        </button>

        <div className="auth-foot">{t('login.serverNote')}</div>
      </form>
    </div>
  )
}
