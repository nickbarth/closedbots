package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nickbarth/closedbots/internal/domain"
)

var slugNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

const (
	taskExtMarkdown = ".md"
)

type TaskStore struct {
	dir string
	mu  sync.RWMutex
}

func NewTaskStore(dir string) *TaskStore {
	return &TaskStore{dir: dir}
}

func (s *TaskStore) EnsureDir() error {
	return os.MkdirAll(s.dir, 0o755)
}

func (s *TaskStore) List() ([]*domain.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := s.EnsureDir(); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("read task dir: %w", err)
	}

	out := make([]*domain.Task, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !isTaskFileName(name) {
			continue
		}
		path := filepath.Join(s.dir, name)
		p, err := readTask(path)
		if err != nil {
			continue
		}
		out = append(out, p)
	}

	sort.Slice(out, func(i, j int) bool {
		left := strings.ToLower(strings.TrimSpace(out[i].Summary))
		right := strings.ToLower(strings.TrimSpace(out[j].Summary))
		if left == right {
			return strings.ToLower(out[i].ID) < strings.ToLower(out[j].ID)
		}
		return left < right
	})
	return out, nil
}

func (s *TaskStore) Get(taskID string) (*domain.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path, err := s.findPathByIDNoLock(taskID)
	if err != nil {
		return nil, err
	}
	return readTask(path)
}

func (s *TaskStore) Save(p *domain.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.EnsureDir(); err != nil {
		return err
	}
	s.normalizeTaskForWrite(p)

	existingPath, err := s.findPathByIDNoLock(p.ID)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	destPath := ""
	switch {
	case err == nil && strings.HasSuffix(strings.ToLower(existingPath), taskExtMarkdown):
		destPath = existingPath
	default:
		destPath, err = s.uniqueCreatePathNoLock(p.Summary)
		if err != nil {
			return err
		}
	}

	p.ID = baseNameNoExt(destPath)
	b := renderTaskMarkdown(p)
	if err := writeAtomic(destPath, b, 0o644); err != nil {
		return err
	}
	return nil
}

func (s *TaskStore) Import(srcPath string) (*domain.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.EnsureDir(); err != nil {
		return nil, err
	}
	if strings.ToLower(filepath.Ext(srcPath)) != taskExtMarkdown {
		return nil, fmt.Errorf("import only supports markdown task files (.md)")
	}

	p, err := readTask(srcPath)
	if err != nil {
		return nil, fmt.Errorf("read import task: %w", err)
	}
	s.normalizeTaskForWrite(p)

	existingPath, findErr := s.findPathByIDNoLock(p.ID)
	if findErr != nil && !errors.Is(findErr, fs.ErrNotExist) {
		return nil, findErr
	}

	destPath := ""
	switch {
	case findErr == nil && strings.HasSuffix(strings.ToLower(existingPath), taskExtMarkdown):
		destPath = existingPath
	default:
		base := baseNameNoExt(filepath.Base(srcPath))
		if strings.TrimSpace(base) == "" || base == "schedules" {
			destPath, err = s.uniqueCreatePathNoLock(p.Summary)
			if err != nil {
				return nil, err
			}
		} else {
			destPath = filepath.Join(s.dir, base+taskExtMarkdown)
		}
	}

	p.ID = baseNameNoExt(destPath)
	b := renderTaskMarkdown(p)
	if err := writeAtomic(destPath, b, 0o644); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *TaskStore) Export(taskID, destPath string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if strings.TrimSpace(destPath) == "" {
		return fmt.Errorf("destination path is required")
	}
	if strings.ToLower(filepath.Ext(destPath)) != taskExtMarkdown {
		return fmt.Errorf("export only supports markdown destination files (.md)")
	}
	path, err := s.findPathByIDNoLock(taskID)
	if err != nil {
		return err
	}
	p, err := readTask(path)
	if err != nil {
		return err
	}
	s.normalizeTaskForWrite(p)
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	return writeAtomic(destPath, renderTaskMarkdown(p), 0o644)
}

func (s *TaskStore) Delete(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := s.findPathByIDNoLock(taskID)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	return nil
}

