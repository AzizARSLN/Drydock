import { useCallback, useEffect, useRef, useState } from 'react'
import { api, ApiError, type FileEntry } from '../../api'
import { useDryRun } from '../../components/DryRun'
import { Modal } from '../../components/Modal'
import { Alert, Empty, Loading, fmtBytes } from '../../components/ui'
import { useToast } from '../../context/toast'
import { useI18n } from '../../i18n'
import type { TabProps } from '../ServerDetail'

const ROOT = '/etc'

function parentOf(p: string): string {
  const trimmed = p.replace(/\/+$/, '')
  const idx = trimmed.lastIndexOf('/')
  if (idx <= 0) return '/'
  return trimmed.slice(0, idx)
}

function join(dir: string, name: string): string {
  return `${dir.replace(/\/+$/, '')}/${name}`
}

function toBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => {
      const result = String(reader.result)
      resolve(result.slice(result.indexOf(',') + 1))
    }
    reader.onerror = () => reject(reader.error)
    reader.readAsDataURL(file)
  })
}

export default function FilesTab({ serverId, canWrite }: TabProps) {
  const { t } = useI18n()
  const toast = useToast()
  const dry = useDryRun()

  const [cwd, setCwd] = useState(ROOT)
  const [entries, setEntries] = useState<FileEntry[] | null>(null)
  const [error, setError] = useState('')
  const [sel, setSel] = useState<{ path: string; name: string; content: string } | null>(null)
  const [loadingFile, setLoadingFile] = useState(false)
  const [mkdirOpen, setMkdirOpen] = useState(false)
  const [renameOpen, setRenameOpen] = useState(false)
  const fileRef = useRef<HTMLInputElement>(null)

  const open = useCallback(
    async (path: string) => {
      try {
        const res = await api.files(serverId, path)
        setEntries(res.entries ?? [])
        setCwd(res.path)
        setError('')
      } catch (e) {
        setError(e instanceof ApiError ? e.message : t('err.network'))
        setEntries([])
      }
    },
    [serverId, t],
  )

  useEffect(() => {
    void open(ROOT)
  }, [open])

  const openFile = async (entry: FileEntry) => {
    setLoadingFile(true)
    try {
      const res = await api.fileContent(serverId, entry.path)
      setSel({ path: res.path, name: entry.name, content: res.content })
    } catch (e) {
      toast.err(e instanceof ApiError ? e.message : t('err.network'))
    } finally {
      setLoadingFile(false)
    }
  }

  const save = () => {
    if (!sel) return
    const { path, content } = sel
    dry.preview({
      title: `${t('files.editor')} — ${path}`,
      run: (d) => api.fileSave(serverId, path, content, d),
      onApplied: () => void open(cwd),
    })
  }

  const remove = (entry: FileEntry) => {
    dry.preview({
      title: `${t('common.delete')} — ${entry.name}`,
      confirmWord: entry.name,
      run: (d) => api.fileDelete(serverId, entry.path, entry.is_dir, d, entry.name),
      onApplied: () => {
        if (sel?.path === entry.path) setSel(null)
        void open(cwd)
      },
    })
  }

  const download = async (target: { path: string; name: string }) => {
    try {
      const res = await api.fileDownload(serverId, target.path)
      const bin = atob(res.content_base64)
      const bytes = new Uint8Array(bin.length)
      for (let i = 0; i < bin.length; i += 1) bytes[i] = bin.charCodeAt(i)
      const url = URL.createObjectURL(new Blob([bytes]))
      const a = document.createElement('a')
      a.href = url
      a.download = target.name
      a.click()
      URL.revokeObjectURL(url)
    } catch (e) {
      toast.err(e instanceof ApiError ? e.message : t('err.network'))
    }
  }

  const upload = async (file: File) => {
    try {
      const b64 = await toBase64(file)
      const target = join(cwd, file.name)
      dry.preview({
        title: `${t('files.upload')} — ${target}`,
        run: (d) => api.fileUpload(serverId, target, b64, d),
        onApplied: () => void open(cwd),
      })
    } catch {
      toast.err(t('err.network'))
    }
  }

  const doMkdir = (name: string) => {
    setMkdirOpen(false)
    const target = join(cwd, name)
    dry.preview({
      title: `${t('files.newFolder')} — ${target}`,
      run: (d) => api.fileMkdir(serverId, target, d),
      onApplied: () => void open(cwd),
    })
  }

  const doRename = (newName: string) => {
    if (!sel) return
    setRenameOpen(false)
    const to = join(parentOf(sel.path), newName)
    const from = sel.path
    dry.preview({
      title: `${t('common.edit')} — ${sel.name}`,
      run: (d) => api.fileMove(serverId, from, to, d),
      onApplied: () => {
        setSel(null)
        void open(cwd)
      },
    })
  }

  return (
    <>
      {error ? <Alert kind="err">{error}</Alert> : null}

      <div className="split">
        <div className="card">
          <div className="card-header">
            <h3>{t('files.browse')}</h3>
            <div className="spacer" />
            <button className="btn btn-xs" onClick={() => void open(parentOf(cwd))} title={t('files.parent')}>
              ↑
            </button>
            <button className="btn btn-xs" onClick={() => void open(cwd)}>
              {t('common.refresh')}
            </button>
          </div>
          <div className="card-body" style={{ padding: '8px 12px' }}>
            <div className="chip mono" style={{ maxWidth: '100%', overflow: 'hidden' }}>
              {cwd}
            </div>
            <div className="toolbar mt-8">
              <button className="btn btn-xs" onClick={() => setMkdirOpen(true)} disabled={!canWrite}>
                {t('files.newFolder')}
              </button>
              <button className="btn btn-xs" onClick={() => fileRef.current?.click()} disabled={!canWrite}>
                {t('files.upload')}
              </button>
              <input
                ref={fileRef}
                type="file"
                style={{ display: 'none' }}
                onChange={(e) => {
                  const f = e.target.files?.[0]
                  if (f) void upload(f)
                  e.target.value = ''
                }}
              />
            </div>
          </div>
          {entries === null ? (
            <Loading />
          ) : entries.length === 0 ? (
            <Empty title={t('files.empty')} />
          ) : (
            <div className="list">
              {entries.map((entry) => (
                <div
                  key={entry.path}
                  className={`list-item${sel?.path === entry.path ? ' active' : ''}`}
                  onClick={() => (entry.is_dir ? void open(entry.path) : void openFile(entry))}
                >
                  <span
                    className="mono"
                    style={{ width: 14, color: entry.is_dir ? 'var(--accent)' : 'var(--text-muted)' }}
                  >
                    {entry.is_dir ? '▸' : '·'}
                  </span>
                  <span className="grow mono">{entry.name}</span>
                  <span className="fs-11 muted">{entry.is_dir ? '' : fmtBytes(entry.size)}</span>
                  <button
                    className="btn btn-xs btn-ghost"
                    onClick={(e) => {
                      e.stopPropagation()
                      remove(entry)
                    }}
                    disabled={!canWrite}
                  >
                    ✕
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>

        <div className="card">
          <div className="card-header">
            <h3>{t('files.editor')}</h3>
            <div className="spacer" />
            {sel ? (
              <>
                <span className="fs-11 muted mono">{sel.path}</span>
                <button className="btn btn-xs" onClick={() => setRenameOpen(true)} disabled={!canWrite}>
                  {t('common.edit')}
                </button>
                <button className="btn btn-xs" onClick={() => void download({ path: sel.path, name: sel.name })}>
                  {t('files.download')}
                </button>
                <button className="btn btn-xs btn-primary" onClick={save} disabled={!canWrite}>
                  {t('common.dryRun')}
                </button>
              </>
            ) : null}
          </div>
          <div className="card-body">
            {loadingFile ? (
              <Loading />
            ) : sel ? (
              <textarea
                className="code-editor"
                style={{ minHeight: 460 }}
                value={sel.content}
                spellCheck={false}
                onChange={(e) => setSel({ ...sel, content: e.target.value })}
              />
            ) : (
              <Empty title={t('files.editor')} hint={t('files.browse')} />
            )}
          </div>
        </div>
      </div>

      {mkdirOpen ? (
        <NameModal
          title={t('files.newFolder')}
          label={t('common.name')}
          onClose={() => setMkdirOpen(false)}
          onSubmit={doMkdir}
        />
      ) : null}

      {renameOpen && sel ? (
        <NameModal
          title={t('common.edit')}
          label={t('common.name')}
          initial={sel.name}
          onClose={() => setRenameOpen(false)}
          onSubmit={doRename}
        />
      ) : null}

      {dry.modal}
    </>
  )
}

function NameModal({
  title,
  label,
  initial,
  onClose,
  onSubmit,
}: {
  title: string
  label: string
  initial?: string
  onClose: () => void
  onSubmit: (value: string) => void
}) {
  const { t } = useI18n()
  const [value, setValue] = useState(initial ?? '')
  return (
    <Modal
      title={title}
      onClose={onClose}
      footer={
        <>
          <button className="btn" onClick={onClose}>
            {t('common.cancel')}
          </button>
          <button className="btn btn-primary" onClick={() => onSubmit(value.trim())} disabled={!value.trim()}>
            {t('common.dryRun')}
          </button>
        </>
      }
    >
      <div className="field">
        <label>{label}</label>
        <input className="input mono" value={value} autoFocus onChange={(e) => setValue(e.target.value)} />
      </div>
    </Modal>
  )
}
