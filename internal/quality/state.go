package quality

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	stateFileName  = "quality-state.json"
	taskIDFileName = "current-task-id"
	feedbackFile   = "quality-feedback.txt"
)

// StatePath returns ~/.clem/quality-state.json under homeDir.
func StatePath(homeDir string) string {
	return filepath.Join(homeDir, ".clem", stateFileName)
}

// TaskIDPath returns ~/.clem/current-task-id under homeDir.
func TaskIDPath(homeDir string) string {
	return filepath.Join(homeDir, ".clem", taskIDFileName)
}

// FeedbackPath returns ~/.clem/quality-feedback.txt under homeDir.
func FeedbackPath(homeDir string) string {
	return filepath.Join(homeDir, ".clem", feedbackFile)
}

// RuntimeConfigPath returns ~/.clem/quality.json under homeDir.
func RuntimeConfigPath(homeDir string) string {
	return filepath.Join(homeDir, ".clem", "quality.json")
}

// JSONLPath returns ~/.clem/<agentKey>-quality.jsonl under homeDir.
func JSONLPath(homeDir, agentKey string) string {
	return filepath.Join(homeDir, ".clem", agentKey+"-quality.jsonl")
}

// LoadState reads persisted attempt state.
func LoadState(homeDir string) (State, error) {
	path := StatePath(homeDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return State{}, nil
		}
		return State{}, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	return s, nil
}

// SaveState writes attempt state.
func SaveState(homeDir string, s State) error {
	if err := os.MkdirAll(filepath.Join(homeDir, ".clem"), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(StatePath(homeDir), data, 0600)
}

// CurrentTaskID reads the active task id or returns "default".
func CurrentTaskID(homeDir string) string {
	data, err := os.ReadFile(TaskIDPath(homeDir))
	if err != nil {
		return "default"
	}
	id := strings.TrimSpace(string(data))
	if id == "" {
		return "default"
	}
	return id
}

// ResetAttemptsIfTaskChanged clears attempts when task id changes.
func ResetAttemptsIfTaskChanged(homeDir, taskID string) (State, error) {
	s, err := LoadState(homeDir)
	if err != nil {
		return State{}, err
	}
	if s.TaskID != taskID {
		s = State{TaskID: taskID, Attempts: 0, Blocked: false}
	}
	return s, nil
}
