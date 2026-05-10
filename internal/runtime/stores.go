package runtime

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/wallentx/jobscout/internal/storage"
)

type Stores struct {
	Jobs            storage.JobStore
	Health          storage.HealthStore
	CompanyIdentity storage.CompanyIdentityStore
}

func DeleteSQLiteDatabase(paths Paths) ([]string, error) {
	var removed []string
	for _, path := range sqliteDatabaseFiles(paths.SQLite) {
		if err := os.Remove(path); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return removed, err
		}
		removed = append(removed, path)
	}
	return removed, nil
}

func sqliteDatabaseFiles(path string) []string {
	return []string{
		path,
		path + "-wal",
		path + "-shm",
	}
}

func OpenStores(paths Paths) (Stores, func(), error) {
	if err := ensureParentDir(paths.SQLite); err != nil {
		return Stores{}, func() {}, err
	}
	sqliteStore, err := storage.NewSQLiteStore(paths.SQLite)
	if err != nil {
		return Stores{}, func() {}, err
	}
	return Stores{
			Jobs:            sqliteStore,
			Health:          sqliteStore,
			CompanyIdentity: sqliteStore,
		}, func() {
			_ = sqliteStore.Close()
		}, nil
}

func ensureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || strings.TrimSpace(dir) == "" {
		return nil
	}
	return os.MkdirAll(dir, 0700)
}
