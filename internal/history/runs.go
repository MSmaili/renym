package history

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Run struct {
	ID            string    `json:"id"`
	Path          string    `json:"path"`
	State         string    `json:"state"`
	SchemaVersion int       `json:"schema_version"`
	Timestamp     time.Time `json:"timestamp"`
	Completed     int       `json:"completed"`
	Undone        int       `json:"undone"`
	Directories   int       `json:"directories"`
	Cleaned       int       `json:"cleaned"`
	Error         string    `json:"error,omitempty"`
}

// Runs discovers journals independently of files still occupying their input folder.
func (s *GlobalStore) Runs() ([]Run, error) {
	var runs []Run
	err := s.visitRunFiles(func(path, id string) error {
		entry, err := s.loadEntry(path)
		run := Run{ID: id}
		if err != nil {
			run.Error = err.Error()
		} else {
			run.Path, run.State, run.SchemaVersion, run.Timestamp = entry.Path, entry.State, entry.SchemaVersion, entry.Timestamp
			run.Completed, run.Undone = len(entry.Operations), entry.Undone
			if entry.Organization != nil {
				run.Directories, run.Cleaned = len(entry.Organization.Directories), entry.Organization.Cleaned
			}
		}
		runs = append(runs, run)
		return nil
	})
	sort.Slice(runs, func(i, j int) bool {
		if runs[i].Timestamp.Equal(runs[j].Timestamp) {
			return runs[i].ID > runs[j].ID
		}
		return runs[i].Timestamp.After(runs[j].Timestamp)
	})
	return runs, err
}

func (s *GlobalStore) FindRun(id string) (*Entry, error) {
	if !validHistoryFile(id) {
		return nil, errors.New("invalid history file ID")
	}
	var found *Entry
	err := s.visitRunFiles(func(path, name string) error {
		if id != name {
			return nil
		}
		if found != nil {
			return errors.New("ambiguous history run ID")
		}
		entry, err := s.loadEntry(path)
		if err != nil {
			return err
		}
		entry.ID = name
		found = entry
		return nil
	})
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, ErrNoHistory
	}
	return found, nil
}

func (s *GlobalStore) visitRunFiles(visit func(path, id string) error) error {
	root := filepath.Join(s.configDir, historySubDir)
	buckets, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, bucket := range buckets {
		if bucket.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("history bucket is a link: %s", bucket.Name())
		}
		if !bucket.IsDir() {
			continue
		}
		path := filepath.Join(root, bucket.Name())
		if err := visitRunBucket(path, visit); err != nil {
			return err
		}
	}
	return nil
}

func visitRunBucket(path string, visit func(path, id string) error) error {
	files, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		if file.Type()&os.ModeSymlink != 0 || file.IsDir() {
			return errors.New("invalid history run file")
		}
		if err := visit(filepath.Join(path, file.Name()), file.Name()); err != nil {
			return err
		}
	}
	return nil
}
