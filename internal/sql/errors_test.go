package sql

import (
	"database/sql"
	"testing"
)

func TestErrors(t *testing.T) {
	if !IsNoRows(sql.ErrNoRows) {
		t.Error("expected true, got false")
	}
}
