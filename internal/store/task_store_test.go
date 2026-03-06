package store

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nickbarth/closedbots/internal/domain"
)

func writeTaskFile(t *testing.T, dir, name, summary, steps string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	body := "# Summary\n" + summary + "\n\n# Steps\n" + steps + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write task file: %v", err)
	}
	return path
}

func TestTaskStoreCRUD(t *testing.T) {
	dir := t.TempDir()
	s := NewTaskStore(dir)
	if err := s.EnsureDir(); err != nil {
		t.Fatalf("ensure: %v", err)
	}

	task := &domain.Task{Summary: "Alpha", Steps: []domain.Step{{Instruction: "Step A"}}}
	if err := s.Save(task); err != nil {
		t.Fatalf("save: %v", err)
	}
	if task.ID == "" {
		t.Fatalf("expected generated id")
	}

	got, err := s.Get(task.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Summary != "Alpha" || len(got.Steps) != 1 {
		t.Fatalf("unexpected task: %#v", got)
	}

	list, err := s.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len=%d", len(list))
	}

	if err := s.Export(task.ID, filepath.Join(dir, "export.md")); err != nil {
		t.Fatalf("export: %v", err)
	}
	if err := s.Delete(task.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get(task.ID); !errorsIs(err, fs.ErrNotExist) {
		t.Fatalf("expected not exist, got %v", err)
	}
}

func TestTaskStoreErrorsAndImport(t *testing.T) {
	dir := t.TempDir()
	s := NewTaskStore(dir)

	if err := s.Export("x", ""); err == nil {
		t.Fatalf("expected dest required error")
	}
	if err := s.Export("x", filepath.Join(dir, "x.txt")); err == nil {
		t.Fatalf("expected ext error")
	}
	if err := s.Delete("missing"); !errorsIs(err, fs.ErrNotExist) {
		t.Fatalf("expected fs.ErrNotExist, got %v", err)
	}
	if _, err := s.Import(filepath.Join(dir, "x.txt")); err == nil {
		t.Fatalf("expected import ext error")
	}

	invalid := filepath.Join(dir, "invalid.md")
	if err := os.WriteFile(invalid, []byte("# Steps\n1. x\n"), 0o644); err != nil {
		t.Fatalf("write invalid: %v", err)
	}
	if _, err := s.Import(invalid); err == nil {
		t.Fatalf("expected invalid markdown import error")
	}

	srcDir := t.TempDir()
	src := writeTaskFile(t, srcDir, "schedules.md", "Imported", "1. First")
	p, err := s.Import(src)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if p.ID == "schedules" {
		t.Fatalf("expected unique auto id, got %q", p.ID)
	}
}

func TestTaskStoreHelpers(t *testing.T) {
	if isTaskFileName("") {
		t.Fatalf("empty should be false")
	}
	if !isTaskFileName("x.md") {
		t.Fatalf("md should be true")
	}
	if isTaskFileName("x.txt") {
		t.Fatalf("txt should be false")
	}

	if _, err := readTask("x.txt"); err == nil {
		t.Fatalf("expected readTask ext error")
	}

	summary, steps := parseTaskMarkdown("# Summary\nA\n\n# Steps\n1. x\n")
	if summary != "A" || !strings.Contains(steps, "1. x") {
		t.Fatalf("summary=%q steps=%q", summary, steps)
	}

	md := string(renderTaskMarkdown(&domain.Task{Summary: "A", Steps: []domain.Step{{Instruction: "x"}}}))
	if !strings.Contains(md, "# Summary") || !strings.Contains(md, "# Steps") {
		t.Fatalf("rendered markdown invalid: %q", md)
	}

	if got := slugify("  Hello, World! "); got != "hello-world" {
		t.Fatalf("slugify=%q", got)
	}
	if got := baseNameNoExt("/tmp/a/b.md"); got != "b" {
		t.Fatalf("basename=%q", got)
	}
}

func TestTaskStoreInternalPathsAndWriteAtomic(t *testing.T) {
	dir := t.TempDir()
	s := NewTaskStore(dir)
	if _, err := s.findPathByIDNoLock(""); !errorsIs(err, fs.ErrNotExist) {
		t.Fatalf("expected not exist, got %v", err)
	}

	task := &domain.Task{Summary: "Dup", Steps: []domain.Step{{Instruction: "x"}}}
	if err := s.Save(task); err != nil {
		t.Fatalf("save: %v", err)
	}
	_, err := s.uniqueCreatePathNoLock("Dup")
	if err != nil {
		t.Fatalf("uniqueCreatePathNoLock: %v", err)
	}

	task2 := &domain.Task{Summary: "", CreatedAt: time.Time{}, Steps: []domain.Step{{Instruction: "x"}}}
	s.normalizeTaskForWrite(task2)
	if task2.Summary == "" || task2.SchemaVersion == 0 || task2.CreatedAt.IsZero() || task2.UpdatedAt.IsZero() {
		t.Fatalf("normalize failed: %#v", task2)
	}
	if task2.Steps[0].ID == "" || task2.Steps[0].Status == "" {
		t.Fatalf("normalize steps failed: %#v", task2.Steps[0])
	}

	path := filepath.Join(dir, "atomic.md")
	if err := writeAtomic(path, []byte("ok"), 0o644); err != nil {
		t.Fatalf("writeAtomic: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat: %v", err)
	}
	if err := writeAtomic(filepath.Join(dir, "missing", "x.md"), []byte("x"), 0o644); err == nil {
		t.Fatalf("expected create temp error")
	}
}

func TestTaskStoreEnsureDirAndListErrors(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "not-a-dir")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	s := NewTaskStore(filePath)

	if err := s.EnsureDir(); err == nil {
		t.Fatalf("expected ensure dir error")
	}
	if _, err := s.List(); err == nil {
		t.Fatalf("expected list ensure-dir error")
	}
	if err := s.Save(&domain.Task{Summary: "x"}); err == nil {
		t.Fatalf("expected save ensure-dir error")
	}
	if _, err := s.Import(filepath.Join(root, "x.md")); err == nil {
		t.Fatalf("expected import ensure-dir error")
	}
}

