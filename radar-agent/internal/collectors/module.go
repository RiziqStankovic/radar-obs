package collectors

import "context"

type Status string

const (
	StatusDisabled Status = "disabled"
	StatusStarting Status = "starting"
	StatusActive   Status = "active"
	StatusStandby  Status = "standby"
	StatusFailed   Status = "failed"
	StatusStopped  Status = "stopped"
)

type Event struct {
	Signal  string         `json:"signal"`
	Source  string         `json:"source"`
	Payload map[string]any `json:"payload"`
}

type Sink interface {
	Publish(context.Context, Event) error
}

type Module interface {
	Name() string
	Start(context.Context) error
	Stop(context.Context) error
	Status() Status
}
