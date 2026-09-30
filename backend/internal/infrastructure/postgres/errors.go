package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// PostgreSQL error codes we react to.
const (
	codeUniqueViolation     = "23505"
	codeForeignKeyViolation = "23503"
	codeCheckViolation      = "23514"
)

// Constraint names, declared in the migrations. Matching on the constraint rather
// than on the message keeps the translation precise when a table grows a second
// unique index.
const (
	constraintPaymentReferenceExternalRef = "payment_references_external_reference_key"
	constraintPaymentReferenceBusiness    = "payment_references_business_id_fkey"
	constraintWebhookEventEventID         = "webhook_events_event_id_key"
)

// isUniqueViolation reports whether err is a unique violation. An empty
// constraint matches any of them.
func isUniqueViolation(err error, constraint string) bool {
	return isConstraintViolation(err, codeUniqueViolation, constraint)
}

// isForeignKeyViolation reports whether err is a foreign key violation.
func isForeignKeyViolation(err error, constraint string) bool {
	return isConstraintViolation(err, codeForeignKeyViolation, constraint)
}

// isCheckViolation reports whether err is a check constraint violation, which
// means a domain invariant escaped the entity and was caught by the database.
func isCheckViolation(err error) bool {
	return isConstraintViolation(err, codeCheckViolation, "")
}

func isConstraintViolation(err error, code, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	if pgErr.Code != code {
		return false
	}
	return constraint == "" || pgErr.ConstraintName == constraint
}
