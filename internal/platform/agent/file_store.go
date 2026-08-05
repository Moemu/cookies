package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type FileStore struct {
	mu   sync.Mutex
	path string
	runs map[string]AgentRun
}

func NewFileStore(path string) (*FileStore, error) {
	if path == "" {
		return nil, fmt.Errorf("agent file store path is required")
	}
	store := &FileStore{path: path, runs: make(map[string]AgentRun)}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *FileStore) Save(ctx context.Context, run AgentRun) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := run.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[run.ID] = cloneRun(run)
	return s.persist()
}

func (s *FileStore) Get(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string) (AgentRun, error) {
	if err := ctx.Err(); err != nil {
		return AgentRun{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.runs[id]
	if !ok || run.OrganizationID != actor.OrganizationID || run.ProjectID != projectID {
		return AgentRun{}, ErrRunNotFound
	}
	return cloneRun(run), nil
}

func (s *FileStore) List(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, limit int) ([]AgentRun, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf("limit must be between 1 and 100")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	runs := make([]AgentRun, 0, len(s.runs))
	for _, run := range s.runs {
		if run.OrganizationID == actor.OrganizationID && run.ProjectID == projectID {
			runs = append(runs, cloneRun(run))
		}
	}
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].CreatedAt.After(runs[j].CreatedAt)
	})
	if len(runs) > limit {
		runs = runs[:limit]
	}
	return runs, nil
}

func (s *FileStore) load() error {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var snapshot fileStoreSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return fmt.Errorf("decode agent file store: %w", err)
	}
	for _, run := range snapshot.Runs {
		if err := run.Validate(); err != nil {
			return fmt.Errorf("decode agent run %q: %w", run.ID, err)
		}
		s.runs[run.ID] = cloneRun(run)
	}
	return nil
}

func (s *FileStore) persist() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	runs := make([]AgentRun, 0, len(s.runs))
	for _, run := range s.runs {
		runs = append(runs, cloneRun(run))
	}
	sort.Slice(runs, func(i, j int) bool {
		if runs[i].CreatedAt.Equal(runs[j].CreatedAt) {
			return runs[i].ID < runs[j].ID
		}
		return runs[i].CreatedAt.Before(runs[j].CreatedAt)
	})
	data, err := json.MarshalIndent(fileStoreSnapshot{Runs: runs}, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

type fileStoreSnapshot struct {
	Runs []AgentRun `json:"runs"`
}
