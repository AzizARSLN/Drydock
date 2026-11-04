import { useTheme } from '../context/theme'
import { useI18n } from '../i18n'

// ThemeToggle switches between the dark and light colour themes. The current
// choice is persisted by the ThemeProvider so it survives reloads.
export function ThemeToggle({ className = 'btn btn-ghost btn-icon' }: { className?: string }) {
  const { theme, toggle } = useTheme()
  const { t } = useI18n()
  const label = theme === 'dark' ? t('theme.light') : t('theme.dark')

  return (
    <button type="button" className={className} onClick={toggle} title={label} aria-label={label}>
      {theme === 'dark' ? (
        <svg width="15" height="15" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round">
          <circle cx="8" cy="8" r="3.2" />
          <path d="M8 1.4v1.6M8 13v1.6M14.6 8H13M3 8H1.4M12.7 3.3l-1.1 1.1M4.4 11.6l-1.1 1.1M12.7 12.7l-1.1-1.1M4.4 4.4 3.3 3.3" />
        </svg>
      ) : (
        <svg width="15" height="15" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round">
          <path d="M13.5 9.4A5.6 5.6 0 0 1 6.6 2.5 5.6 5.6 0 1 0 13.5 9.4Z" />
        </svg>
      )}
    </button>
  )
}
