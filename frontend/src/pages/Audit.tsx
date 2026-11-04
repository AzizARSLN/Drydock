import { useCallback, useEffect, useState } from 'react'
import { api, ApiError, type AuditEntry, type Server } from '../api'
import { Alert, Badge, Empty, Loading, PageHead, fmtDate } from '../components/ui'
import { useI18n } from '../i18n'

const LIMITS = [50, 100, 200, 500]

// statusKind maps an audit status to a badge variant.
function statusKind(status: string): '' | 'ok' | 'warn' | 'danger' | 'info' {
  if (status === 'success') return 'ok'
  if (status === 'error') return 'danger'
  if (status === 'dry-run') return 'info'
  return ''
}

export default function Audit() {
  const { t } = useI18n()
  const [entries, setEntries] = useState<AuditEntry[] | null>(null)
  const [servers, setServers] = useState<Server[]>([])
  const [serverId, setServerId] = useState(0)
  const [limit, setLimit] = useState(200)
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    try {
      setEntries(await api.audit(limit, serverId))
      setError('')
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t('err.network'))
    }
  }, [limit, serverId, t])

  useEffect(() => {
    api
      .servers()
      .then(setServers)
      .catch(() => setServers([]))
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  return (
    <>
      <PageHead title={t('audit.title')} subtitle={t('audit.subtitle')}>
        <select className="select" style={{ width: 200 }} value={serverId} onChange={(e) => setServerId(Number(e.target.value))}>
          <option value={0}>{`${t('audit.target')}: ${t('common.all')}`}</option>
          {servers.map((s) => (
            <option key={s.id} value={s.id}>
              {s.name}
            </option>
          ))}
        </select>
        <select className="select" style={{ width: 110 }} value={limit} onChange={(e) => setLimit(Number(e.target.value))}>
          {LIMITS.map((n) => (
            <option key={n} value={n}>
              {n}
            </option>
          ))}
        </select>
        <button className="btn" onClick={load}>
          {t('common.refresh')}
        </button>
      </PageHead>

      {error ? <Alert kind="err">{error}</Alert> : null}

      <div className="card">
        {entries === null ? (
          <Loading />
        ) : entries.length === 0 ? (
          <Empty title={t('audit.empty')} />
        ) : (
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th>{t('audit.time')}</th>
                  <th>{t('audit.user')}</th>
                  <th>{t('audit.action')}</th>
                  <th>{t('audit.target')}</th>
                  <th>{t('server.title')}</th>
                  <th>{t('audit.detail')}</th>
                  <th>{t('audit.ip')}</th>
                  <th>{t('audit.result')}</th>
                </tr>
              </thead>
              <tbody>
                {entries.map((e) => (
                  <tr key={e.id}>
                    <td className="fs-11 mono">{fmtDate(e.created_at)}</td>
                    <td className="fs-11">{e.username || '—'}</td>
                    <td className="mono fs-11">{e.action}</td>
                    <td className="mono fs-11">{e.target || '—'}</td>
                    <td className="fs-11">{e.server_name || '—'}</td>
                    <td className="fs-11">{e.detail || '—'}</td>
                    <td className="mono fs-11">{e.ip || '—'}</td>
                    <td>
                      <Badge kind={statusKind(e.status)}>{e.status}</Badge>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </>
  )
}
