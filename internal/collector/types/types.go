package types

import "context"

type Publisher interface {
	Publish(ctx context.Context, subject string, data any) error
}

type Collector interface {
	Name() string
	Start() error
	Stop() error
	Health() error
}
