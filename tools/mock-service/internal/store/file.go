package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/raywall/workflows/sample-test/mock-service/internal/domain"
)

type FileRepository struct {
	path  string
	mu    sync.RWMutex
	items map[string]domain.MockDefinition
}

func NewFileRepository(path string) (*FileRepository, error) {
	repo := &FileRepository{
		path:  path,
		items: map[string]domain.MockDefinition{},
	}
	if err := repo.load(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (r *FileRepository) List(context.Context) ([]domain.MockDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]domain.MockDefinition, 0, len(r.items))
	for _, item := range r.items {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items, nil
}

func (r *FileRepository) Get(_ context.Context, id string) (domain.MockDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.items[id]
	if !ok {
		return domain.MockDefinition{}, domain.ErrNotFound
	}
	return item, nil
}

func (r *FileRepository) Save(_ context.Context, mock domain.MockDefinition) (domain.MockDefinition, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	mock.Normalize()
	if mock.ID == "" {
		id, err := domain.NewID()
		if err != nil {
			return domain.MockDefinition{}, err
		}
		mock.ID = id
	}
	if err := mock.Validate(); err != nil {
		return domain.MockDefinition{}, err
	}
	r.items[mock.ID] = mock
	if err := r.persistLocked(); err != nil {
		return domain.MockDefinition{}, err
	}
	return mock, nil
}

func (r *FileRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[id]; !ok {
		return domain.ErrNotFound
	}
	delete(r.items, id)
	return r.persistLocked()
}

func (r *FileRepository) load() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	data, err := os.ReadFile(r.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read data file: %w", err)
	}
	if len(data) == 0 {
		return nil
	}
	var items []domain.MockDefinition
	if err := json.Unmarshal(data, &items); err != nil {
		return fmt.Errorf("decode data file: %w", err)
	}
	for _, item := range items {
		item.Normalize()
		r.items[item.ID] = item
	}
	return nil
}

func (r *FileRepository) persistLocked() error {
	items := make([]domain.MockDefinition, 0, len(r.items))
	for _, item := range r.items {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return fmt.Errorf("encode data file: %w", err)
	}
	tempPath := r.path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o644); err != nil {
		return fmt.Errorf("write temp data file: %w", err)
	}
	if err := os.Rename(tempPath, r.path); err != nil {
		return fmt.Errorf("replace data file: %w", err)
	}
	return nil
}
