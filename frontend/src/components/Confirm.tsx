import { useState } from 'react'
import { useI18n } from '../i18n'
import { Modal } from './Modal'
import { Alert, Spinner } from './ui'

// ConfirmModal is the two-step guard used by destructive actions that are not
// routed through the dry-run workflow (server and user deletion).
export function ConfirmModal({
  title,
  message,
  word,
  confirmLabel,
  busy,
  onConfirm,
  onClose,
}: {
  title: string
  message: string
  word?: string
  confirmLabel?: string
  busy?: boolean
  onConfirm: () => void
  onClose: () => void
}) {
  const { t } = useI18n()
  const [typed, setTyped] = useState('')
  const ok = !word || typed.trim() === word

  return (
    <Modal
      title={title}
      onClose={onClose}
      footer={
        <>
          <button className="btn" onClick={onClose} disabled={busy}>
            {t('common.cancel')}
          </button>
          <button className="btn btn-danger" onClick={onConfirm} disabled={!ok || busy}>
            {busy ? <Spinner /> : null}
            {confirmLabel ?? t('common.confirm')}
          </button>
        </>
      }
    >
      <Alert kind="err">{message}</Alert>
      {word ? (
        <div className="field mt-12">
          <label>{t('dry.typeConfirm')}</label>
          <input
            className="input mono"
            value={typed}
            onChange={(e) => setTyped(e.target.value)}
            placeholder={word}
            autoFocus
          />
        </div>
      ) : null}
    </Modal>
  )
}
