package core

import (
	"context"
	"log"
	"sync"
	"time"

	"go-weathermap/internal/config"
	"go-weathermap/internal/datasource"
)

type MapProcessor struct {
	config      *config.Map
	datasources map[string]datasource.DataSource
	mu          sync.RWMutex
	lastUpdate  time.Time
}

type ProcessedMap struct {
	Config      *config.Map     `json:"config"`
	ProcessedAt time.Time       `json:"processed_at"`
	Nodes       []ProcessedNode `json:"nodes"`
	Links       []ProcessedLink `json:"links"`
	Status      string          `json:"status"`
}

type ProcessedNode struct {
	config.Node
	Status     string    `json:"status"` // up, down, unknown
	LastUpdate time.Time `json:"last_update"`
}

type ProcessedLink struct {
	config.Link
	InTraffic   int64        `json:"in_traffic"`
	OutTraffic  int64        `json:"out_traffic"`
	Utilization float64      `json:"utilization"` // 0-100%
	Color       config.Color `json:"color"`
	Status      string       `json:"status"` // up, down, unknown
	LastUpdate  time.Time    `json:"last_update"`
}

func NewMapProcessor(cfg *config.Map) *MapProcessor {
	mp := &MapProcessor{
		config:      cfg,
		datasources: make(map[string]datasource.DataSource),
	}

	for _, link := range cfg.Links {
		if link.DataSource != nil {
			ds := mp.createDataSource(link.DataSource)
			if ds != nil {
				mp.datasources[link.Name] = ds
			}
		}
	}

	return mp
}

func (mp *MapProcessor) createDataSource(ref *config.DataSourceRef) datasource.DataSource {
	switch ref.Type {
	case "mock":
		return datasource.NewMockDataSource(nil)
	case "zabbix":
		log.Printf("Zabbix not implemented yet")
		return datasource.NewMockDataSource(nil)
	default:
		return datasource.NewMockDataSource(nil)
	}
}

func (mp *MapProcessor) Process(ctx context.Context) (*ProcessedMap, error) {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	processed := &ProcessedMap{
		Config:      mp.config,
		ProcessedAt: time.Now(),
		Status:      "ok",
		Nodes:       make([]ProcessedNode, len(mp.config.Nodes)),
		Links:       make([]ProcessedLink, len(mp.config.Links)),
	}

	for i, node := range mp.config.Nodes {
		processed.Nodes[i] = ProcessedNode{
			Node:       node,
			Status:     "up", // Пока всегда up
			LastUpdate: time.Now(),
		}
	}

	for i, link := range mp.config.Links {
		processedLink := ProcessedLink{
			Link:       link,
			Status:     "unknown",
			LastUpdate: time.Now(),
		}

		if ds, exists := mp.datasources[link.Name]; exists {
			if traffic, err := ds.GetTraffic(ctx); err == nil {
				processedLink.InTraffic = traffic.InBytes
				processedLink.OutTraffic = traffic.OutBytes
				processedLink.Utilization = traffic.Utilization
				processedLink.Status = "up"
				processedLink.Color = mp.getColorForUtilization(processedLink.Utilization)
			} else {
				log.Printf("Failed to get traffic for link %s: %v", link.Name, err)
				processedLink.Status = "down"
				// grey color for unknown datasource
				processedLink.Color = config.Color{R: 128, G: 128, B: 128}
			}
		}

		processed.Links[i] = processedLink
	}

	mp.lastUpdate = time.Now()
	return processed, nil
}

func (mp *MapProcessor) getColorForUtilization(utilization float64) config.Color {
	defaultScale := mp.config.Scales["default"]
	if defaultScale == nil {
		if utilization < 50 {
			return config.Color{R: 0, G: 255, B: 0}
		} else if utilization < 80 {
			return config.Color{R: 255, G: 255, B: 0}
		}
		return config.Color{R: 255, G: 0, B: 0}
	}

	for _, scale := range defaultScale {
		if utilization >= scale.Min && utilization < scale.Max {
			return scale.Color
		}
	}

	if len(defaultScale) > 0 {
		return defaultScale[len(defaultScale)-1].Color
	}

	// return grey by default
	return config.Color{R: 128, G: 128, B: 128}
}
