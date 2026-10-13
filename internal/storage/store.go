// Package storage persists configuration only. In-flight SIP state is never
// restored. An OS lock owns the runtime directory until Close, including crashes.
package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"sync"
)

var ErrOwned = errors.New("runtime directory is already owned")
var ErrRevision = errors.New("configuration revision conflict")

type Repository interface {
	Load(context.Context) (int64, []byte, error)
	Save(context.Context, int64, []byte) (int64, error)
	Close() error
}

type Store struct {
	db   *sql.DB
	lock *os.File
	once sync.Once
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, errors.New("cannot create runtime directory")
	}
	if err := secureDirectory(dir); err != nil {
		return nil, errors.New("cannot protect runtime directory")
	}
	f, err := os.OpenFile(filepath.Join(dir, "engine.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, errors.New("cannot open runtime lock")
	}
	if err = lockFile(f); err != nil {
		f.Close()
		return nil, ErrOwned
	}
	s := &Store{lock: f}
	ok := false
	defer func() {
		if !ok {
			s.Close()
		}
	}()
	path := filepath.Join(dir, "lab.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, errors.New("cannot open SQLite")
	}
	s.db = db
	db.SetMaxOpenConns(1)
	if _, err = db.Exec("PRAGMA busy_timeout=3000; PRAGMA foreign_keys=ON"); err != nil {
		return nil, errors.New("cannot initialise SQLite")
	}
	var version int
	if err = db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return nil, errors.New("cannot read database version")
	}
	if version > 1 {
		return nil, errors.New("database is newer than this application; original data retained")
	}
	if version == 0 {
		// Back up an existing database before the first migration. VACUUM INTO
		// reads a consistent SQLite snapshot, including any committed WAL pages.
		if info, e := os.Stat(path); e == nil && info.Size() > 0 {
			backup := filepath.Join(dir, "pre-migration-"+RandomID()+".db")
			if _, err = db.Exec("VACUUM INTO ?", backup); err != nil {
				return nil, errors.New("migration backup failed; original retained")
			}
		}
		tx, err := db.Begin()
		if err != nil {
			return nil, errors.New("migration transaction failed")
		}
		defer tx.Rollback()
		if _, err = tx.Exec("CREATE TABLE configuration (id INTEGER PRIMARY KEY CHECK(id=1), revision INTEGER NOT NULL, document BLOB NOT NULL); PRAGMA user_version=1"); err != nil {
			return nil, errors.New("migration failed; original retained")
		}
		if err = tx.Commit(); err != nil {
			return nil, errors.New("migration commit failed")
		}
	}
	if _, err = db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, errors.New("cannot enable SQLite WAL")
	}
	ok = true
	return s, nil
}

func (s *Store) Load(ctx context.Context) (int64, []byte, error) {
	var v int64
	var b []byte
	err := s.db.QueryRowContext(ctx, "SELECT revision,document FROM configuration WHERE id=1").Scan(&v, &b)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil, nil
	}
	if err != nil {
		return 0, nil, errors.New("configuration read failed")
	}
	return v, b, nil
}
func (s *Store) Save(ctx context.Context, expected int64, b []byte) (int64, error) {
	if len(b) > 128*1024 {
		return 0, errors.New("configuration exceeds size limit")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, errors.New("configuration write unavailable")
	}
	defer tx.Rollback()
	var result sql.Result
	if expected == 0 {
		result, err = tx.ExecContext(ctx, "INSERT OR IGNORE INTO configuration(id,revision,document) VALUES(1,1,?)", b)
	} else {
		result, err = tx.ExecContext(ctx, "UPDATE configuration SET revision=revision+1,document=? WHERE id=1 AND revision=?", b, expected)
	}
	if err != nil {
		return 0, errors.New("configuration write failed")
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return 0, ErrRevision
	}
	if err = tx.Commit(); err != nil {
		return 0, errors.New("configuration commit failed")
	}
	return expected + 1, nil
}
func (s *Store) Close() error {
	var err error
	s.once.Do(func() {
		if s.db != nil {
			err = s.db.Close()
		}
		if s.lock != nil {
			unlockFile(s.lock)
			s.lock.Close()
		}
	})
	return err
}
func RandomID() string {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic(fmt.Errorf("secure random unavailable: %w", e))
	}
	return hex.EncodeToString(b[:])
}

// LocalSecret is read only by local hosts, never returned by a config DTO.
func LocalSecret(dir, name string) (string, error) {
	path := filepath.Join(dir, name)
	if b, e := os.ReadFile(path); e == nil {
		if len(b) != 64 {
			return "", errors.New("invalid local secret file")
		}
		return string(b), nil
	} else if !os.IsNotExist(e) {
		return "", errors.New("cannot read local secret")
	}
	value := RandomID() + RandomID()
	f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return "", errors.New("cannot create local secret")
	}
	_, e = f.WriteString(value)
	ce := f.Close()
	if e != nil || ce != nil {
		return "", errors.New("cannot save local secret")
	}
	return value, nil
}
