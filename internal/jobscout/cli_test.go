package jobscout

import (
	"encoding/json"
	"io"
	"os"
	"strings"
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

func TestRunRejectsRemovedMigrateCommand(t *testing.T) {
	code := Run([]string{"jobscout", "--migrate"})
	if code != 1 {
		t.Fatalf("Run(... --migrate) = %d; want 1", code)
	}
}

func TestHelpOmitsRemovedMigrateCommand(t *testing.T) {
	output := captureStdout(t, printHelp)
	if strings.Contains(output, "--migrate") {
		t.Fatalf("help output contains removed migrate command:\n%s", output)
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

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	previous := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = writer
	fn()
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close() error = %v", err)
	}
	os.Stdout = previous
	t.Cleanup(func() {
		_ = reader.Close()
	})
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("io.ReadAll() error = %v", err)
	}
	return string(data)
}
