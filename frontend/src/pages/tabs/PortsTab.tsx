import { useCallback, useEffect, useState } from 'react'
import { api, ApiError, type ListeningPort, type NginxSite } from '../../api'
import { useDryRun } from '../../components/DryRun'
import { Alert, Badge, Empty, Loading } from '../../components/ui'
import { useI18n } from '../../i18n'
import type { TabProps } from '../ServerDetail'

function firstPort(site: NginxSite | undefined): string {
  if (!site) return ''
  for (const l of site.listen) {
    const m = /(\d+)/.exec(l)
    if (m) return m[1]
  }
  return ''
}

export default function PortsTab({ serverId, canWrite }: TabProps) {
  const { t } = useI18n()
  const dry = useDryRun()
  const [ports, setPorts] = useState<ListeningPort[] | null>(null)
  const [sites, setSites] = useState<NginxSite[]>([])
  const [error, setError] = useState('')

  const [path, setPath] = useState('')
  const [oldPort, setOldPort] = useState('')
  const [newPort, setNewPort] = useState('')

  const load = useCallback(async () => {
    try {
      const [p, n] = await Promise.all([api.ports(serverId), api.nginx(serverId)])
      setPorts(p)
      setSites(n.sites ?? [])
      if (!path && n.sites && n.sites.length > 0) {
        setPath(n.sites[0].path)
        setOldPort(firstPort(n.sites[0]) || '80')
      }
      setError('')
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t('err.network'))
    }
  }, [serverId, t, path])

  useEffect(() => {
    void load()
  }, [load])

  const pickSite = (p: string) => {
    setPath(p)
    const site = sites.find((s) => s.path === p)
    setOldPort(firstPort(site) || '80')
  }

  const change = () => {
    const oldN = Number(oldPort)
    const newN = Number(newPort)
    dry.preview({
      title: `${t('ports.change')} — ${path}`,
      run: (d) => api.portChange(serverId, path, oldN, newN, d),
      onApplied: () => {
        setNewPort('')
        void load()
      },
    })
  }

  return (
    <>
      <div className="card mb-16">
        <div className="card-header">
          <h3>{t('ports.change')}</h3>
          <div className="spacer" />
          <span className="fs-11 muted">{t('nginx.autoBackup')}</span>
        </div>
        <div className="card-body">
          <div className="row">
            <div className="field" style={{ flex: 2 }}>
              <label>{t('ports.configFile')}</label>
              <select className="select mono" value={path} onChange={(e) => pickSite(e.target.value)}>
                {sites.length === 0 ? <option value="">{t('nginx.empty')}</option> : null}
                {sites.map((s) => (
                  <option key={s.path} value={s.path}>
                    {s.path}
                  </option>
                ))}
              </select>
            </div>
            <div className="field">
              <label>{t('ports.oldPort')}</label>
              <input className="input mono" value={oldPort} onChange={(e) => setOldPort(e.target.value)} />
            </div>
            <div className="field">
              <label>{t('ports.newPort')}</label>
              <input className="input mono" value={newPort} onChange={(e) => setNewPort(e.target.value)} />
            </div>
          </div>
          <button
            className="btn btn-primary"
            onClick={change}
            disabled={!canWrite || !path || !oldPort || !newPort}
          >
            {t('common.dryRun')}
          </button>
        </div>
      </div>

      {error ? <Alert kind="err">{error}</Alert> : null}

      <div className="card">
        <div className="card-header">
          <h3>{t('ports.listening')}</h3>
          <div className="spacer" />
          <button className="btn btn-sm btn-ghost" onClick={load}>
            {t('common.refresh')}
          </button>
        </div>
        {ports === null ? (
          <Loading />
        ) : ports.length === 0 ? (
          <Empty title={t('ports.empty')} />
        ) : (
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th>{t('ports.proto')}</th>
                  <th>{t('ports.address')}</th>
                  <th>{t('ports.port')}</th>
                  <th>{t('ports.process')}</th>
                </tr>
              </thead>
              <tbody>
                {ports.map((p, i) => (
                  <tr key={`${p.proto}-${p.address}-${p.port}-${i}`}>
                    <td>
                      <Badge>{p.proto}</Badge>
                    </td>
                    <td className="mono">{p.address}</td>
                    <td className="mono">{p.port}</td>
                    <td className="fs-11">{p.process || '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {dry.modal}
    </>
  )
}
