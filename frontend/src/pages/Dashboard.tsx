import { useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, ApiError, type Server } from '../api'
import { ConfirmModal } from '../components/Confirm'
import { ServerForm } from '../components/ServerForm'
import { Alert, Badge, Empty, Loading, PageHead, Spinner } from '../components/ui'
import { useAuth } from '../context/auth'
import { useToast } from '../context/toast'
import { useI18n } from '../i18n'

export default function Dashboard() {
  const { t } = useI18n()
  const { user } = useAuth()
  const toast = useToast()
  const nav = useNavigate()
  const isAdmin = user?.role === 'admin'
  const canTest = user?.role === 'admin' || user?.role === 'operator'

  const [servers, setServers] = useState<Server[] | null>(null)
  const [error, setError] = useState('')
  const [form, setForm] = useState<{ open: boolean; server?: Server }>({ open: false })
  const [pending, setPending] = useState<Server | null>(null)
  const [busy, setBusy] = useState(false)
  const [testing, setTesting] = useState<number | null>(null)

  const load = useCallback(async () => {
    try {
      setServers(await api.servers())
      setError('')
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t('err.network'))
      setServers([])
    }
  }, [t])

  useEffect(() => {
    void load()
  }, [load])

  const test = async (s: Server) => {
    setTesting(s.id)
    try {
      const res = await api.testServer(s.id)
      toast.ok(`${t('server.testOk')} — ${res.fingerprint.slice(0, 24)}…`)
      setServers((prev) => prev?.map((x) => (x.id === s.id ? { ...x, host_key_fingerprint: res.fingerprint } : x)) ?? prev)
    } catch (e) {
      toast.err(e instanceof ApiError ? e.message : t('err.network'))
    } finally {
      setTesting(null)
    }
  }

  const remove = async () => {
    if (!pending) return
    setBusy(true)
    try {
      await api.deleteServer(pending.id)
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
      <PageHead title={t('dash.title')} subtitle={t('dash.subtitle')}>
        <span className="fs-12 muted">
          {servers ? `${servers.length} ${t('dash.servers')}` : ''}
        </span>
        {isAdmin ? (
          <button className="btn btn-primary" onClick={() => setForm({ open: true })}>
            + {t('dash.addServer')}
          </button>
        ) : null}
      </PageHead>

      {error ? <Alert kind="err">{error}</Alert> : null}

      {servers === null ? (
        <Loading />
      ) : servers.length === 0 ? (
        <div className="card">
          <Empty title={t('dash.noServers')} hint={isAdmin ? t('dash.addServer') : undefined} />
        </div>
      ) : (
        <div className="srv-grid">
          {servers.map((s) => (
            <div key={s.id} className="card">
              <div className="card-header">
                <h3>{s.name}</h3>
                <div className="spacer" />
                <Badge kind={s.auth_type === 'key' ? 'info' : 'warn'}>
                  {s.auth_type === 'key' ? t('server.authKey') : t('server.authPassword')}
                </Badge>
              </div>
              <div className="card-body">
                <div className="kv">
                  <div className="kv-row">
                    <span className="k">{t('server.host')}</span>
                    <span className="v">
                      {s.host}:{s.port}
                    </span>
                  </div>
                  <div className="kv-row">
                    <span className="k">{t('server.username')}</span>
                    <span className="v">{s.username}</span>
                  </div>
                  {s.description ? <p className="fs-12 muted">{s.description}</p> : null}
                </div>
                {s.tags ? (
                  <div className="flex wrap mt-8 gap-4">
                    {s.tags.split(',').map((tag) => (
                      <span key={tag} className="chip">
                        {tag.trim()}
                      </span>
                    ))}
                  </div>
                ) : null}
              </div>
              <div className="card-foot">
                <button className="btn btn-sm btn-primary" onClick={() => nav(`/servers/${s.id}`)}>
                  {t('dash.openServer')}
                </button>
                {canTest ? (
                  <button className="btn btn-sm" onClick={() => test(s)} disabled={testing === s.id}>
                    {testing === s.id ? <Spinner /> : null}
                    {t('common.test')}
                  </button>
                ) : null}
                <div className="spacer" />
                {isAdmin ? (
                  <>
                    <button className="btn btn-sm btn-ghost" onClick={() => setForm({ open: true, server: s })}>
                      {t('common.edit')}
                    </button>
                    <button className="btn btn-sm btn-danger" onClick={() => setPending(s)}>
                      {t('common.delete')}
                    </button>
                  </>
                ) : null}
              </div>
            </div>
          ))}
        </div>
      )}

      {form.open ? (
        <ServerForm
          initial={form.server}
          onClose={() => setForm({ open: false })}
          onSaved={(saved) => {
            setForm({ open: false })
            toast.ok(t('common.success'))
            if (form.server) {
              setServers((prev) => prev?.map((x) => (x.id === saved.id ? saved : x)) ?? null)
            } else {
              setServers((prev) => [...(prev ?? []), saved])
              nav(`/servers/${saved.id}`)
            }
          }}
        />
      ) : null}

      {pending ? (
        <ConfirmModal
          title={t('server.deleteConfirm')}
          message={t('server.deleteHint')}
          word={pending.name}
          busy={busy}
          onConfirm={remove}
          onClose={() => setPending(null)}
        />
      ) : null}
    </>
  )
}
