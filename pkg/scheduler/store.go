package scheduler

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	ErrJobNotFound = errors.New("job not found")
)

// Store defines persistence operations for jobs.
type Store interface {
	Save(job *Job) error
	Delete(id string) error
	Get(id string) (*Job, error)
	List() ([]*Job, error)
}

// MemoryStore is an in-memory thread-safe implementation of Store.
type MemoryStore struct {
	jobs map[string]*Job
	mu   sync.RWMutex
}

// NewMemoryStore creates an in-memory job store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		jobs: make(map[string]*Job),
	}
}

func (s *MemoryStore) Save(job *Job) error {
	if job == nil {
		return errors.New("job cannot be nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.ID] = job.Clone()
	return nil
}

func (s *MemoryStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.jobs, id)
	return nil
}

func (s *MemoryStore) Get(id string) (*Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[id]
	if !ok {
		return nil, ErrJobNotFound
	}
	return job.Clone(), nil
}

func (s *MemoryStore) List() ([]*Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]*Job, 0, len(s.jobs))
	for _, j := range s.jobs {
		list = append(list, j.Clone())
	}
	return list, nil
}

// FileStore is a JSON file-backed implementation of Store.
type FileStore struct {
	filePath string
	jobs     map[string]*Job
	mu       sync.RWMutex
}

// NewFileStore creates or loads a JSON file-backed job store.
func NewFileStore(filePath string) (*FileStore, error) {
	fs := &FileStore{
		filePath: filePath,
		jobs:     make(map[string]*Job),
	}

	if err := fs.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("load job store from %s: %w", filePath, err)
	}

	return fs, nil
}

func (s *FileStore) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	var list []*Job
	if err := json.Unmarshal(data, &list); err != nil {
		return fmt.Errorf("unmarshal jobs: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs = make(map[string]*Job, len(list))
	for _, j := range list {
		if j != nil && j.ID != "" {
			s.jobs[j.ID] = j.Clone()
		}
	}

	return nil
}

func (s *FileStore) persistLocked(targetJobs map[string]*Job) error {
	dir := filepath.Dir(s.filePath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}

	list := make([]*Job, 0, len(targetJobs))
	for _, j := range targetJobs {
		list = append(list, j.Clone())
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal jobs: %w", err)
	}

	tmpFile := fmt.Sprintf("%s.tmp.%d", s.filePath, time.Now().UnixNano())
	f, err := os.OpenFile(tmpFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("create tmp file %s: %w", tmpFile, err)
	}

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpFile)
		return fmt.Errorf("write tmp file %s: %w", tmpFile, err)
	}

	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpFile)
		return fmt.Errorf("sync tmp file %s: %w", tmpFile, err)
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("close tmp file %s: %w", tmpFile, err)
	}

	if err := os.Rename(tmpFile, s.filePath); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("rename tmp file to %s: %w", s.filePath, err)
	}

	return nil
}

func (s *FileStore) Save(job *Job) error {
	if job == nil {
		return errors.New("job cannot be nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clone target map state
	targetJobs := make(map[string]*Job, len(s.jobs)+1)
	for k, v := range s.jobs {
		targetJobs[k] = v.Clone()
	}
	targetJobs[job.ID] = job.Clone()

	if err := s.persistLocked(targetJobs); err != nil {
		return err
	}

	s.jobs = targetJobs
	return nil
}

func (s *FileStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.jobs[id]; !ok {
		return nil
	}

	targetJobs := make(map[string]*Job, len(s.jobs))
	for k, v := range s.jobs {
		if k != id {
			targetJobs[k] = v.Clone()
		}
	}

	if err := s.persistLocked(targetJobs); err != nil {
		return err
	}

	s.jobs = targetJobs
	return nil
}

func (s *FileStore) Get(id string) (*Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[id]
	if !ok {
		return nil, ErrJobNotFound
	}
	return job.Clone(), nil
}

func (s *FileStore) List() ([]*Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]*Job, 0, len(s.jobs))
	for _, j := range s.jobs {
		list = append(list, j.Clone())
	}
	return list, nil
}
