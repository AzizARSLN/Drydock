import { useEffect, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { api, ApiError, type Server } from '../api'
import { ServerForm } from '../components/ServerForm'
import { Alert, Badge, Loading } from '../components/ui'
import { useAuth, useCanWrite } from '../context/auth'
import { useI18n, type Key } from '../i18n'
import AppsTab from './tabs/AppsTab'
import BackupsTab from './tabs/BackupsTab'
import FilesTab from './tabs/FilesTab'
import NginxTab from './tabs/NginxTab'
import OverviewTab from './tabs/OverviewTab'
import PortsTab from './tabs/PortsTab'
import SslTab from './tabs/SslTab'
import TerminalTab from './tabs/TerminalTab'

const TABS = ['overview', 'nginx', 'ssl', 'apps', 'ports', 'files', 'terminal', 'backups'] as const
type TabID = (typeof TABS)[number]

export default function ServerDetail() {
  const { t } = useI18n()
  const { id } = useParams()
  const serverId = Number(id)
  const nav = useNavigate()
  const { user } = useAuth()
  const canWrite = useCanWrite()
  const [params, setParams] = useSearchParams()

  const [server, setServer] = useState<Server | null>(null)
  const [error, setError] = useState('')
  const [edit, setEdit] = useState(false)

  const rawTab = params.get('tab') as TabID | null
  const tab: TabID = rawTab && TABS.includes(rawTab) ? rawTab : 'overview'

  useEffect(() => {
    let alive = true
    api
      .server(serverId)
      .then((s) => {
        if (alive) setServer(s)
      })
      .catch((e) => {
        if (alive) setError(e instanceof ApiError ? e.message : t('err.network'))
      })
    return () => {
      alive = false
    }
  }, [serverId, t])

  if (error) {
    return (
      <div>
        <button className="btn btn-sm" onClick={() => nav('/')}>
          ← {t('common.back')}
        </button>
        <div className="mt-12">
          <Alert kind="err">{error}</Alert>
        </div>
      </div>
    )
  }

  if (!server) return <Loading />

  const shared = { serverId, canWrite }

  return (
    <>
      <div className="page-head">
        <div>
          <button className="btn btn-ghost btn-sm mb-12" onClick={() => nav('/')}>
            ← {t('common.back')}
          </button>
          <h1>{server.name}</h1>
          <p className="mono">
            {server.username}@{server.host}:{server.port}
          </p>
        </div>
        <div className="spacer" />
        <div className="flex">
          <Badge kind={server.auth_type === 'key' ? 'info' : 'warn'}>
            {server.auth_type === 'key' ? t('server.authKey') : t('server.authPassword')}
          </Badge>
          {user?.role === 'admin' ? (
            <button className="btn btn-sm" onClick={() => setEdit(true)}>
              {t('common.edit')}
            </button>
          ) : null}
        </div>
      </div>

      <div className="tabs mb-16">
        {TABS.map((id) => (
          <button
            key={id}
            className={`tab${tab === id ? ' active' : ''}`}
            onClick={() => setParams(id === 'overview' ? {} : { tab: id })}
          >
            {t(`tab.${id}` as Key)}
          </button>
        ))}
      </div>

      {tab === 'overview' ? <OverviewTab {...shared} /> : null}
      {tab === 'nginx' ? <NginxTab {...shared} /> : null}
      {tab === 'ssl' ? <SslTab {...shared} /> : null}
      {tab === 'apps' ? <AppsTab {...shared} /> : null}
      {tab === 'ports' ? <PortsTab {...shared} /> : null}
      {tab === 'files' ? <FilesTab {...shared} /> : null}
      {tab === 'terminal' ? <TerminalTab {...shared} /> : null}
      {tab === 'backups' ? <BackupsTab {...shared} /> : null}

      {edit ? (
        <ServerForm
          initial={server}
          onClose={() => setEdit(false)}
          onSaved={(s) => {
            setServer(s)
            setEdit(false)
          }}
        />
      ) : null}
    </>
  )
}

export interface TabProps {
  serverId: number
  canWrite: boolean
}