func TestTaskStoreListSkipsBadEntriesAndSortsTiesByID(t *testing.T) {
	dir := t.TempDir()
	s := NewTaskStore(dir)
	if err := s.EnsureDir(); err != nil {
		t.Fatalf("ensure: %v", err)
	}

	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not a task"), 0o644); err != nil {
		t.Fatalf("write txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.md"), []byte("# Steps\n1. x\n"), 0o644); err != nil {
		t.Fatalf("write broken md: %v", err)
	}
	writeTaskFile(t, dir, "b.md", "Same Summary", "1. one")
	writeTaskFile(t, dir, "a.md", "Same Summary", "1. two")

	list, err := s.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 parsed tasks, got %d", len(list))
	}
	if list[0].ID != "a" || list[1].ID != "b" {
		t.Fatalf("expected ID tie sort by id, got %q then %q", list[0].ID, list[1].ID)
	}
}

func TestTaskStoreSaveUpdateAndPathLookupBranches(t *testing.T) {
	dir := t.TempDir()
	s := NewTaskStore(dir)
	if err := s.EnsureDir(); err != nil {
		t.Fatalf("ensure: %v", err)
	}

	task := &domain.Task{Summary: "Original", Steps: []domain.Step{{Instruction: "x"}}}
	if err := s.Save(task); err != nil {
		t.Fatalf("save first: %v", err)
	}
	firstID := task.ID
	firstPath, err := s.findPathByIDNoLock(firstID)
	if err != nil {
		t.Fatalf("find first path: %v", err)
	}

	task.Summary = "Updated"
	if err := s.Save(task); err != nil {
		t.Fatalf("save update: %v", err)
	}
	secondPath, err := s.findPathByIDNoLock(firstID)
	if err != nil {
		t.Fatalf("find second path: %v", err)
	}
	if firstPath != secondPath {
		t.Fatalf("expected update in-place, got %q then %q", firstPath, secondPath)
	}

	nonDir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(nonDir, []byte("x"), 0o644); err != nil {
		t.Fatalf("write non-dir: %v", err)
	}
	s2 := NewTaskStore(nonDir)
	if _, err := s2.findPathByIDNoLock("task-id"); err == nil {
		t.Fatalf("expected read dir error for non-directory path")
	}
}

func TestTaskStoreImportAndExportErrorBranches(t *testing.T) {
	dir := t.TempDir()
	s := NewTaskStore(dir)
	if err := s.EnsureDir(); err != nil {
		t.Fatalf("ensure: %v", err)
	}

	task := &domain.Task{Summary: "Existing", Steps: []domain.Step{{Instruction: "x"}}}
	if err := s.Save(task); err != nil {
		t.Fatalf("save existing: %v", err)
	}

	importPath := filepath.Join(t.TempDir(), task.ID+".md")
	if err := os.WriteFile(importPath, []byte("# Summary\nImported Existing\n\n# Steps\n1. updated\n"), 0o644); err != nil {
		t.Fatalf("write import existing: %v", err)
	}
	imported, err := s.Import(importPath)
	if err != nil {
		t.Fatalf("import existing id: %v", err)
	}
	if imported.ID != task.ID {
		t.Fatalf("expected import overwrite existing ID %q, got %q", task.ID, imported.ID)
	}

	newImportPath := filepath.Join(t.TempDir(), "fresh.md")
	if err := os.WriteFile(newImportPath, []byte("# Summary\nFresh\n\n# Steps\n1. a\n"), 0o644); err != nil {
		t.Fatalf("write import fresh: %v", err)
	}
	newImported, err := s.Import(newImportPath)
	if err != nil {
		t.Fatalf("import fresh: %v", err)
	}
	if newImported.ID != "fresh" {
		t.Fatalf("expected imported id fresh, got %q", newImported.ID)
	}

	badParent := filepath.Join(dir, "bad-parent")
	if err := os.WriteFile(badParent, []byte("x"), 0o644); err != nil {
		t.Fatalf("write bad parent: %v", err)
	}
	if err := s.Export(task.ID, filepath.Join(badParent, "dest.md")); err == nil {
		t.Fatalf("expected export mkdir error")
	}
}

func TestTaskStoreReadAndFindPathExtraBranches(t *testing.T) {
	s := NewTaskStore(filepath.Join(t.TempDir(), "missing-dir"))
	if _, err := s.findPathByIDNoLock(""); !errorsIs(err, fs.ErrNotExist) {
		t.Fatalf("expected empty id not-exist, got %v", err)
	}
	if _, err := s.findPathByIDNoLock("something"); !errorsIs(err, fs.ErrNotExist) {
		t.Fatalf("expected missing dir not-exist, got %v", err)
	}

	nonexistent := filepath.Join(t.TempDir(), "missing.md")
	if _, err := readTaskMarkdown(nonexistent); err == nil {
		t.Fatalf("expected readTaskMarkdown read error")
	}
}

func errorsIs(err, target error) bool {
	return errors.Is(err, target)
}
