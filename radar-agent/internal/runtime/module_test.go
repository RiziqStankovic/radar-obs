package runtime

import (
	"context"
	"testing"
	"time"

	"gitrepo.xlaxiata.id/radar/radar-agent/internal/collectors"
	"gitrepo.xlaxiata.id/radar/radar-agent/internal/qan"
)

type recordingSink struct {
	events chan collectors.Event
	qan    chan qan.CollectRequest
}

func newRecordingSink() *recordingSink {
	return &recordingSink{events: make(chan collectors.Event, 4), qan: make(chan qan.CollectRequest, 4)}
}

func (s *recordingSink) Publish(_ context.Context, event collectors.Event) error {
	s.events <- event
	return nil
}

func (s *recordingSink) PublishQAN(_ context.Context, request qan.CollectRequest) error {
	s.qan <- request
	return nil
}

func TestQANModuleDisabledDoesNotPublish(t *testing.T) {
	sink := newRecordingSink()
	module := NewQANModule(false, sink, true)
	if err := module.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case request := <-sink.qan:
		t.Fatalf("QAN request = %#v, want none", request)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestQANModulePublishesFixtureWhenEnabled(t *testing.T) {
	sink := newRecordingSink()
	module := NewQANModule(true, sink, false)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := module.Start(ctx); err != nil {
		t.Fatal(err)
	}
	var request qan.CollectRequest
	select {
	case request = <-sink.qan:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for QAN publish")
	}
	if len(request.Buckets) != 1 {
		t.Fatalf("buckets = %d, want 1", len(request.Buckets))
	}
	if request.Buckets[0].Example != "" {
		t.Fatalf("Example = %q, want empty when examples disabled", request.Buckets[0].Example)
	}
	if request.Buckets[0].Fingerprint == "" || request.Buckets[0].NumQueries == 0 {
		t.Fatalf("fixture lost fingerprint or metrics: %#v", request.Buckets[0])
	}
}
