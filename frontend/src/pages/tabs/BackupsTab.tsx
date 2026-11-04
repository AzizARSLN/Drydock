import { useCallback, useEffect, useState } from 'react'
import { api, ApiError, type Snapshot } from '../../api'
import { useDryRun } from '../../components/DryRun'
import { Alert, Badge, Empty, Loading, fmtDate } from '../../components/ui'
import { useI18n } from '../../i18n'
import type { TabProps } from '../ServerDetail'

export default function BackupsTab({ serverId, canWrite }: TabProps) {
  const { t } = useI18n()
  const dry = useDryRun()
  const [list, setList] = useState<Snapshot[] | null>(null)
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    try {
      setList(await api.snapshots(serverId))
      setError('')
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t('err.network'))
    }
  }, [serverId, t])

  useEffect(() => {
    void load()
  }, [load])

  const restore = (snap: Snapshot) => {
    dry.preview({
      title: `${t('backups.restore')} — ${snap.path}`,
      run: (d) => api.snapshotRestore(serverId, snap.id, d),
      onApplied: () => void load(),
    })
  }

  if (error) return <Alert kind="err">{error}</Alert>
  if (!list) return <Loading />

  return (
    <>
      <div className="toolbar mb-12">
        <h3>{t('backups.title')}</h3>
        <div className="spacer" />
        <button className="btn btn-sm btn-ghost" onClick={load}>
          {t('common.refresh')}
        </button>
      </div>

      {list.length === 0 ? (
        <div className="card">
          <Empty title={t('backups.empty')} />
        </div>
      ) : (
        <div className="card">
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th>{t('backups.kind')}</th>
                  <th>{t('ports.configFile')}</th>
                  <th>{t('backups.note')}</th>
                  <th>{t('backups.by')}</th>
                  <th>{t('backups.date')}</th>
                  <th className="right">{t('common.actions')}</th>
                </tr>
              </thead>
              <tbody>
                {list.map((snap) => (
                  <tr key={snap.id}>
                    <td>
                      <Badge>{snap.kind}</Badge>
                    </td>
                    <td className="mono fs-11">{snap.path}</td>
                    <td className="fs-11">{snap.note || '—'}</td>
                    <td className="fs-11">{snap.created_by || '—'}</td>
                    <td className="fs-11">{fmtDate(snap.created_at)}</td>
                    <td className="right">
                      <button className="btn btn-xs" onClick={() => restore(snap)} disabled={!canWrite}>
                        {t('backups.restore')}
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
