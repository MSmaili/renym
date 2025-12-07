package history

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const historyDir = ".rnm-history"
const limitNumberOfHistoryFiles = 5

func Save(basePath string, e Entry) error {

	dir := filepath.Join(basePath, historyDir)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}

	fileName := e.Timestamp.Format("2006-01-02_150405") + ".json"
	path := filepath.Join(dir, fileName)

	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal history: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write history file: %w", err)
	}

	return Cleanup(limitNumberOfHistoryFiles)
}

func List() ([]string, error) {
	pattern := filepath.Join(historyDir, "*.json")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	var historyFiles []string
	historyFiles = append(historyFiles, files...)

	sort.Slice(historyFiles, func(i, j int) bool {
		return historyFiles[i] > historyFiles[j]
	})

	return historyFiles, nil
}

func Cleanup(keepLast int) error {
	files, err := List()
	if err != nil {
		return err
	}

	if len(files) <= keepLast {
		return nil
	}

	for i := keepLast; i < len(files); i++ {
		if err := os.Remove(files[i]); err != nil {
			return fmt.Errorf("failed to remove %s: %w", files[i], err)
		}
	}

	return nil
}

func Load(path string) (*Entry, error) {

	if path == "" {
		latestFileName, err := latest(historyDir)
		if err != nil {
			return nil, err
		}
		path = filepath.Join(historyDir, latestFileName)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading file %w", err)
	}

	var entry Entry
	err = json.Unmarshal(data, &entry)
	if err != nil {
		return nil, fmt.Errorf("error parsing file %w", err)
	}

	return &entry, nil
}

func Delete(path string) error {

	if path == "" {
		latestFileName, err := latest(historyDir)
		if err != nil {
			return err
		}
		path = filepath.Join(historyDir, latestFileName)
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to remove %s: %w", path, err)
	}

	return nil
}

func latest(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("reading file, for this directory \"%s\",  %w", dir, err)
	}

	var files []fs.DirEntry
	for _, f := range entries {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".json") {
			files = append(files, f)
		}
	}

	if len(files) == 0 {
		return "", fmt.Errorf("no history found")
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() < files[j].Name()
	})

	latest := files[len(files)-1]
	return latest.Name(), nil
}
