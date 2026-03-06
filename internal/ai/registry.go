package ai

import (
	"fmt"
	"strings"
	"sync"
)

const ProviderCodexCLI = "codex_cli"
const ProviderClaudeCLI = "claude_cli"
const ProviderOllamaCLI = "ollama_cli"

type Factory func(workDir string) (Provider, error)

type Registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

func NewRegistry() *Registry {
	r := &Registry{
		factories: map[string]Factory{},
	}
	r.Register(ProviderCodexCLI, func(workDir string) (Provider, error) {
		return NewCodexCLI(workDir), nil
	})
	r.Register(ProviderClaudeCLI, func(workDir string) (Provider, error) {
		return NewClaudeCLI(workDir), nil
	})
	r.Register(ProviderOllamaCLI, func(workDir string) (Provider, error) {
		return NewOllamaCLI(workDir), nil
	})
	return r
}

func (r *Registry) Register(providerID string, f Factory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.factories == nil {
		r.factories = map[string]Factory{}
	}
	r.factories[strings.ToLower(strings.TrimSpace(providerID))] = f
}

func (r *Registry) Build(providerID, workDir string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id := strings.ToLower(strings.TrimSpace(providerID))
	f, ok := r.factories[id]
	if !ok {
		return nil, fmt.Errorf("unknown provider %q", providerID)
	}
	return f(workDir)
}
