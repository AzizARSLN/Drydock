import { useCallback, useMemo, useState } from 'react'
import { ApiError, type Outcome } from '../api'
import { useI18n } from '../i18n'
import { useToast } from '../context/toast'
import { Modal } from './Modal'
import { Alert, Loading, Spinner } from './ui'

// DiffView renders the line-oriented diff returned by the backend.
export function DiffView({ diff }: { diff: string }) {
  return (
    <pre className="diff">
      {diff.split('\n').map((line, i) => {
        let cls = 'diff-line-ctx'
        if (line.startsWith('@@')) cls = 'diff-line-hunk'
        else if (line.startsWith('+ ')) cls = 'diff-line-add'
        else if (line.startsWith('- ')) cls = 'diff-line-del'
        return (
          <span key={i} className={cls}>
            {line || ' '}
          </span>
        )
      })}
    </pre>
  )
}

interface PreviewOptions {
  title: string
  // run performs the operation; the boolean selects dry-run vs. apply.
  run: (dryRun: boolean) => Promise<Outcome>
  // confirmWord, when set, must be typed verbatim before applying.
  confirmWord?: string
  onApplied?: (outcome: Outcome) => void
}

// useDryRun drives the mandatory preview -> confirm -> apply workflow shared by
// every mutating action. Rendering `modal` is the only wiring a caller needs.
export function useDryRun() {
  const { t } = useI18n()
  const toast = useToast()
  const [open, setOpen] = useState(false)
  const [loading, setLoading] = useState(false)
  const [applying, setApplying] = useState(false)
  const [opts, setOpts] = useState<PreviewOptions | null>(null)
  const [outcome, setOutcome] = useState<Outcome | null>(null)
  const [error, setError] = useState('')
  const [word, setWord] = useState('')

  const preview = useCallback(async (o: PreviewOptions) => {
    setOpts(o)
    setOutcome(null)
    setError('')
    setWord('')
    setLoading(true)
    setOpen(true)
    try {
      setOutcome(await o.run(true))
    } catch (e) {
      if (e instanceof ApiError && e.outcome) {
        setOutcome(e.outcome)
        setError(e.message)
      } else {
        setError(e instanceof ApiError ? e.message : String(e))
      }
    } finally {
      setLoading(false)
    }
  }, [])

  const close = useCallback(() => {
    if (applying) return
    setOpen(false)
  }, [applying])

  const apply = useCallback(async () => {
    if (!opts) return
    setApplying(true)
    setError('')
    try {
      const o = await opts.run(false)
      setOpen(false)
      toast.ok(o.message || t('dry.applied'))
      opts.onApplied?.(o)
    } catch (e) {
      const msg = e instanceof ApiError ? e.message : String(e)
      setError(msg)
      toast.err(msg)
    } finally {
      setApplying(false)
    }
  }, [opts, toast, t])

  const wordOk = !opts?.confirmWord || word.trim() === opts.confirmWord
  const canApply = !loading && !applying && !error && wordOk

  const commands = outcome?.commands ?? []

  const modal = useMemo(() => {
    if (!open) return null
    return (
      <Modal
        wide
        title={opts?.title ?? t('dry.title')}
        onClose={close}
        footer={
          <>
            <button className="btn" onClick={close} disabled={applying}>
              {t('common.cancel')}
            </button>
            <button className="btn btn-primary" onClick={apply} disabled={!canApply}>
              {applying ? (
                <>
                  <Spinner /> {t('dry.applying')}
                </>
              ) : (
                t('dry.confirmApply')
              )}
            </button>
          </>
        }
      >
        {loading ? (
          <Loading label={t('health.checking')} />
        ) : (
          <>
            {error ? <Alert kind="err">{error}</Alert> : null}
            {outcome ? (
              <div className="flex-col">
                <p className="dim">{outcome.message || t('dry.willApply')}</p>

                {outcome.diff ? (
                  <div>
                    <div className="section-title">{t('dry.diff')}</div>
                    <DiffView diff={outcome.diff} />
                  </div>
                ) : null}

                {commands.length > 0 ? (
                  <div>
                    <div className="section-title">{t('dry.commands')}</div>
                    <pre className="code">{commands.join('\n')}</pre>
                  </div>
                ) : null}

                {outcome.output ? (
                  <div>
                    <div className="section-title">{t('dry.output')}</div>
                    <pre className="code">{outcome.output}</pre>
                  </div>
                ) : null}

                {outcome.backup ? (
                  <div className="fs-11 muted">
                    {t('dry.backup')}: <span className="mono">{outcome.backup}</span>
                  </div>
                ) : null}

                {outcome.diff ? <div className="fs-11 muted">{t('dry.rollback')}</div> : null}

                {opts?.confirmWord ? (
                  <div className="field mt-8">
                    <label>{t('dry.typeConfirm')}</label>
                    <input
                      className="input mono"
                      value={word}
                      onChange={(e) => setWord(e.target.value)}
                      placeholder={opts.confirmWord}
                      autoFocus
                    />
                  </div>
                ) : null}
              </div>
            ) : null}
          </>
        )}
      </Modal>
    )
  }, [open, loading, applying, opts, outcome, error, word, canApply, close, apply, t])

  return { preview, modal }
}
