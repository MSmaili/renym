package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	historySubDir    = "history"
	maxEntriesPerDir = 2
)

type GlobalStore struct {
	configDir string
	pathID    PathIdentifier
}

func NewGlobalStore(pathID PathIdentifier) (*GlobalStore, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get config dir: %w", err)
	}

	rnmConfigDir := filepath.Join(configDir, "rnm")

	return &GlobalStore{
		configDir: rnmConfigDir,
		pathID:    pathID,
	}, nil
}

func sanitizeDirID(dirID string) string {
	return strings.ReplaceAll(dirID, ":", "_")
}

// dirHistoryPath returns the path to a directory's history folder
func (s *GlobalStore) dirHistoryPath(dirID string) string {
	return filepath.Join(s.configDir, historySubDir, sanitizeDirID(dirID))
}

func (s *GlobalStore) Save(dirPath string, entry Entry) (string, error) {
	dirID, err := s.resolveDirID(dirPath)
	if err != nil {
		return "", err
	}

	absPath, _ := resolveAbsolutePath(dirPath)

	entry.Path = absPath
	entry.DirID = dirID

	histDir := s.dirHistoryPath(dirID)
	if err := os.MkdirAll(histDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create history dir: %w", err)
	}

	fileName := entry.Timestamp.Format("2006-01-02_150405") + ".json"
	filePath := filepath.Join(histDir, fileName)

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal entry: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write history: %w", err)
	}

	if err := s.cleanup(histDir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: cleanup failed: %v\n", err)
	}

	return fileName, nil
}

func (s *GlobalStore) Latest(dirPath string) (*Entry, error) {
	dirID, err := s.resolveDirID(dirPath)
	if err != nil {
		return nil, err
	}

	histDir := s.dirHistoryPath(dirID)

	latest, err := s.latestFile(histDir)
	if err != nil {
		return nil, err
	}
	filePath := filepath.Join(histDir, latest)

	return s.loadEntry(filePath)
}

func (s *GlobalStore) loadEntry(path string) (*Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read history: %w", err)
	}

	var entry Entry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("failed to parse history: %w", err)
	}

	sortOperationsByDepth(entry.Operations)
	return &entry, nil
}

func (s *GlobalStore) latestFile(histDir string) (string, error) {
	entries, err := os.ReadDir(histDir)
	if err != nil {
		return "", fmt.Errorf("failed to read history directory: %w", err)
	}

	var latest string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}

		if latest == "" || name > latest {
			latest = name
		}
	}

	if latest == "" {
		return "", fmt.Errorf("No history found for directory")
	}

	return latest, nil
}

func (s *GlobalStore) Delete(dirPath string) error {
	dirID, err := s.resolveDirID(dirPath)
	if err != nil {
		return err
	}

	histDir := s.dirHistoryPath(dirID)

	latest, err := s.latestFile(histDir)
	if err != nil {
		return err
	}
	filePath := filepath.Join(histDir, latest)

	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete history file: %w", err)
	}

	return nil
}

func (s *GlobalStore) cleanup(histDir string) error {
	entries, err := os.ReadDir(histDir)
	if err != nil {
		return err
	}

	var jsonFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			jsonFiles = append(jsonFiles, e.Name())
		}
	}

	if len(jsonFiles) <= maxEntriesPerDir {
		return nil
	}

	sort.Strings(jsonFiles)

	toRemove := len(jsonFiles) - maxEntriesPerDir
	for i := range toRemove {
		os.Remove(filepath.Join(histDir, jsonFiles[i]))
	}

	return nil
}

func resolveAbsolutePath(dirPath string) (string, error) {

	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return "", err
	}

	absPath, err = resolveDir(absPath)
	if err != nil {
		return "", err
	}

	return absPath, nil
}

func (s *GlobalStore) resolveDirID(dirPath string) (string, error) {
	absPath, err := resolveAbsolutePath(dirPath)
	if err != nil {
		return "", err
	}

	dirID, err := s.pathID.PathIdentifier(absPath)
	if err != nil {
		return "", err
	}

	return dirID, nil
}

func resolveDir(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return filepath.Dir(path), nil
	}
	return path, nil
}

// sortOperationsByDepth sorts operations by path depth (top-level first, deeper paths later).
func sortOperationsByDepth(ops []Operation) {
	type opWithDepth struct {
		op    Operation
		depth int
	}

	items := make([]opWithDepth, len(ops))
	for i := range ops {
		items[i] = opWithDepth{
			op:    ops[i],
			depth: strings.Count(ops[i].Old, string(filepath.Separator)),
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].depth < items[j].depth
	})

	for i := range items {
		ops[i] = items[i].op
	}
}
