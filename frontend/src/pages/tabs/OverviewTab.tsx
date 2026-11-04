import { useCallback, useEffect, useState } from 'react'
import { api, ApiError, type HealthReport } from '../../api'
import { Alert, Badge, Empty, KV, Loading, Spinner, levelClass } from '../../components/ui'
import { useI18n } from '../../i18n'
import type { TabProps } from '../ServerDetail'

export default function OverviewTab({ serverId }: TabProps) {
  const { t } = useI18n()
  const [report, setReport] = useState<HealthReport | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    setBusy(true)
    setError('')
    try {
      setReport(await api.health(serverId))
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t('err.network'))
    } finally {
      setBusy(false)
    }
  }, [serverId, t])

  useEffect(() => {
    void load()
  }, [load])

  if (error) return <Alert kind="err">{error}</Alert>
  if (!report) return <Loading />

  if (!report.reachable) {
    return (
      <div>
        <div className="toolbar mb-12">
          <Badge kind="danger">{t('dash.unreachable')}</Badge>
          <div className="spacer" />
          <button className="btn btn-sm" onClick={load} disabled={busy}>
            {busy ? <Spinner /> : null}
            {t('health.checkNow')}
          </button>
        </div>
        <Alert kind="err">{report.error ?? t('dash.unreachable')}</Alert>
      </div>
    )
  }

  return (
    <>
      <div className="toolbar mb-12">
        <Badge kind="ok">
          <span className="dot" /> {t('dash.reachable')}
        </Badge>
        <span className="fs-11 muted">{report.checked_at}</span>
        <div className="spacer" />
        <button className="btn btn-sm" onClick={load} disabled={busy}>
          {busy ? <Spinner /> : null}
          {busy ? t('health.checking') : t('health.checkNow')}
        </button>
      </div>

      {report.failed_units.length > 0 ? (
        <div className="mb-12">
          <Alert kind="warn">
            {t('health.failedUnits')}: <span className="mono">{report.failed_units.join(', ')}</span>
          </Alert>
        </div>
      ) : null}

      <div className="grid cols-4 mb-16">
        <div className="card stat">
          <div className="label">{t('health.cpu')}</div>
          <div className="value">{report.cpu_percent.toFixed(1)}%</div>
          <div className={`bar ${levelClass(report.cpu_percent)} mt-8`}>
            <div style={{ width: `${Math.min(report.cpu_percent, 100)}%` }} />
          </div>
        </div>
        <div className="card stat">
          <div className="label">{t('health.memory')}</div>
          <div className="value">{report.mem_percent.toFixed(0)}%</div>
          <div className={`bar ${levelClass(report.mem_percent)} mt-8`}>
            <div style={{ width: `${Math.min(report.mem_percent, 100)}%` }} />
          </div>
          <div className="fs-11 muted mt-8">
            {report.mem_used_mb.toFixed(0)} / {report.mem_total_mb.toFixed(0)} MB
          </div>
        </div>
        <div className="card stat">
          <div className="label">{t('health.load')}</div>
          <div className="value" style={{ fontSize: 16 }}>
            {report.load_avg}
          </div>
        </div>
        <div className="card stat">
          <div className="label">{t('health.uptime')}</div>
          <div className="value" style={{ fontSize: 16 }}>
            {report.uptime}
          </div>
        </div>
      </div>

      <div className="grid cols-2">
        <div className="card">
          <div className="card-header">
            <h3>{t('server.title')}</h3>
          </div>
          <div className="card-body">
            <div className="kv">
              <KV k={t('health.hostname')} v={report.hostname} />
              <KV k={t('health.os')} v={report.os} />
              <KV k={t('health.kernel')} v={report.kernel} />
              <KV
                k={t('health.nginxState')}
                v={
                  <Badge kind={report.nginx_state === 'active' ? 'ok' : 'danger'}>
                    {report.nginx_state || t('common.none')}
                  </Badge>
                }
              />
              <KV
                k={t('health.docker')}
                v={
                  <Badge kind={report.docker_on ? 'ok' : ''}>
                    {report.docker_on ? t('health.running') : t('health.down')}
                  </Badge>
                }
              />
            </div>
          </div>
        </div>

        <div className="card">
          <div className="card-header">
            <h3>{t('health.disk')}</h3>
          </div>
          <div className="card-body">
            {report.disks.length === 0 ? (
              <Empty title={t('health.noData')} />
            ) : (
              <div className="flex-col">
                {report.disks.map((d) => (
                  <div key={d.mount}>
                    <div className="flex fs-12">
                      <span className="mono">{d.mount}</span>
                      <div className="spacer" style={{ flex: 1 }} />
                      <span className="muted">
                        {d.used_gb.toFixed(1)} / {d.total_gb.toFixed(1)} GB
                      </span>
                    </div>
                    <div className={`bar ${levelClass(d.used_pct)} mt-8`}>
                      <div style={{ width: `${Math.min(d.used_pct, 100)}%` }} />
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      </div>

      {report.cert_expiring.length > 0 ? (
        <div className="card mt-16">
          <div className="card-header">
            <h3>{t('health.certExpiring')}</h3>
          </div>
          <div className="card-body">
            <div className="flex-col">
              {report.cert_expiring.map((c) => (
                <div key={c.path} className="flex">
                  <span className="mono">{c.domain || c.path}</span>
                  <div className="spacer" style={{ flex: 1 }} />
                  <Badge kind={c.days_left <= 7 ? 'danger' : 'warn'}>
                    {c.days_left} {t('health.days')}
                  </Badge>
                </div>
              ))}
            </div>
          </div>
        </div>
      ) : null}
    </>
  )
}
