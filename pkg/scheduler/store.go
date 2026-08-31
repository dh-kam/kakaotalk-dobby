package scheduler

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
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
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.ID] = job
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
	return job, nil
}

func (s *MemoryStore) List() ([]*Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]*Job, 0, len(s.jobs))
	for _, j := range s.jobs {
		list = append(list, j)
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
		s.jobs[j.ID] = j
	}

	return nil
}

func (s *FileStore) persist() error {
	dir := filepath.Dir(s.filePath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}

	list := make([]*Job, 0, len(s.jobs))
	for _, j := range s.jobs {
		list = append(list, j)
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal jobs: %w", err)
	}

	tmpFile := s.filePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("write tmp file %s: %w", tmpFile, err)
	}

	return os.Rename(tmpFile, s.filePath)
}

func (s *FileStore) Save(job *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.ID] = job
	return s.persist()
}

func (s *FileStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.jobs, id)
	return s.persist()
}

func (s *FileStore) Get(id string) (*Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[id]
	if !ok {
		return nil, ErrJobNotFound
	}
	return job, nil
}

func (s *FileStore) List() ([]*Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]*Job, 0, len(s.jobs))
	for _, j := range s.jobs {
		list = append(list, j)
	}
	return list, nil
}
