package history

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	historySubDir    = "history"
	maxEntriesPerDir = 2
)

var ErrNoHistory = errors.New("no history found for directory")

type GlobalStore struct {
	configDir string
	pathID    PathIdentifier
}

func NewGlobalStore(pathID PathIdentifier) (*GlobalStore, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get config dir: %w", err)
	}

	rnmConfigDir := filepath.Join(configDir, "renym")

	return &GlobalStore{
		configDir: rnmConfigDir,
		pathID:    pathID,
	}, nil
}

// NewStore uses an explicit state directory, useful for isolated consumers and
// tests. NewGlobalStore preserves the existing platform-native history location.
func NewStore(configDir string, pathID PathIdentifier) *GlobalStore {
	return &GlobalStore{configDir: configDir, pathID: pathID}
}

func (s *GlobalStore) Directory() string { return s.configDir }

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

	absPath, err := resolveAbsolutePath(dirPath)
	if err != nil {
		return "", err
	}

	entry.Path = absPath
	entry.DirID = dirID

	histDir := s.dirHistoryPath(dirID)
	if err := os.MkdirAll(histDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create history dir: %w", err)
	}

	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return "", fmt.Errorf("create history ID: %w", err)
	}
	fileName := entry.Timestamp.UTC().Format("2006-01-02_150405.000000000") + fmt.Sprintf("_%x.json", id)
	entry.ID = fileName
	filePath := filepath.Join(histDir, fileName)

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal entry: %w", err)
	}

	f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", fmt.Errorf("create history: %w", err)
	}
	_, writeErr := f.Write(data)
	if writeErr == nil {
		writeErr = f.Sync()
	}
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		return "", fmt.Errorf("write history: %w", errors.Join(writeErr, closeErr))
	}

	if err := s.cleanup(histDir); err != nil {
		return "", fmt.Errorf("history cleanup: %w", err)
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

	entry, err := s.loadEntry(filePath)
	if err != nil {
		return nil, err
	}
	entry.ID = latest
	return entry, nil
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

	// Legacy records stored intent and historically sorted it on load. Verified
	// records retain physical completion order; undo reverses it exactly.
	if entry.SchemaVersion == 0 {
		sortOperationsByDepth(entry.Operations)
	}
	return &entry, nil
}

func (s *GlobalStore) latestFile(histDir string) (string, error) {
	entries, err := os.ReadDir(histDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrNoHistory
		}
		return "", fmt.Errorf("failed to read history directory: %w", err)
	}

	var latest string
	var latestTime time.Time
	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}

		entry, err := s.loadEntry(filepath.Join(histDir, name))
		if err != nil {
			return name, nil
		} // Corrupt records block rather than disappear.
		if entry.SchemaVersion != 0 && (entry.SchemaVersion != SchemaVersion || entry.State != Complete && entry.State != Partial) {
			return name, nil // Never bury uncertain intent behind newer records.
		}
		if latest == "" || entry.Timestamp.After(latestTime) || entry.Timestamp.Equal(latestTime) && name > latest {
			latest = name
			latestTime = entry.Timestamp
		}
	}

	if latest == "" {
		return "", ErrNoHistory
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
	return s.DeleteEntry(dirPath, latest)
}

func validHistoryFile(name string) bool {
	return name != "" && filepath.Base(name) == name && !strings.ContainsAny(name, `/\`) && strings.HasSuffix(name, ".json")
}

func (s *GlobalStore) DeleteEntry(dirPath, name string) error {
	if !validHistoryFile(name) {
		return fmt.Errorf("invalid history file ID")
	}
	dirID, err := s.resolveDirID(dirPath)
	if err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(s.dirHistoryPath(dirID), name)); err != nil {
		return fmt.Errorf("delete history: %w", err)
	}
	return nil
}

// Update replaces a journal atomically after syncing its contents. Pending
// records are intentionally retained after errors; normal undo cannot replay
// uncertain syscall/checkpoint windows. Full crash recovery is a later gate.
func (s *GlobalStore) Update(dirPath, name string, entry Entry) error {
	if !validHistoryFile(name) {
		return fmt.Errorf("invalid history file ID")
	}
	dirID, err := s.resolveDirID(dirPath)
	if err != nil {
		return err
	}
	entry.Path, err = resolveAbsolutePath(dirPath)
	if err != nil {
		return err
	}
	entry.DirID, entry.ID = dirID, name
	histDir := s.dirHistoryPath(dirID)
	target := filepath.Join(histDir, name)
	if _, err := os.Lstat(target); err != nil {
		return fmt.Errorf("find history to update: %w", err)
	}
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(histDir, ".checkpoint-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	_, writeErr := f.Write(data)
	if writeErr == nil {
		writeErr = f.Sync()
	}
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		return fmt.Errorf("checkpoint history: %w", errors.Join(writeErr, closeErr))
	}
	if err := os.Rename(tmp, target); err != nil {
		return fmt.Errorf("commit checkpoint: %w", err)
	}
	return s.cleanup(histDir)
}

func (s *GlobalStore) cleanup(histDir string) error {
	entries, err := os.ReadDir(histDir)
	if err != nil {
		return err
	}

	var jsonFiles []string
	timestamps := make(map[string]time.Time)
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			entry, err := s.loadEntry(filepath.Join(histDir, e.Name()))
			if err != nil {
				continue
			} // Never discard corrupt/uncertain records.
			if entry.SchemaVersion != 0 && (entry.SchemaVersion != SchemaVersion || entry.State != Complete) {
				continue
			}
			jsonFiles = append(jsonFiles, e.Name())
			timestamps[e.Name()] = entry.Timestamp
		}
	}

	if len(jsonFiles) <= maxEntriesPerDir {
		return nil
	}

	sort.Slice(jsonFiles, func(i, j int) bool {
		left, right := timestamps[jsonFiles[i]], timestamps[jsonFiles[j]]
		if left.Equal(right) {
			return jsonFiles[i] < jsonFiles[j]
		}
		return left.Before(right)
	})

	toRemove := len(jsonFiles) - maxEntriesPerDir
	for i := range toRemove {
		if err := os.Remove(filepath.Join(histDir, jsonFiles[i])); err != nil {
			return err
		}
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
			depth: strings.Count(filepath.FromSlash(ops[i].Old), string(filepath.Separator)),
		}
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].depth < items[j].depth
	})

	for i := range items {
		ops[i] = items[i].op
	}
}
