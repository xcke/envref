package backend

import (
	"github.com/xcke/envref/internal/audit"
)

// AuditBackend wraps a Backend and logs mutation operations (Set, Delete) to
// an audit logger. Read operations (Get, List) are passed through without logging.
type AuditBackend struct {
	inner   Backend
	logger  *audit.Logger
	project string
	profile string
	op      audit.Operation
}

// AuditOption configures an AuditBackend.
type AuditOption func(*AuditBackend)

// WithAuditProfile sets the profile scope for audit log entries.
func WithAuditProfile(profile string) AuditOption {
	return func(a *AuditBackend) {
		a.profile = profile
	}
}

// WithAuditOperation sets a default operation type for all mutations logged
// by this wrapper. If not set, defaults to audit.OpSet for Set calls and
// audit.OpDelete for Delete calls.
func WithAuditOperation(op audit.Operation) AuditOption {
	return func(a *AuditBackend) {
		a.op = op
	}
}

// NewAuditBackend creates an AuditBackend that wraps the given backend and
// logs mutations to the provided audit logger.
func NewAuditBackend(inner Backend, logger *audit.Logger, project string, opts ...AuditOption) *AuditBackend {
	ab := &AuditBackend{
		inner:   inner,
		logger:  logger,
		project: project,
	}
	for _, opt := range opts {
		opt(ab)
	}
	return ab
}

// Name returns the name of the underlying backend.
func (a *AuditBackend) Name() string {
	return a.inner.Name()
}

// Get retrieves a secret from the underlying backend without logging.
func (a *AuditBackend) Get(key string) (string, error) {
	return a.inner.Get(key)
}

// Set stores a secret and logs the operation to the audit log.
// The audit entry is written only if the underlying Set succeeds.
func (a *AuditBackend) Set(key, value string) error {
	if err := a.inner.Set(key, value); err != nil {
		return err
	}

	op := a.op
	if op == "" {
		op = audit.OpSet
	}

	// Log the operation; audit failures are non-fatal (best effort).
	_ = a.logger.Log(audit.Entry{
		Operation: op,
		Key:       key,
		Backend:   a.inner.Name(),
		Project:   a.project,
		Profile:   a.profile,
	})

	return nil
}

// Delete removes a secret and logs the operation to the audit log.
// The audit entry is written only if the underlying Delete succeeds.
func (a *AuditBackend) Delete(key string) error {
	if err := a.inner.Delete(key); err != nil {
		return err
	}

	_ = a.logger.Log(audit.Entry{
		Operation: audit.OpDelete,
		Key:       key,
		Backend:   a.inner.Name(),
		Project:   a.project,
		Profile:   a.profile,
	})

	return nil
}

// List returns all secret keys from the underlying backend without logging.
func (a *AuditBackend) List() ([]string, error) {
	return a.inner.List()
}
