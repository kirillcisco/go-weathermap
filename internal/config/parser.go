package config

import (
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

type Parser struct{}

func NewParser() *Parser {
	return &Parser{}
}

func (p *Parser) ParseYAML(r io.Reader) (*Map, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var m Map
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	if err := p.validate(&m); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &m, nil
}

func (p *Parser) validate(m *Map) error {
	if m.Width <= 0 || m.Height <= 0 {
		return fmt.Errorf("width and height of map %s must be positive", m.Title)
	}

	nodeMap := make(map[string]bool)
	for _, node := range m.Nodes {
		if node.Name == "" {
			return fmt.Errorf("node name cannot be empty")
		}
		nodeMap[node.Name] = true
	}

	for _, link := range m.Links {
		if link.Name == "" {
			return fmt.Errorf("link name cannot be empty")
		}
		if !nodeMap[link.From] {
			return fmt.Errorf("link %s references unknown node: %s", link.Name, link.From)
		}
		if !nodeMap[link.To] {
			return fmt.Errorf("link %s references unknown node: %s", link.Name, link.To)
		}
	}

	return nil
}
