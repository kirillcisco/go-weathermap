package datasource

import (
	"context"
	"math/rand"
	"time"

	"go-weathermap/internal/config"
)

type MockClient struct{}

func NewMockClient() *MockClient {
	return &MockClient{}
}

func (c *MockClient) GetTraffic(ctx context.Context) (*config.TrafficData, error) {
	inBps := rand.Int63n(50000)  // <~400 Kbps
	outBps := rand.Int63n(50000) // <~400 Kbps

	return &config.TrafficData{
		InBytes:   inBps,
		OutBytes:  outBps,
		Timestamp: time.Now(),
	}, nil
}

func (c *MockClient) GetStatus(ctx context.Context) (*config.NodeStatus, error) {
	return &config.NodeStatus{
		Status:    "up",
		Timestamp: time.Now(),
	}, nil
}
