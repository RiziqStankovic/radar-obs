package runtime

import (
	"context"
	"sync"
	"time"

	"gitrepo.xlaxiata.id/radar/radar-agent/internal/collectors"
)

type Module struct {
	name    string
	enabled bool
	active  bool
	sink    collectors.Sink
	cancel  context.CancelFunc
	mu      sync.RWMutex
}

func NewModule(name string, enabled bool, active bool, sink collectors.Sink) *Module {
	return &Module{name: name, enabled: enabled, active: enabled && active, sink: sink}
}

func (m *Module) Name() string { return m.name }

func (m *Module) Status() collectors.Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.enabled {
		return collectors.StatusDisabled
	}
	if !m.active {
		return collectors.StatusStandby
	}
	return collectors.StatusActive
}

func (m *Module) Start(ctx context.Context) error {
	m.mu.Lock()
	if !m.active {
		m.mu.Unlock()
		return nil
	}
	loopCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.mu.Unlock()

	if m.name != "node" || m.sink == nil {
		return nil
	}
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-loopCtx.Done():
				return
			case <-ticker.C:
				_ = m.sink.Publish(loopCtx, collectors.Event{Signal: "metrics", Source: "node", Payload: map[string]any{"heartbeat": 1}})
			}
		}
	}()
	return nil
}

func (m *Module) Stop(context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		m.cancel()
	}
	m.active = false
	return nil
}
