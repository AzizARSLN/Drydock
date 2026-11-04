import { useCallback, useEffect, useState } from 'react'
import { api, ApiError, type CertInfo } from '../../api'
import { useDryRun } from '../../components/DryRun'
import { Alert, Badge, Empty, Loading } from '../../components/ui'
import { useI18n } from '../../i18n'
import type { TabProps } from '../ServerDetail'

export default function SslTab({ serverId, canWrite }: TabProps) {
  const { t } = useI18n()
  const dry = useDryRun()
  const [certs, setCerts] = useState<CertInfo[] | null>(null)
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    try {
      setCerts(await api.certs(serverId))
      setError('')
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t('err.network'))
    }
  }, [serverId, t])

  useEffect(() => {
    void load()
  }, [load])

  const renew = (cert: CertInfo) => {
    dry.preview({
      title: `${t('ssl.renew')} — ${cert.domain}`,
      run: (d) => api.certRenew(serverId, cert.domain, d),
      onApplied: () => void load(),
    })
  }

  if (error) return <Alert kind="err">{error}</Alert>
  if (!certs) return <Loading />

  return (
    <>
      <div className="toolbar mb-12">
        <span className="fs-12 muted">{`${certs.length} ${t('ssl.domain')}`}</span>
        <div className="spacer" />
        <button className="btn btn-sm btn-ghost" onClick={load}>
          {t('common.refresh')}
        </button>
      </div>

      {certs.length === 0 ? (
        <div className="card">
          <Empty title={t('ssl.empty')} />
        </div>
      ) : (
        <div className="card">
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th>{t('ssl.domain')}</th>
                  <th>{t('ssl.issuer')}</th>
                  <th>{t('ssl.notAfter')}</th>
                  <th>{t('ssl.daysLeft')}</th>
                  <th>{t('ssl.autoRenew')}</th>
                  <th className="right">{t('common.actions')}</th>
                </tr>
              </thead>
              <tbody>
                {certs.map((c) => (
                  <tr key={c.path}>
                    <td className="mono">{c.domain || '—'}</td>
                    <td className="mono fs-11">{c.issuer || '—'}</td>
                    <td className="mono fs-11">{c.not_after || '—'}</td>
                    <td>
                      <Badge kind={c.days_left <= 7 ? 'danger' : c.days_left <= 30 ? 'warn' : 'ok'}>
                        {`${c.days_left} ${t('health.days')}`}
                      </Badge>
                    </td>
                    <td>
                      {c.auto_renew ? <Badge kind="info">{t('common.yes')}</Badge> : <span className="muted">—</span>}
                    </td>
                    <td className="right">
                      <button className="btn btn-xs" onClick={() => renew(c)} disabled={!canWrite}>
                        {t('ssl.renew')}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {dry.modal}
    </>
  )
}
