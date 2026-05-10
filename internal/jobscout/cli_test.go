package jobscout

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/wallentx/jobscout/internal/domain"
	appruntime "github.com/wallentx/jobscout/internal/runtime"
	"github.com/wallentx/jobscout/internal/storage"
)

type recordingJobStore struct {
	loaded    []storage.Job
	saved     []storage.Job
	saveCalls int
}

func (s *recordingJobStore) LoadJobs() ([]storage.Job, error) {
	return append([]storage.Job(nil), s.loaded...), nil
}

func (s *recordingJobStore) SaveJobs(jobs []storage.Job) error {
	s.saveCalls++
	s.saved = append([]storage.Job(nil), jobs...)
	return nil
}

func TestRunImportCLISavesDuplicateImportEnrichment(t *testing.T) {
	store := &recordingJobStore{
		loaded: []storage.Job{
			{
				Company:      "Acme",
				Title:        "Platform Engineer",
				Compensation: "Not listed",
				Status:       "Unopened",
			},
		},
	}
	imported, err := json.Marshal([]domain.Job{
		{
			Company:      "Acme",
			Title:        "Platform Engineer",
			Compensation: "$180,000",
			Status:       "Unopened",
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	withImportStdin(t, imported)

	code := runImportCLI(appruntime.Stores{Jobs: store})
	if code != 0 {
		t.Fatalf("runImportCLI() = %d; want 0", code)
	}
	if store.saveCalls != 1 {
		t.Fatalf("SaveJobs calls = %d; want 1", store.saveCalls)
	}
	if len(store.saved) != 1 {
		t.Fatalf("saved jobs len = %d; want 1 (%#v)", len(store.saved), store.saved)
	}
	if store.saved[0].Compensation != "$180,000" {
		t.Fatalf("saved compensation = %q; want imported enrichment", store.saved[0].Compensation)
	}
}

func withImportStdin(t *testing.T, data []byte) {
	t.Helper()

	previous := os.Stdin
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	if _, err := writer.Write(data); err != nil {
		t.Fatalf("writer.Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close() error = %v", err)
	}
	os.Stdin = reader
	t.Cleanup(func() {
		os.Stdin = previous
		_ = reader.Close()
	})
}