func (s *TaskStore) findPathByIDNoLock(taskID string) (string, error) {
	if strings.TrimSpace(taskID) == "" {
		return "", fs.ErrNotExist
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", fs.ErrNotExist
		}
		return "", err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !isTaskFileName(name) {
			continue
		}
		path := filepath.Join(s.dir, name)
		p, err := readTask(path)
		if err != nil {
			continue
		}
		if p.ID == taskID {
			return path, nil
		}
	}
	return "", fs.ErrNotExist
}

func isTaskFileName(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "" {
		return false
	}
	return strings.HasSuffix(lower, taskExtMarkdown)
}

func readTask(path string) (*domain.Task, error) {
	if strings.ToLower(filepath.Ext(path)) != taskExtMarkdown {
		return nil, fmt.Errorf("unsupported task file extension: %s", filepath.Ext(path))
	}
	return readTaskMarkdown(path)
}

func readTaskMarkdown(path string) (*domain.Task, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	summary, stepsText := parseTaskMarkdown(string(b))
	if strings.TrimSpace(summary) == "" {
		return nil, fmt.Errorf("markdown task missing summary")
	}
	instructions := domain.ParsePointFormSteps(stepsText)
	steps := make([]domain.Step, 0, len(instructions))
	for i, instruction := range instructions {
		steps = append(steps, domain.Step{
			ID:          fmt.Sprintf("step_%03d", i+1),
			Instruction: instruction,
			Status:      domain.StepPending,
		})
	}

	mod := time.Now().UTC()
	if fi, statErr := os.Stat(path); statErr == nil {
		mod = fi.ModTime().UTC()
	}
	return &domain.Task{
		SchemaVersion: domain.SchemaVersionTask,
		ID:            baseNameNoExt(path),
		Summary:       summary,
		CreatedAt:     mod,
		UpdatedAt:     mod,
		Steps:         steps,
	}, nil
}

func parseTaskMarkdown(md string) (summary string, stepsText string) {
	text := strings.ReplaceAll(md, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	section := ""
	summaryLines := []string{}
	stepLines := []string{}
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		switch strings.ToLower(trim) {
		case "# summary":
			section = "summary"
			continue
		case "# steps":
			section = "steps"
			continue
		}
		switch section {
		case "summary":
			summaryLines = append(summaryLines, line)
		case "steps":
			stepLines = append(stepLines, line)
		}
	}
	return strings.TrimSpace(strings.Join(summaryLines, "\n")), strings.TrimSpace(strings.Join(stepLines, "\n"))
}

func renderTaskMarkdown(p *domain.Task) []byte {
	summary := strings.TrimSpace(p.Summary)
	steps := domain.FormatPointFormSteps(p.Steps)
	out := "# Summary\n" + summary + "\n\n# Steps\n"
	if strings.TrimSpace(steps) != "" {
		out += steps + "\n"
	}
	return []byte(out)
}

func (s *TaskStore) uniqueCreatePathNoLock(summary string) (string, error) {
	slug := slugify(summary)
	if slug == "" {
		slug = "task"
	}
	base := fmt.Sprintf("automation-%s", slug)
	path := filepath.Join(s.dir, base+taskExtMarkdown)
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return path, nil
	}
	for i := 2; i < 10000; i++ {
		candidate := filepath.Join(s.dir, fmt.Sprintf("%s-%d%s", base, i, taskExtMarkdown))
		if _, err := os.Stat(candidate); errors.Is(err, fs.ErrNotExist) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not allocate unique task filename for %q", summary)
}

func (s *TaskStore) normalizeTaskForWrite(p *domain.Task) {
	now := time.Now().UTC()
	if p.SchemaVersion == 0 {
		p.SchemaVersion = domain.SchemaVersionTask
	}
	if strings.TrimSpace(p.Summary) == "" {
		p.Summary = "Untitled Task"
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	for i := range p.Steps {
		if p.Steps[i].ID == "" {
			p.Steps[i].ID = fmt.Sprintf("step_%03d", i+1)
		}
		if p.Steps[i].Status == "" {
			p.Steps[i].Status = domain.StepPending
		}
	}
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugNonAlnum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

func baseNameNoExt(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "tmp-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		_ = os.Remove(tmpName)
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	return nil
}
