package audit

import (
	"log"

	"github.com/AzizARSLN/drydock/internal/models"
	"github.com/AzizARSLN/drydock/internal/store"
)

// Common status values written to the audit trail.
const (
	StatusSuccess = "success"
	StatusError   = "error"
	StatusDryRun  = "dry-run"
)

// Logger persists audit entries. Failures never block the tracked operation.
type Logger struct {
	store *store.Store
}

// New builds an audit logger backed by the given store.
func New(s *store.Store) *Logger { return &Logger{store: s} }

// Record writes an entry to the audit trail.
func (l *Logger) Record(e models.AuditEntry) {
	if err := l.store.AddAudit(&e); err != nil {
		log.Printf("audit: failed to persist entry %q: %v", e.Action, err)
	}
}
