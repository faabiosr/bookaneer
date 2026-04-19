package sql

import (
	"database/sql"
	"errors"
)

// IsNoRows wraps the sql.ErrNoRows.
func IsNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
