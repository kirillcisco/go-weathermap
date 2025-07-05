package config

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

type Map struct {
	Width   int                `yaml:"width" json:"width"`
	Height  int                `yaml:"height" json:"height"`
	Title   string             `yaml:"title" json:"title"`
	BGColor *Color             `yaml:"bg_color,omitempty,flow" json:"bgcolor,omitempty"`
	Scales  map[string][]Scale `yaml:"scales,omitempty" json:"scales,omitempty"`

	Nodes []Node `yaml:"nodes" json:"nodes"`
	Links []Link `yaml:"links" json:"links"`

	// Global variables (like zabbix creds)
	Variables map[string]string `yaml:"variables,omitempty" json:"variables,omitempty"`
}

type Color struct {
	R int `yaml:"r"`
	G int `yaml:"g"`
	B int `yaml:"b"`
}

type Legend struct {
	Position     Position `yaml:"position"`
	Title        string   `yaml:"title"`
	TextColor    *Color   `yaml:"text_color,omitempty"`
	OutlineColor *Color   `yaml:"outline_color,omitempty"`
	BGColor      *Color   `yaml:"bg_color,omitempty"`
	HideZero     bool     `yaml:"hide_zero,omitempty"`
}

type Scale struct {
	Name  string  `yaml:"name"`
	Min   float64 `yaml:"min"`
	Max   float64 `yaml:"max"`
	Color Color   `yaml:"color"`
}

type Position struct {
	X int `yaml:"x" json:"x"`
	Y int `yaml:"y" json:"y"`
}

type Defaults struct {
	Node *NodeDefaults `yaml:"node,omitempty"`
	Link *LinkDefaults `yaml:"link,omitempty"`
}

type NodeDefaults struct {
	MaxValue   int    `yaml:"max_value,omitempty"`
	Icon       string `yaml:"icon,omitempty"`
	Monitoring bool   `yaml:"monitoring"`
}

type LinkDefaults struct {
	Width      int      `yaml:"width,omitempty"`
	ArrowStyle string   `yaml:"arrow_style,omitempty"`
	BWLabel    string   `yaml:"bw_label,omitempty"`
	BWLabelPos Position `yaml:"bw_label_pos,omitempty"`
	Bandwidth  string   `yaml:"bandwidth,omitempty"`
}

type Node struct {
	Name       string   `yaml:"name"`
	Label      string   `yaml:"label,omitempty"`
	Position   Position `yaml:"position,flow"`
	Icon       string   `yaml:"icon,omitempty"`
	Monitoring bool     `yaml:"monitoring"`
	MaxValue   int      `yaml:"max_value,omitempty"`
}

type Link struct {
	Name         string         `yaml:"name"`
	From         string         `yaml:"from"`
	To           string         `yaml:"to"`
	DataSource   *DataSourceRef `yaml:"datasource,omitempty"`
	OverlibGraph *DataSourceRef `yaml:"overlib_graph,omitempty"`
	Bandwidth    string         `yaml:"bandwidth,omitempty"`
	Width        int            `yaml:"width,omitempty"`
	BWLabelPos   *Position      `yaml:"bw_label_pos,omitempty"`
	Via          []Position     `yaml:"via,omitempty,flow"`
	Scale        string         `yaml:"scale,omitempty"`
}

type DataSourceRef struct {
	Type            string         `yaml:"type"`
	RefreshInterval time.Duration  `yaml:"refresh_interval,omitempty"`
	Config          map[string]any `yaml:"config,omitempty,flow"`
}

type TrafficData struct {
	InBytes     int64     `yaml:"-"`
	OutBytes    int64     `yaml:"-"`
	Timestamp   time.Time `yaml:"-"`
	Utilization float64   `yaml:"-"`
}

type MapWithData struct {
	*Map
	ProcessedAt time.Time  `json:"processed_at"`
	LinksData   []LinkData `json:"links_data"`
}

type LinkData struct {
	Name        string  `json:"name"`
	Utilization float64 `json:"utilization"`
	Status      string  `json:"status"`
}

func (p Position) MarshalYAML() (interface{}, error) {
	node := &yaml.Node{
		Kind:  yaml.MappingNode,
		Style: yaml.FlowStyle,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "x"},
			{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%d", p.X)},
			{Kind: yaml.ScalarNode, Value: "y"},
			{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%d", p.Y)},
		},
	}
	return node, nil
}
