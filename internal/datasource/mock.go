package datasource

import (
	"context"
	"math/rand"
	"time"
)

type MockDataSource struct{}

func NewMockDataSource(config map[string]any) *MockDataSource {
	return &MockDataSource{}
}

func (m *MockDataSource) GetTraffic(ctx context.Context) (*TrafficData, error) {
	utilization := rand.Float64() * 100

	return &TrafficData{
		InBytes:     1000000, // 1MB/s
		OutBytes:    800000,  // 800KB/s
		Timestamp:   time.Now(),
		Utilization: utilization,
	}, nil
}

func (m *MockDataSource) GetStatus(ctx context.Context) (*NodeStatus, error) {
	return &NodeStatus{
		Status:    "up",
		Timestamp: time.Now(),
	}, nil
}
