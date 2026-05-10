package tuiapp

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"
)

func deleteJob(jobs []Job, index int) ([]Job, error) {
	if index < 0 || index >= len(jobs) {
		return jobs, fmt.Errorf("index out of bounds")
	}
	// Remove element
	newJobs := append(jobs[:index], jobs[index+1:]...)
	if err := saveRuntimeJobs(newJobs); err != nil {
		return jobs, err
	}
	return newJobs, nil
}

func editJob(job Job) tea.Cmd {
	return func() tea.Msg {
		// Serialize current job to JSON
		data, err := json.MarshalIndent(job, "", "  ")
		if err != nil {
			return healthLoadedMsg{err: fmt.Errorf("failed to marshal job: %v", err)}
		}

		// Create temp file
		f, err := os.CreateTemp("", "job_edit_*.json")
		if err != nil {
			return healthLoadedMsg{err: fmt.Errorf("failed to create temp file: %v", err)}
		}
		tmpPath := f.Name()
		if _, err := f.Write(data); err != nil {
			_ = f.Close()
			return healthLoadedMsg{err: fmt.Errorf("failed to write to temp file: %v", err)}
		}
		if err := f.Close(); err != nil {
			return healthLoadedMsg{err: fmt.Errorf("failed to close temp file: %v", err)}
		}
		defer func() {
			_ = os.Remove(tmpPath)
		}()

		// Open editor
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "nano"
		}
		cmd := exec.Command(editor, tmpPath)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return healthLoadedMsg{err: fmt.Errorf("editor failed: %v", err)}
		}

		// Read back content
		newData, err := os.ReadFile(tmpPath)
		if err != nil {
			return healthLoadedMsg{err: fmt.Errorf("failed to read edit file: %v", err)}
		}

		// Parse
		var newJob Job
		if err := json.Unmarshal(newData, &newJob); err != nil {
			return healthLoadedMsg{err: fmt.Errorf("invalid JSON: %v", err)}
		}

		return jobEditedMsg{job: newJob}
	}
}

type jobEditedMsg struct {
	job Job
}

func openURL(url string) tea.Cmd {
	return func() tea.Msg {
		if cmdName := urlOpenCommand(runtime.GOOS, exec.LookPath); cmdName != "" {
			_ = exec.Command(cmdName, url).Run()
		}
		return nil
	}
}

func urlOpenCommand(goos string, lookPath func(string) (string, error)) string {
	candidates := []string{"termux-open-url"}
	if goos == "darwin" {
		candidates = append(candidates, "open")
	}
	candidates = append(candidates, "xdg-open")

	for _, candidate := range candidates {
		if _, err := lookPath(candidate); err == nil {
			return candidate
		}
	}
	return ""
}
