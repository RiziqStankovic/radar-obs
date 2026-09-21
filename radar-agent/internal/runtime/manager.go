package runtime

import (
	"context"
	"fmt"
	"sync"

	"gitrepo.xlaxiata.id/radar/radar-agent/internal/collectors"
)

type Manager struct {
	modules []collectors.Module
	mu      sync.RWMutex
}

func New(modules ...collectors.Module) *Manager {
	return &Manager{modules: modules}
}

func (m *Manager) Start(ctx context.Context) error {
	for _, module := range m.modules {
		if err := module.Start(ctx); err != nil {
			return fmt.Errorf("start %s: %w", module.Name(), err)
		}
	}
	return nil
}

func (m *Manager) Stop(ctx context.Context) error {
	for i := len(m.modules) - 1; i >= 0; i-- {
		if err := m.modules[i].Stop(ctx); err != nil {
			return fmt.Errorf("stop %s: %w", m.modules[i].Name(), err)
		}
	}
	return nil
}

func (m *Manager) Status() map[string]collectors.Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]collectors.Status, len(m.modules))
	for _, module := range m.modules {
		result[module.Name()] = module.Status()
	}
	return result
}
