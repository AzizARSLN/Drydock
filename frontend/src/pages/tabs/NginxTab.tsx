import { useCallback, useEffect, useState } from 'react'
import { api, ApiError, type NginxOverview, type NginxSite } from '../../api'
import { useDryRun } from '../../components/DryRun'
import { Modal } from '../../components/Modal'
import { Alert, Badge, Empty, KV, Loading } from '../../components/ui'
import { useToast } from '../../context/toast'
import { useI18n } from '../../i18n'
import type { TabProps } from '../ServerDetail'

export default function NginxTab({ serverId, canWrite }: TabProps) {
  const { t } = useI18n()
  const toast = useToast()
  const dry = useDryRun()

  const [data, setData] = useState<NginxOverview | null>(null)
  const [error, setError] = useState('')
  const [editor, setEditor] = useState<{ path: string; content: string } | null>(null)
  const [editorBusy, setEditorBusy] = useState(false)
  const [testOut, setTestOut] = useState<string | null>(null)

  const load = useCallback(async () => {
    try {
      setData(await api.nginx(serverId))
      setError('')
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t('err.network'))
    }
  }, [serverId, t])

  useEffect(() => {
    void load()
  }, [load])

  const openEditor = async (site: NginxSite) => {
    setEditorBusy(true)
    try {
      const res = await api.nginxConfig(serverId, site.path)
      setEditor({ path: res.path, content: res.content })
    } catch (e) {
      toast.err(e instanceof ApiError ? e.message : t('err.network'))
    } finally {
      setEditorBusy(false)
    }
  }

  const saveConfig = () => {
    if (!editor) return
    const path = editor.path
    const content = editor.content
    dry.preview({
      title: `${t('nginx.editConfig')} — ${path}`,
      run: (d) => api.nginxSave(serverId, path, content, d),
      onApplied: () => {
        setEditor(null)
        void load()
      },
    })
  }

  const toggleSite = (site: NginxSite) => {
    dry.preview({
      title: site.enabled ? t('nginx.disable') : t('nginx.enable'),
      run: (d) => (site.enabled ? api.nginxDisable(serverId, site.name, d) : api.nginxEnable(serverId, site.name, d)),
      onApplied: () => void load(),
    })
  }

  const testConfig = async () => {
    try {
      const res = await api.nginxTest(serverId)
      setTestOut(res.output)
      if (res.ok) toast.ok(t('nginx.testConfig'))
    } catch (e) {
      toast.err(e instanceof ApiError ? e.message : t('err.network'))
    }
  }

  const reload = () => {
    dry.preview({
      title: t('nginx.reload'),
      run: (d) => api.nginxReload(serverId, d),
      onApplied: () => void load(),
    })
  }

  if (error) return <Alert kind="err">{error}</Alert>
  if (!data) return <Loading />

  return (
    <>
      <div className="toolbar mb-12">
        <Badge kind={data.installed ? 'ok' : 'danger'}>
          {data.installed ? t('nginx.installed') : t('nginx.notInstalled')}
        </Badge>
        {data.installed ? (
          <>
            <Badge>{`${t('nginx.version')}: ${data.version || '—'}`}</Badge>
            <Badge kind={data.state === 'active' ? 'ok' : 'danger'}>
              {`${t('nginx.state')}: ${data.state || '—'}`}
            </Badge>
          </>
        ) : null}
        <div className="spacer" />
        <button className="btn btn-sm" onClick={testConfig} disabled={!canWrite}>
          {t('nginx.testConfig')}
        </button>
        <button className="btn btn-sm" onClick={reload} disabled={!canWrite}>
          {t('nginx.reload')}
        </button>
        <button className="btn btn-sm btn-ghost" onClick={load}>
          {t('common.refresh')}
        </button>
      </div>

      {!data.installed ? (
        <div className="card">
          <Empty title={t('nginx.notInstalled')} />
        </div>
      ) : data.sites.length === 0 ? (
        <div className="card">
          <Empty title={t('nginx.empty')} />
        </div>
      ) : (
        <div className="card">
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th>{t('common.name')}</th>
                  <th>{t('nginx.serverName')}</th>
                  <th>{t('nginx.listen')}</th>
                  <th>SSL</th>
                  <th>{t('common.status')}</th>
                  <th className="right">{t('common.actions')}</th>
                </tr>
              </thead>
              <tbody>
                {data.sites.map((site) => (
                  <tr key={site.path}>
                    <td className="mono">{site.name}</td>
                    <td className="mono">{site.server_name || '—'}</td>
                    <td className="mono">{(site.listen ?? []).join(', ') || '—'}</td>
                    <td>
                      {site.ssl ? <Badge kind="ok">SSL</Badge> : <span className="muted">—</span>}
                    </td>
                    <td>
                      <Badge kind={site.enabled ? 'ok' : ''}>
                        {site.enabled ? t('nginx.enabled') : t('nginx.disabled')}
                      </Badge>
                    </td>
                    <td className="right">
                      <div className="flex" style={{ justifyContent: 'flex-end' }}>
                        <button className="btn btn-xs" onClick={() => openEditor(site)} disabled={editorBusy}>
                          {t('common.edit')}
                        </button>
                        <button className="btn btn-xs" onClick={() => toggleSite(site)} disabled={!canWrite}>
                          {site.enabled ? t('nginx.disable') : t('nginx.enable')}
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

      <div className="fs-11 muted mt-12">{t('nginx.autoBackup')}</div>

      {editor ? (
        <Modal
          wide
          title={`${t('nginx.editConfig')}`}
          onClose={() => setEditor(null)}
          footer={
            <>
              <button className="btn" onClick={() => setEditor(null)}>
                {t('common.close')}
              </button>
              <button className="btn btn-primary" onClick={saveConfig} disabled={!canWrite}>
                {t('common.dryRun')}
              </button>
            </>
          }
        >
          <div className="kv mb-12">
            <KV k={t('ports.configFile')} v={editor.path} />
          </div>
          <textarea
            className="code-editor"
            value={editor.content}
            spellCheck={false}
            onChange={(e) => setEditor({ ...editor, content: e.target.value })}
          />
        </Modal>
      ) : null}

      {testOut !== null ? (
        <Modal
          title={t('nginx.testConfig')}
          onClose={() => setTestOut(null)}
          footer={
            <button className="btn" onClick={() => setTestOut(null)}>
              {t('common.close')}
            </button>
          }
        >
          <pre className="code">{testOut || '—'}</pre>
        </Modal>
      ) : null}

      {dry.modal}
    </>
  )
}
