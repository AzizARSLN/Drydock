import { useCallback, useEffect, useState } from 'react'
import { api, ApiError, type Service } from '../../api'
import { useDryRun } from '../../components/DryRun'
import { Modal } from '../../components/Modal'
import { Alert, Badge, Empty, Loading, Spinner } from '../../components/ui'
import { useI18n } from '../../i18n'
import type { TabProps } from '../ServerDetail'

type Kind = 'all' | 'docker' | 'systemd'

// composePath extracts the compose file from the "compose: <file> (<service>)"
// detail string produced by the backend listing.
function composePath(detail: string): string {
  const m = /^compose:\s*(.+?)\s*\(/.exec(detail)
  return m ? m[1] : ''
}

export default function AppsTab({ serverId, canWrite }: TabProps) {
  const { t } = useI18n()
  const dry = useDryRun()
  const [kind, setKind] = useState<Kind>('all')
  const [filter, setFilter] = useState('')
  const [services, setServices] = useState<Service[] | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [update, setUpdate] = useState<Service | null>(null)

  const load = useCallback(
    async (k: Kind, f: string, showSpinner: boolean) => {
      if (showSpinner) setBusy(true)
      try {
        setServices(await api.services(serverId, k === 'all' ? '' : k, f))
        setError('')
      } catch (e) {
        setError(e instanceof ApiError ? e.message : t('err.network'))
      } finally {
        if (showSpinner) setBusy(false)
      }
    },
    [serverId, t],
  )

  useEffect(() => {
    const id = window.setTimeout(() => void load(kind, filter, true), 300)
    return () => window.clearTimeout(id)
  }, [kind, filter, load])

  const action = (svc: Service, act: string) => {
    dry.preview({
      title: `${svc.name} — ${act}`,
      run: (d) => api.serviceAction(serverId, svc.kind, svc.name, act, d),
      onApplied: () => void load(kind, filter, false),
    })
  }

  const runUpdate = () => {
    if (!update) return
    const svc = update
    dry.preview({
      title: `${t('apps.update')} — ${svc.name}`,
      run: (d) => api.serviceUpdate(serverId, svc.kind, svc.name, svc.image, composePath(svc.detail), d),
      onApplied: () => {
        setUpdate(null)
        void load(kind, filter, false)
      },
    })
  }

  return (
    <>
      <div className="toolbar mb-12">
        <div className="toggle">
          {(['all', 'docker', 'systemd'] as Kind[]).map((k) => (
            <button key={k} className={kind === k ? 'active' : ''} onClick={() => setKind(k)}>
              {k === 'all' ? t('common.all') : k === 'docker' ? t('apps.docker') : t('apps.systemd')}
            </button>
          ))}
        </div>
        <input
          className="input"
          style={{ maxWidth: 260 }}
          placeholder={t('apps.filter')}
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
        />
        {busy ? <Spinner /> : null}
        <div className="spacer" />
        <button className="btn btn-sm btn-ghost" onClick={() => load(kind, filter, true)}>
          {t('common.refresh')}
        </button>
      </div>

      {error ? <Alert kind="err">{error}</Alert> : null}

      {services === null ? (
        <Loading />
      ) : services.length === 0 ? (
        <div className="card">
          <Empty title={t('apps.empty')} />
        </div>
      ) : (
        <div className="card">
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th>{t('common.name')}</th>
                  <th>{t('apps.kind')}</th>
                  <th>{t('apps.image')}</th>
                  <th>{t('apps.version')}</th>
                  <th>{t('common.status')}</th>
                  <th className="right">{t('common.actions')}</th>
                </tr>
              </thead>
              <tbody>
                {services.map((svc) => (
                  <tr key={`${svc.kind}-${svc.name}`}>
                    <td className="mono">{svc.name}</td>
                    <td>
                      <Badge kind={svc.kind === 'docker' ? 'info' : ''}>{svc.kind}</Badge>
                    </td>
                    <td className="mono fs-11">{svc.image || '—'}</td>
                    <td className="mono fs-11">{svc.version || '—'}</td>
                    <td className="fs-11">{svc.status || '—'}</td>
                    <td className="right">
                      <div className="flex" style={{ justifyContent: 'flex-end' }}>
                        <button className="btn btn-xs" onClick={() => action(svc, 'start')} disabled={!canWrite}>
                          {t('apps.start')}
                        </button>
                        <button className="btn btn-xs" onClick={() => action(svc, 'stop')} disabled={!canWrite}>
                          {t('apps.stop')}
                        </button>
                        <button className="btn btn-xs" onClick={() => action(svc, 'restart')} disabled={!canWrite}>
                          {t('apps.restart')}
                        </button>
                        {svc.kind === 'systemd' ? (
                          <button className="btn btn-xs" onClick={() => action(svc, 'reload')} disabled={!canWrite}>
                            {t('apps.reload')}
                          </button>
                        ) : null}
                        <button className="btn btn-xs btn-primary" onClick={() => setUpdate(svc)} disabled={!canWrite}>
                          {t('apps.update')}
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {update ? (
        <UpdateModal
          service={update}
          onClose={() => setUpdate(null)}
          onConfirm={(image) => {
            setUpdate({ ...update, image })
            runUpdate()
          }}
        />
      ) : null}

      {dry.modal}
    </>
  )
}

function UpdateModal({
  service,
  onClose,
  onConfirm,
}: {
  service: Service
  onClose: () => void
  onConfirm: (image: string) => void
}) {
  const { t } = useI18n()
  const [image, setImage] = useState(service.image)

  return (
    <Modal
      title={`${t('apps.update')} — ${service.name}`}
      onClose={onClose}
      footer={
        <>
          <button className="btn" onClick={onClose}>
            {t('common.cancel')}
          </button>
          <button className="btn btn-primary" onClick={() => onConfirm(image.trim())} disabled={!image.trim()}>
            {t('common.dryRun')}
          </button>
        </>
      }
    >
      {service.kind === 'docker' ? (
        <div className="field">
          <label>{t('apps.image')}</label>
          <input className="input mono" value={image} onChange={(e) => setImage(e.target.value)} />
        </div>
      ) : (
        <Alert kind="info">{t('apps.update')}</Alert>
      )}
    </Modal>
  )
}
