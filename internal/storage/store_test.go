package storage

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func TestOwnershipPersistenceRevision(t *testing.T) {
	d := t.TempDir()
	s, e := Open(d)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = Open(d); !errors.Is(e, ErrOwned) {
		t.Fatal("second owner accepted", e)
	}
	c := context.Background()
	if _, e = s.Save(c, 0, []byte(`{"name":"摄像机"}`)); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Save(c, 0, []byte("wrong")); !errors.Is(e, ErrRevision) {
		t.Fatal(e)
	}
	s.Close()
	s, e = Open(d)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	v, b, e := s.Load(c)
	if e != nil || v != 1 || string(b) != `{"name":"摄像机"}` {
		t.Fatal("persistence failed", e)
	}
}
func TestFailedMigrationRetainsOriginal(t *testing.T) {
	d := t.TempDir()
	db, e := sql.Open("sqlite", filepath.Join(d, "lab.db"))
	if e != nil {
		t.Fatal(e)
	}
	db.Exec("CREATE TABLE configuration (original TEXT); INSERT INTO configuration VALUES ('retained')")
	db.Close()
	if _, e = Open(d); e == nil {
		t.Fatal("conflicting migration should fail")
	}
	db, _ = sql.Open("sqlite", filepath.Join(d, "lab.db"))
	defer db.Close()
	var original string
	if e = db.QueryRow("SELECT original FROM configuration").Scan(&original); e != nil || original != "retained" {
		t.Fatal("original data changed")
	}
	backups, _ := filepath.Glob(filepath.Join(d, "pre-migration-*.db"))
	if len(backups) != 1 {
		t.Fatal("missing migration backup")
	}
}
