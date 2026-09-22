package runtime

import (
	"context"
	"log"
	"sync"
	"time"

	"gitrepo.xlaxiata.id/radar/radar-agent/internal/collectors"
	"gitrepo.xlaxiata.id/radar/radar-agent/internal/qan"
)

type QANSource interface {
	Start(context.Context) error
	Collect(context.Context, time.Time) (qan.CollectRequest, error)
	Stop() error
	Interval() time.Duration
}

type Module struct {
	name        string
	enabled     bool
	active      bool
	sink        collectors.Sink
	qanExamples bool
	qanSource   QANSource
	cancel      context.CancelFunc
	mu          sync.RWMutex
}

func NewModule(name string, enabled bool, active bool, sink collectors.Sink) *Module {
	return &Module{name: name, enabled: enabled, active: enabled && active, sink: sink, qanExamples: true}
}

func NewQANModule(enabled bool, sink collectors.Sink, includeExamples bool) *Module {
	return &Module{name: "qan", enabled: enabled, active: enabled, sink: sink, qanExamples: includeExamples}
}

func NewQANPostgresModule(enabled bool, sink collectors.Sink, source QANSource) *Module {
	return &Module{name: "qan", enabled: enabled, active: enabled, sink: sink, qanSource: source}
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

	if m.name == "node" && m.sink != nil {
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
	if m.name == "qan" && m.qanSource != nil {
		if err := m.qanSource.Start(loopCtx); err != nil {
			cancel()
			return err
		}
	}
	if m.name == "qan" && m.sink != nil {
		qanSink, ok := m.sink.(collectors.QANSink)
		if !ok {
			return nil
		}
		go func() {
			interval := time.Minute
			if m.qanSource != nil && m.qanSource.Interval() > 0 {
				interval = m.qanSource.Interval()
			}
			publish := func() {
				request := qan.FixtureRequest(m.qanExamples)
				if m.qanSource != nil {
					var err error
					request, err = m.qanSource.Collect(loopCtx, time.Now().Add(-time.Minute))
					if err != nil {
						log.Printf("QAN PostgreSQL collect failed: %v", err)
						return
					}
					if len(request.Buckets) == 0 {
						return
					}
				}
				if err := qanSink.PublishQAN(loopCtx, request); err != nil {
					log.Printf("QAN publish failed: %v", err)
				}
			}
			publish()
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-loopCtx.Done():
					return
				case <-ticker.C:
					publish()
				}
			}
		}()
	}
	return nil
}

func (m *Module) Stop(context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		m.cancel()
	}
	if m.qanSource != nil {
		if err := m.qanSource.Stop(); err != nil {
			return err
		}
	}
	m.active = false
	return nil
}
