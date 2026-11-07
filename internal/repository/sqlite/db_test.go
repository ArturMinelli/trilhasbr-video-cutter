package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// Connections are recycled; foreign keys (needed for ON DELETE CASCADE) must be on for every new one.
func TestOpen_ForeignKeysSurviveConnectionRecycling(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "fk.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	db.SetConnMaxLifetime(time.Nanosecond) // every query now gets a fresh connection
	for i := 0; i < 3; i++ {
		time.Sleep(2 * time.Millisecond)
		var on int
		if err = db.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&on); err != nil || on != 1 {
			t.Fatalf("foreign_keys=%d err=%v on connection %d", on, err, i)
		}
	}
}
