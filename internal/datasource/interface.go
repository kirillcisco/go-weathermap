package datasource

import (
	"context"
	"time"
)

type DataSource interface {
	GetTraffic(ctx context.Context) (*TrafficData, error)
	GetStatus(ctx context.Context) (*NodeStatus, error)
}

type TrafficData struct {
	InBytes     int64     `json:"in_bytes"`
	OutBytes    int64     `json:"out_bytes"`
	Timestamp   time.Time `json:"timestamp"`
	Utilization float64   `json:"utilization"` // процент 0-100
}

type NodeStatus struct {
	Status    string    `json:"status"` // up, down, unknown
	Timestamp time.Time `json:"timestamp"`
}
