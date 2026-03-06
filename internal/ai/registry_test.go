package ai

import "testing"

func TestRegistryBuildKnownProviders(t *testing.T) {
	r := NewRegistry()
	for _, id := range []string{ProviderCodexCLI, ProviderClaudeCLI, ProviderOllamaCLI} {
		p, err := r.Build(id, t.TempDir())
		if err != nil {
			t.Fatalf("build %s: %v", id, err)
		}
		if p == nil {
			t.Fatalf("provider %s nil", id)
		}
	}
}

func TestRegistryRegisterAndUnknown(t *testing.T) {
	r := &Registry{}
	r.Register("custom", func(workDir string) (Provider, error) { return NewCodexCLI(workDir), nil })
	if _, err := r.Build("custom", t.TempDir()); err != nil {
		t.Fatalf("build custom: %v", err)
	}
	if _, err := r.Build("missing", t.TempDir()); err == nil {
		t.Fatalf("expected unknown provider error")
	}
}
