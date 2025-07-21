package config

import (
	"fmt"
	"io"
	"regexp"

	"gopkg.in/yaml.v3"
)

type Parser struct{}

func NewParser() *Parser {
	return &Parser{}
}

func (p *Parser) ParseYAML(r io.Reader) (*Map, error) {
	var m Map
	decoder := yaml.NewDecoder(r)
	if err := decoder.Decode(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

func (p *Parser) Validate(m *Map) error {
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
		if err := validateBandwidth(link.Bandwidth); err != nil {
			return fmt.Errorf("link '%s': %w", link.Name, err)
		}
	}

	return nil
}

var bandwidthParserRegex = regexp.MustCompile(`^(\d+)(M|G|T)$`)

func validateBandwidth(bandwidth string) error {
	if !bandwidthParserRegex.MatchString(bandwidth) {
		return fmt.Errorf("invalid bandwidth format: '%s', must be like '100M', '1G' or '1T'", bandwidth)
	}
	return nil
}
