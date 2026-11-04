import { useState } from 'react'
import { api, ApiError, type Server, type ServerInput } from '../api'
import { useI18n } from '../i18n'
import { Modal } from './Modal'
import { Alert } from './ui'

// ServerForm creates or edits a managed host. Secret fields are write-only:
// leaving them blank on edit preserves the stored value.
export function ServerForm({
  initial,
  onSaved,
  onClose,
}: {
  initial?: Server
  onSaved: (server: Server) => void
  onClose: () => void
}) {
  const { t } = useI18n()
  const editing = Boolean(initial)

  const [name, setName] = useState(initial?.name ?? '')
  const [host, setHost] = useState(initial?.host ?? '')
  const [port, setPort] = useState(String(initial?.port ?? 22))
  const [username, setUsername] = useState(initial?.username ?? 'root')
  const [authType, setAuthType] = useState<'key' | 'password'>(initial?.auth_type ?? 'key')
  const [privateKey, setPrivateKey] = useState('')
  const [password, setPassword] = useState('')
  const [passphrase, setPassphrase] = useState('')
  const [sudoPassword, setSudoPassword] = useState('')
  const [tags, setTags] = useState(initial?.tags ?? '')
  const [description, setDescription] = useState(initial?.description ?? '')
  const [resetHostKey, setResetHostKey] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const secretHint = editing ? t('server.secretKeep') : undefined

  const submit = async () => {
    setBusy(true)
    setError('')
    const input: ServerInput = {
      name: name.trim(),
      host: host.trim(),
      port: Number(port) || 22,
      username: username.trim(),
      auth_type: authType,
      private_key: privateKey,
      password,
      passphrase,
      sudo_password: sudoPassword,
      tags: tags.trim(),
      description: description.trim(),
      reset_host_key: resetHostKey,
    }
    try {
      const saved = editing && initial ? await api.updateServer(initial.id, input) : await api.createServer(input)
      onSaved(saved)
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t('err.network'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      wide
      title={editing ? `${t('server.title')}: ${initial?.name}` : t('dash.addServer')}
      onClose={onClose}
      footer={
        <>
          <button className="btn" onClick={onClose} disabled={busy}>
            {t('common.cancel')}
          </button>
          <button className="btn btn-primary" onClick={submit} disabled={busy || !name || !host || !username}>
            {editing ? t('common.save') : t('common.create')}
          </button>
        </>
      }
    >
      {error ? <Alert kind="err">{error}</Alert> : null}

      <div className="row">
        <div className="field">
          <label>{t('common.name')}</label>
          <input className="input" value={name} onChange={(e) => setName(e.target.value)} autoFocus />
        </div>
        <div className="field">
          <label>{t('server.tags')}</label>
          <input className="input" value={tags} onChange={(e) => setTags(e.target.value)} placeholder={t('server.tagsHint')} />
        </div>
      </div>

      <div className="row">
        <div className="field" style={{ flex: 3 }}>
          <label>{t('server.host')}</label>
          <input className="input mono" value={host} onChange={(e) => setHost(e.target.value)} placeholder="10.0.0.12" />
        </div>
        <div className="field">
          <label>{t('server.port')}</label>
          <input className="input mono" value={port} onChange={(e) => setPort(e.target.value)} />
        </div>
      </div>

      <div className="field">
        <label>{t('server.username')}</label>
        <input className="input mono" value={username} onChange={(e) => setUsername(e.target.value)} />
      </div>

      <div className="field">
        <label>{t('server.authType')}</label>
        <div className="toggle">
          <button type="button" className={authType === 'key' ? 'active' : ''} onClick={() => setAuthType('key')}>
            {t('server.authKey')}
          </button>
          <button
            type="button"
            className={authType === 'password' ? 'active' : ''}
            onClick={() => setAuthType('password')}
          >
            {t('server.authPassword')}
          </button>
        </div>
      </div>

      {authType === 'key' ? (
        <>
          <div className="field">
            <label>{t('server.privateKey')}</label>
            <textarea
              className="textarea"
              style={{ minHeight: 130 }}
              value={privateKey}
              onChange={(e) => setPrivateKey(e.target.value)}
              placeholder={secretHint ?? '-----BEGIN OPENSSH PRIVATE KEY-----'}
            />
            {secretHint ? <span className="hint">{secretHint}</span> : null}
          </div>
          <div className="field">
            <label>{t('server.passphrase')}</label>
            <input className="input" type="password" value={passphrase} onChange={(e) => setPassphrase(e.target.value)} />
            {secretHint ? <span className="hint">{secretHint}</span> : null}
          </div>
        </>
      ) : (
        <div className="field">
          <label>{t('server.password')}</label>
          <input className="input" type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
          {secretHint ? <span className="hint">{secretHint}</span> : null}
        </div>
      )}

      <div className="field">
        <label>{t('server.sudoPassword')}</label>
        <input className="input" type="password" value={sudoPassword} onChange={(e) => setSudoPassword(e.target.value)} />
        {secretHint ? <span className="hint">{secretHint}</span> : null}
      </div>

      <div className="field">
        <label>{t('common.description')}</label>
        <input className="input" value={description} onChange={(e) => setDescription(e.target.value)} />
      </div>

      {editing ? (
        <div className="field">
          <label className="flex" style={{ cursor: 'pointer' }}>
            <input type="checkbox" checked={resetHostKey} onChange={(e) => setResetHostKey(e.target.checked)} />
            {t('server.resetHostKey')}
          </label>
          {initial?.host_key_fingerprint ? (
            <span className="hint mono">{`${t('server.fingerprint')}: ${initial.host_key_fingerprint}`}</span>
          ) : null}
        </div>
      ) : null}
    </Modal>
  )
}
