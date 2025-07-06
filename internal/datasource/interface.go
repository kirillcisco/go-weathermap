package datasource

import (
	"context"

	"go-weathermap/internal/config"
)

type DataSource interface {
	GetTraffic(ctx context.Context) (*config.TrafficData, error)
	GetStatus(ctx context.Context) (*config.NodeStatus, error)
}
