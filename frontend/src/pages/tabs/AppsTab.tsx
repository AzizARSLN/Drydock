import { useCallback, useEffect, useMemo, useState } from 'react'
import { api, ApiError, type ComposeProject, type Service } from '../../api'
import { useDryRun } from '../../components/DryRun'
import { Modal } from '../../components/Modal'
import { Alert, Badge, Empty, Loading, Spinner } from '../../components/ui'
import { useI18n } from '../../i18n'
import type { TabProps } from '../ServerDetail'

type Kind = 'all' | 'docker' | 'systemd'
type View = 'service' | 'project'

// composePath extracts the compose file from the "compose: <file> (<service>)"
// detail string produced by the backend listing.
function composePath(detail: string): string {
  const m = /^compose:\s*(.+?)\s*\(/.exec(detail)
  return m ? m[1] : ''
}

// baseName returns the last path segment, i.e. the project folder name.
function baseName(dir: string): string {
  const parts = dir.split('/').filter(Boolean)
  return parts.length ? parts[parts.length - 1] : dir
}

interface ComposeGroup {
  key: string
  name: string
  dir: string
  file: string
  project?: string
  items: Service[]
}

export default function AppsTab({ serverId, canWrite }: TabProps) {
  const { t } = useI18n()
  const dry = useDryRun()
  const [kind, setKind] = useState<Kind>('all')
  const [view, setView] = useState<View>('project')
  const [filter, setFilter] = useState('')
  const [services, setServices] = useState<Service[] | null>(null)
  const [projects, setProjects] = useState<ComposeProject[]>([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [update, setUpdate] = useState<Service | null>(null)

  const load = useCallback(
    async (k: Kind, f: string, showSpinner: boolean) => {
      if (showSpinner) setBusy(true)
      try {
        const [svc, proj] = await Promise.all([
          api.services(serverId, k === 'all' ? '' : k, f),
          api.composeProjects(serverId),
        ])
        setServices(svc)
        setProjects(proj ?? [])
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

  // Merge two sources into one folder-centric view:
  //   1. running containers, grouped by the compose working directory (folder),
  //   2. compose folders discovered by scanning the host (even when stopped).
  // Groups are keyed by their absolute directory so a container and its folder
  // collapse into the same project card. Fallback to the compose project name
  // when no directory was recorded on the container.
  const groups = useMemo<ComposeGroup[]>(() => {
    const map = new Map<string, ComposeGroup>()
    const keyOf = (s: Service) => s.working_dir || (s.project ? `project:${s.project}` : '')

    for (const s of services ?? []) {
      if (s.kind !== 'docker') continue
      const key = keyOf(s)
      if (!key) continue
      const g =
        map.get(key) ??
        ({
          key,
          name: s.working_dir ? baseName(s.working_dir) : s.project || key,
          dir: s.working_dir || '',
          file: '',
          project: s.project,
          items: [],
        } as ComposeGroup)
      g.items.push(s)
      if (!g.file && s.compose_file) g.file = s.compose_file
      if (!g.project && s.project) g.project = s.project
      map.set(key, g)
    }

    for (const p of projects) {
      const g =
        map.get(p.dir) ??
        ({ key: p.dir, name: p.name, dir: p.dir, file: '', items: [] } as ComposeGroup)
      if (!g.file) g.file = p.file
      if (!g.dir) g.dir = p.dir
      map.set(p.dir, g)
    }

    return [...map.values()].sort((a, b) => a.name.localeCompare(b.name))
  }, [services, projects])

  const loose = useMemo(
    () =>
      (services ?? []).filter(
        (s) => s.kind !== 'docker' || !(s.working_dir || s.project),
      ),
    [services],
  )

  const action = (svc: Service, act: string) => {
    dry.preview({
      title: `${svc.name} — ${act}`,
      run: (d) => api.serviceAction(serverId, svc.kind, svc.name, act, d),
      onApplied: () => void load(kind, filter, false),
    })
  }

  const projectAction = (g: ComposeGroup, act: string) => {
    dry.preview({
      title: `${g.name} — docker compose ${act}`,
      // "down" tears the project down, so it must be confirmed by name.
      confirmWord: act === 'down' ? g.name : undefined,
      run: (d) => api.composeAction(serverId, g.dir, g.file, act, d),
      onApplied: () => void load(kind, filter, false),
    })
  }

  const runUpdate = () => {
    if (!update) return
    const svc = update
    dry.preview({
      title: `${t('apps.update')} — ${svc.name}`,
      run: (d) =>
        api.serviceUpdate(serverId, svc.kind, svc.name, svc.image, svc.compose_file || composePath(svc.detail), d),
      onApplied: () => {
        setUpdate(null)
        void load(kind, filter, false)
      },
    })
  }

  const serviceTable = (items: Service[]) => (
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
          {items.map((svc) => (
            <tr key={`${svc.kind}-${svc.name}`}>
              <td className="mono">{svc.compose_service || svc.name}</td>
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
  )

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
        <div className="toggle">
          {(['project', 'service'] as View[]).map((v) => (
            <button key={v} className={view === v ? 'active' : ''} onClick={() => setView(v)}>
              {v === 'project' ? t('apps.byProject') : t('apps.byService')}
            </button>
          ))}
        </div>
        <input
          className="input"
          style={{ maxWidth: 240 }}
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
      ) : view === 'service' ? (
        <div className="card">{serviceTable(services)}</div>
      ) : (
        <div className="flex-col" style={{ gap: 16 }}>
          {groups.map((g) => (
            <div className="card" key={g.key}>
              <div className="card-header">
                <h3 className="mono">{g.name}</h3>
                <Badge>{g.items.length}</Badge>
                {g.project && g.project !== g.name ? <span className="chip">{g.project}</span> : null}
                {g.dir ? <span className="chip">{g.dir}</span> : null}
                <div className="spacer" />
                <button
                  className="btn btn-xs btn-primary"
                  onClick={() => projectAction(g, 'up')}
                  disabled={!canWrite || !g.dir}
                >
                  {t('apps.pullUp')}
                </button>
                <button
                  className="btn btn-xs"
                  onClick={() => projectAction(g, 'restart')}
                  disabled={!canWrite || !g.dir}
                >
                  {t('apps.composeRestart')}
                </button>
                <button className="btn btn-xs" onClick={() => projectAction(g, 'logs')} disabled={!g.dir}>
                  {t('apps.logs')}
                </button>
                <button className="btn btn-xs" onClick={() => projectAction(g, 'ps')} disabled={!g.dir}>
                  {t('apps.ps')}
                </button>
                <button
                  className="btn btn-xs btn-danger"
                  onClick={() => projectAction(g, 'down')}
                  disabled={!canWrite || !g.dir}
                >
                  {t('apps.down')}
                </button>
              </div>
              <div className="card-body tight">
                {g.items.length > 0 ? (
                  serviceTable(g.items)
                ) : (
                  <div className="fs-12 muted" style={{ padding: '14px 16px' }}>
                    {t('apps.noContainers')}
                  </div>
                )}
              </div>
            </div>
          ))}

          {loose.length > 0 ? (
            <div className="card">
              <div className="card-header">
                <h3>{t('apps.standalone')}</h3>
                <Badge>{loose.length}</Badge>
              </div>
              <div className="card-body tight">{serviceTable(loose)}</div>
            </div>
          ) : null}
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
