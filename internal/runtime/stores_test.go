package runtime

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wallentx/jobscout/internal/storage"
)

func TestOpenStoresErrorsWhenSQLiteParentCannotBeCreated(t *testing.T) {
	tmpDir := t.TempDir()
	blockedParent := filepath.Join(tmpDir, "blocked")
	if err := os.WriteFile(blockedParent, []byte("not a directory"), 0600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", blockedParent, err)
	}

	_, cleanup, err := OpenStores(Paths{
		SQLite: filepath.Join(blockedParent, SQLiteFileName),
	})
	defer cleanup()

	if err == nil {
		t.Fatal("OpenStores(...) error = nil; want SQLite initialization error")
	}
}

func TestDeleteSQLiteDatabaseRemovesSQLiteFiles(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), SQLiteFileName)
	paths := Paths{SQLite: dbPath}
	for _, path := range sqliteDatabaseFiles(dbPath) {
		if err := os.WriteFile(path, []byte("test"), 0600); err != nil {
			t.Fatalf("os.WriteFile(%q) error = %v", path, err)
		}
	}

	removed, err := DeleteSQLiteDatabase(paths)
	if err != nil {
		t.Fatalf("DeleteSQLiteDatabase(%#v) error = %v", paths, err)
	}
	if len(removed) != 3 {
		t.Fatalf("DeleteSQLiteDatabase(%#v) removed %d files; want 3 (%#v)", paths, len(removed), removed)
	}
	for _, path := range sqliteDatabaseFiles(dbPath) {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("os.Stat(%q) error = %v; want not exist", path, err)
		}
	}
}

func TestDeleteSQLiteDatabaseIgnoresMissingFiles(t *testing.T) {
	paths := Paths{SQLite: filepath.Join(t.TempDir(), SQLiteFileName)}

	removed, err := DeleteSQLiteDatabase(paths)
	if err != nil {
		t.Fatalf("DeleteSQLiteDatabase(%#v) error = %v", paths, err)
	}
	if len(removed) != 0 {
		t.Fatalf("DeleteSQLiteDatabase(%#v) removed = %#v; want none", paths, removed)
	}
}

func TestInMemoryStoresDoNotPersistToDisk(t *testing.T) {
	stores := InMemoryStores()
	jobs := []storage.Job{{
		Company: "Acme",
		Title:   "Software Engineer",
	}}
	if err := stores.Jobs.SaveJobs(jobs); err != nil {
		t.Fatalf("InMemoryStores().Jobs.SaveJobs(...) error = %v", err)
	}

	loaded, err := stores.Jobs.LoadJobs()
	if err != nil {
		t.Fatalf("InMemoryStores().Jobs.LoadJobs() error = %v", err)
	}
	if len(loaded) != 1 || loaded[0].Company != "Acme" {
		t.Fatalf("InMemoryStores().Jobs.LoadJobs() = %#v; want Acme job", loaded)
	}

	loaded[0].Company = "Mutated"
	reloaded, err := stores.Jobs.LoadJobs()
	if err != nil {
		t.Fatalf("InMemoryStores().Jobs.LoadJobs() second error = %v", err)
	}
	if reloaded[0].Company != "Acme" {
		t.Fatalf("InMemoryStores().Jobs.LoadJobs()[0].Company = %q; want Acme copy isolation", reloaded[0].Company)
	}
}
