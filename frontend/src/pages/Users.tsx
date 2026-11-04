import { useCallback, useEffect, useState } from 'react'
import { api, ApiError, type User } from '../api'
import { ConfirmModal } from '../components/Confirm'
import { Modal } from '../components/Modal'
import { Alert, Badge, Empty, Loading, PageHead, fmtDate } from '../components/ui'
import { useAuth } from '../context/auth'
import { useToast } from '../context/toast'
import { useI18n, type Key } from '../i18n'

const ROLES: ('admin' | 'operator' | 'viewer')[] = ['admin', 'operator', 'viewer']

function roleLabel(role: string, t: (k: Key) => string): string {
  const key = `users.role.${role}` as Key
  return t(key)
}

export default function Users() {
  const { t } = useI18n()
  const { user } = useAuth()
  const toast = useToast()
  const [users, setUsers] = useState<User[] | null>(null)
  const [error, setError] = useState('')
  const [form, setForm] = useState<{ open: boolean; user?: User }>({ open: false })
  const [pending, setPending] = useState<User | null>(null)
  const [busy, setBusy] = useState(false)

  const load = useCallback(async () => {
    try {
      setUsers(await api.users())
      setError('')
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t('err.network'))
    }
  }, [t])

  useEffect(() => {
    void load()
  }, [load])

  const remove = async () => {
    if (!pending) return
    setBusy(true)
    try {
      await api.deleteUser(pending.id)
      toast.ok(t('common.success'))
      setPending(null)
      void load()
    } catch (e) {
      toast.err(e instanceof ApiError ? e.message : t('err.network'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <>
      <PageHead title={t('users.title')} subtitle={t('users.subtitle')}>
        <button className="btn btn-primary" onClick={() => setForm({ open: true })}>
          + {t('users.add')}
        </button>
      </PageHead>

      {error ? <Alert kind="err">{error}</Alert> : null}

      <div className="card">
        {users === null ? (
          <Loading />
        ) : users.length === 0 ? (
          <Empty title={t('users.empty')} />
        ) : (
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th>{t('login.username')}</th>
                  <th>{t('users.fullName')}</th>
                  <th>{t('users.role')}</th>
                  <th>{t('users.lastLogin')}</th>
                  <th className="right">{t('common.actions')}</th>
                </tr>
              </thead>
              <tbody>
                {users.map((u) => (
                  <tr key={u.id}>
                    <td className="mono">
                      {u.username}
                      {u.id === user?.id ? <span className="fs-11 muted"> ({t('common.you')})</span> : null}
                    </td>
                    <td>{u.full_name || '—'}</td>
                    <td>
                      <Badge kind={u.role === 'admin' ? 'info' : u.role === 'operator' ? 'ok' : ''}>
                        {roleLabel(u.role, t)}
                      </Badge>
                    </td>
                    <td className="fs-11">{u.last_login ? fmtDate(u.last_login) : '—'}</td>
                    <td className="right">
                      <div className="flex" style={{ justifyContent: 'flex-end' }}>
                        <button className="btn btn-xs" onClick={() => setForm({ open: true, user: u })}>
                          {t('common.edit')}
                        </button>
                        <button
                          className="btn btn-xs btn-danger"
                          onClick={() => setPending(u)}
                          disabled={u.id === user?.id}
                        >
                          {t('common.delete')}
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <div className="fs-11 muted mt-12">{t('users.roleHint')}</div>

      {form.open ? (
        <UserForm
          initial={form.user}
          onClose={() => setForm({ open: false })}
          onSaved={() => {
            setForm({ open: false })
            toast.ok(t('common.success'))
            void load()
          }}
        />
      ) : null}

      {pending ? (
        <ConfirmModal
          title={t('users.deleteConfirm')}
          message={`${pending.username} — ${t('users.deleteConfirm')}`}
          word={pending.username}
          busy={busy}
          onConfirm={remove}
          onClose={() => setPending(null)}
        />
      ) : null}
    </>
  )
}

function UserForm({
  initial,
  onClose,
  onSaved,
}: {
  initial?: User
  onClose: () => void
  onSaved: () => void
}) {
  const { t } = useI18n()
  const editing = Boolean(initial)
  const [username, setUsername] = useState(initial?.username ?? '')
  const [fullName, setFullName] = useState(initial?.full_name ?? '')
  const [role, setRole] = useState<'admin' | 'operator' | 'viewer'>(
    (initial?.role as 'admin' | 'operator' | 'viewer') ?? 'viewer',
  )
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const submit = async () => {
    setBusy(true)
    setError('')
    try {
      if (editing && initial) {
        await api.updateUser(initial.id, role, fullName.trim(), password)
      } else {
        await api.createUser(username.trim(), password, role, fullName.trim())
      }
      onSaved()
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t('err.network'))
    } finally {
      setBusy(false)
    }
  }

  const invalid = busy || (!editing && (!username.trim() || password.length < 8))

  return (
    <Modal
      title={editing ? `${t('common.edit')} — ${initial?.username}` : t('users.add')}
      onClose={onClose}
      footer={
        <>
          <button className="btn" onClick={onClose} disabled={busy}>
            {t('common.cancel')}
          </button>
          <button className="btn btn-primary" onClick={submit} disabled={invalid}>
            {editing ? t('common.save') : t('common.create')}
          </button>
        </>
      }
    >
      {error ? <Alert kind="err">{error}</Alert> : null}

      {!editing ? (
        <div className="field">
          <label>{t('login.username')}</label>
          <input className="input" value={username} autoFocus onChange={(e) => setUsername(e.target.value)} />
        </div>
      ) : null}

      <div className="field">
        <label>{t('users.fullName')}</label>
        <input className="input" value={fullName} onChange={(e) => setFullName(e.target.value)} />
      </div>

      <div className="field">
        <label>{t('users.role')}</label>
        <select className="select" value={role} onChange={(e) => setRole(e.target.value as typeof role)}>
          {ROLES.map((r) => (
            <option key={r} value={r}>
              {roleLabel(r, t)}
            </option>
          ))}
        </select>
      </div>

      <div className="field">
        <label>{t('users.newPassword')}</label>
        <input
          className="input"
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          placeholder={editing ? t('users.passwordHint') : ''}
        />
      </div>
    </Modal>
  )
}
