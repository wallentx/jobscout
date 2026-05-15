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

func TestPrintHelpDocumentsCommandLineOptions(t *testing.T) {
	help := captureStdout(t, printHelp)
	for _, want := range []string{
		"--sources=<list>",
		"rss, site, llm, llm_web, all",
		"--config=<path>",
		"--bench-llm [options]",
		"--list",
		"--task <task|case>",
		"--task=<task|case>",
		"--provider <name>",
		"--provider=<name>",
		"--model <name>",
		"--model=<name>",
		"--all-models",
		"tasks: llm_job_search, llm_job_filtering, llm_company_health, job_identity, resume_to_criteria",
		"--bench-report [options]",
		"--latest",
		"--format <text|md|json>",
		"--format=<text|md|json>",
		"--json",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("printHelp() missing %q in:\n%s", want, help)
		}
	}
	if strings.Contains(help, "--migrate") {
		t.Fatalf("printHelp() includes stale --migrate text:\n%s", help)
	}
	if !strings.Contains(help, "\x1b[") {
		t.Fatalf("printHelp() should include ANSI color styling:\n%s", help)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	previous := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = writer
	defer func() {
		os.Stdout = previous
		_ = reader.Close()
	}()

	fn()
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close() error = %v", err)
	}
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("io.ReadAll(stdout) error = %v", err)
	}
	return string(out)
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
