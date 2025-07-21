package service

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"go-weathermap/internal/config"
	"go-weathermap/internal/utils"

	"gopkg.in/yaml.v3"
)

type MapService struct {
	configDir string
	iconsDir  string
	parser    *config.Parser
}

func NewMapService(configDir string) *MapService {
	absConfigDir, _ := filepath.Abs(configDir)
	iconsDir := filepath.Join(filepath.Dir(absConfigDir), "internal", "assets", "icons")

	return &MapService{
		configDir: configDir,
		iconsDir:  iconsDir,
		parser:    config.NewParser(),
	}
}

func (s *MapService) ListMaps() ([]string, error) {
	files, err := filepath.Glob(filepath.Join(s.configDir, "*.yaml"))
	if err != nil {
		return nil, err
	}

	maps := make([]string, 0, len(files))
	for _, file := range files {
		name := filepath.Base(file)
		name = name[:len(name)-len(filepath.Ext(name))]
		maps = append(maps, name)
	}

	return maps, nil
}

func (s *MapService) GetMap(name string) (*config.Map, error) {
	return s.loadMapConfig(name)
}

func (s *MapService) GetMapWithData(name string, dsService *DataSourceService) (*config.MapWithData, error) {
	mapConfig, err := s.loadMapConfig(name)
	if err != nil {
		return nil, err
	}
	linksData := make([]config.LinkData, 0, len(mapConfig.Links))
	for _, link := range mapConfig.Links {
		linkData := config.LinkData{
			Name:   link.Name,
			Status: "unknown",
		}

		if dsService != nil && link.DataSource != "" && link.Interface != "" && len(link.Metrics) > 0 {
			fmt.Printf("[MAP DEBUG] link=%s ds=%s iface=%s metrics=%v\n", link.Name, link.DataSource, link.Interface, link.Metrics)

			metrics, err := dsService.GetInterfaceMetrics(
				context.Background(), link.DataSource, link.Interface, link.Metrics)

			fmt.Printf("[MAP DEBUG] metrics result: %v err: %v\n", metrics, err)

			if err == nil {
				linkData.Status = "up"
				linkData.Metrics = metrics

				if inVal, okIn := metrics["in"].(int64); okIn {
					if outVal, okOut := metrics["out"].(int64); okOut {
						bw := utils.ParseBandwidth(link.Bandwidth)
						if bw > 0 {
							utilization := float64(max(inVal, outVal)) / float64(bw) * 100
							linkData.Utilization = math.Round(utilization*10) / 10
						}
					}
				}
			} else {
				linkData.Status = "down"
				fmt.Printf("[ERROR] Failed to get metrics for link %s: %v\n", link.Name, err)
			}
		}
		linksData = append(linksData, linkData)
	}
	return &config.MapWithData{
		Map:         mapConfig,
		ProcessedAt: time.Now(),
		LinksData:   linksData,
	}, nil
}

func (s *MapService) CreateMap(newMap *config.Map, mapName string) error {
	if newMap.Width <= 0 || newMap.Height <= 0 {
		return fmt.Errorf("width and Height of map must be greater than 0")
	}
	if newMap.Title == "" {
		return fmt.Errorf("title for map is required")
	}
	return s.saveMap(mapName, newMap)
}

func (s *MapService) ReplaceMap(mapName string, replaceMap *config.Map) error {
	if replaceMap.Width <= 0 || replaceMap.Height <= 0 {
		return fmt.Errorf("width and Height of map must be greater than 0")
	}
	if replaceMap.Title == "" {
		return fmt.Errorf("title for map is required")
	}
	return s.saveMap(mapName, replaceMap)
}

func (s *MapService) DeleteMap(mapName string) error {
	configPath := filepath.Join(s.configDir, mapName+".yaml")
	return os.Remove(configPath)
}

func (s *MapService) AddNode(mapName string, newNode *config.Node) error {
	mapConfig, err := s.loadMapConfig(mapName)
	if err != nil {
		return err
	}

	for _, node := range mapConfig.Nodes {
		if node.Name == newNode.Name {
			return fmt.Errorf("node with name '%s' already exists", newNode.Name)
		}
	}

	if newNode.Position.X > mapConfig.Width || newNode.Position.Y > mapConfig.Height {
		return fmt.Errorf("node position is out of map bounds")
	}

	mapConfig.Nodes = append(mapConfig.Nodes, *newNode)
	return s.saveMap(mapName, mapConfig)
}

func (s *MapService) DeleteNode(mapName, nodeName string) error {
	mapConfig, err := s.loadMapConfig(mapName)
	if err != nil {
		return err
	}

	nodeFound := false
	newNodes := make([]config.Node, 0, len(mapConfig.Nodes))
	for _, node := range mapConfig.Nodes {
		if node.Name != nodeName {
			newNodes = append(newNodes, node)
		} else {
			nodeFound = true
		}
	}

	if !nodeFound {
		return fmt.Errorf("node not found")
	}

	newLinks := make([]config.Link, 0, len(mapConfig.Links))
	for _, link := range mapConfig.Links {
		if link.From != nodeName && link.To != nodeName {
			newLinks = append(newLinks, link)
		}
	}

	mapConfig.Nodes = newNodes
	mapConfig.Links = newLinks

	return s.saveMap(mapName, mapConfig)
}

func (s *MapService) EditMap(mapName string, updates map[string]any) error {
	mapConfig, err := s.loadMapConfig(mapName)
	if err != nil {
		return err
	}

	if title, ok := updates["title"].(string); ok {
		mapConfig.Title = title
	}

	if width, ok := updates["width"].(float64); ok {
		if width <= 0 {
			return fmt.Errorf("width must be greater than 0")
		}
		mapConfig.Width = int(width)
	}
	if height, ok := updates["height"].(float64); ok {
		if height <= 0 {
			return fmt.Errorf("height must be greater than 0")
		}
		mapConfig.Height = int(height)
	}

	return s.saveMap(mapName, mapConfig)
}

func (s *MapService) EditNode(mapName, nodeName string, updates map[string]any) error {
	mapConfig, err := s.loadMapConfig(mapName)
	if err != nil {
		return err
	}

	nodeFound := false
	for i, node := range mapConfig.Nodes {
		if node.Name == nodeName {
			if label, ok := updates["label"].(string); ok {
				mapConfig.Nodes[i].Label = label
			}
			if icon, ok := updates["icon"].(string); ok {
				mapConfig.Nodes[i].Icon = icon
			}
			if pos, ok := updates["position"].(map[string]any); ok {
				if x, ok := pos["x"].(float64); ok {
					mapConfig.Nodes[i].Position.X = int(x)
				}
				if y, ok := pos["y"].(float64); ok {
					mapConfig.Nodes[i].Position.Y = int(y)
				}
			}
			nodeFound = true
			break
		}
	}

	if !nodeFound {
		return fmt.Errorf("node not found")
	}

	return s.saveMap(mapName, mapConfig)
}

func (s *MapService) EditLink(mapName, linkName string, updates map[string]any) error {
	mapConfig, err := s.loadMapConfig(mapName)
	if err != nil {
		return err
	}

	for i, link := range mapConfig.Links {
		if link.Name == linkName {

			if bandwidth, ok := updates["bandwidth"].(string); ok {
				mapConfig.Links[i].Bandwidth = bandwidth
			}

			if viaData, ok := updates["via"].([]any); ok {

				if len(viaData) == 0 {
					mapConfig.Links[i].Via = nil
				} else {
					viaPositions := make([]config.Position, 0, len(viaData))
					for _, item := range viaData {
						if viaMap, ok := item.(map[string]any); ok {
							if x, ok := viaMap["x"].(float64); ok {
								if y, ok := viaMap["y"].(float64); ok {
									viaPositions = append(viaPositions, config.Position{X: int(x), Y: int(y)})
								}
							}
						}
					}
					mapConfig.Links[i].Via = viaPositions
				}
			}

			return s.saveMap(mapName, mapConfig)
		}
	}

	return fmt.Errorf("link not found")
}

func (s *MapService) AddLink(mapName string, newLink *config.Link) error {
	mapConfig, err := s.loadMapConfig(mapName)
	if err != nil {
		return err
	}

	for _, link := range mapConfig.Links {
		if link.Name == newLink.Name {
			return fmt.Errorf("link with name '%s' already exists", newLink.Name)
		}
	}
	// link bandwidth vilidating at saveMap by parser before save
	mapConfig.Links = append(mapConfig.Links, *newLink)
	return s.saveMap(mapName, mapConfig)
}

func (s *MapService) DeleteLink(mapName, linkName string) error {
	mapConfig, err := s.loadMapConfig(mapName)
	if err != nil {
		return err
	}

	linkFound := false
	newLinks := make([]config.Link, 0, len(mapConfig.Links))
	for _, link := range mapConfig.Links {
		if link.Name != linkName {
			newLinks = append(newLinks, link)
		} else {
			linkFound = true
		}
	}

	if !linkFound {
		return fmt.Errorf("link not found")
	}

	mapConfig.Links = newLinks
	return s.saveMap(mapName, mapConfig)
}

func (s *MapService) AddNodesBulk(mapName string, newNodes []config.Node) error {
	mapConfig, err := s.loadMapConfig(mapName)
	if err != nil {
		return err
	}

	existingNodes := make(map[string]bool)
	for _, node := range mapConfig.Nodes {
		existingNodes[node.Name] = true
	}

	for _, newNode := range newNodes {
		if existingNodes[newNode.Name] {
			return fmt.Errorf("node with name '%s' already exists", newNode.Name)
		}
		if newNode.Position.X > mapConfig.Width || newNode.Position.Y > mapConfig.Height {
			return fmt.Errorf("node '%s' position is out of map bounds", newNode.Name)
		}
		existingNodes[newNode.Name] = true
	}

	mapConfig.Nodes = append(mapConfig.Nodes, newNodes...)
	return s.saveMap(mapName, mapConfig)
}

func (s *MapService) DeleteNodesBulk(mapName string, nodeNames []string) error {
	mapConfig, err := s.loadMapConfig(mapName)
	if err != nil {
		return err
	}

	nodesToDelete := make(map[string]bool)
	for _, name := range nodeNames {
		nodesToDelete[name] = true
	}

	nodeFound := false
	newNodes := make([]config.Node, 0, len(mapConfig.Nodes))
	for _, node := range mapConfig.Nodes {
		if !nodesToDelete[node.Name] {
			newNodes = append(newNodes, node)
		} else {
			nodeFound = true
		}
	}

	if !nodeFound {
		return fmt.Errorf("nodes not found")
	}

	mapConfig.Nodes = newNodes
	return s.saveMap(mapName, mapConfig)
}

func (s *MapService) AddLinksBulk(mapName string, newLinks []config.Link) error {
	mapConfig, err := s.loadMapConfig(mapName)
	if err != nil {
		return err
	}

	existingLinks := make(map[string]bool)
	for _, link := range mapConfig.Links {
		existingLinks[link.Name] = true
	}

	for _, newLink := range newLinks {
		if existingLinks[newLink.Name] {
			return fmt.Errorf("link with name '%s' already exists", newLink.Name)
		}
		existingLinks[newLink.Name] = true
	}

	mapConfig.Links = append(mapConfig.Links, newLinks...)
	return s.saveMap(mapName, mapConfig)
}

func (s *MapService) DeleteLinksBulk(mapName string, linkNames []string) error {
	mapConfig, err := s.loadMapConfig(mapName)
	if err != nil {
		return err
	}

	linksToDelete := make(map[string]bool)
	for _, name := range linkNames {
		linksToDelete[name] = true
	}

	linkFound := false
	newLinks := make([]config.Link, 0, len(mapConfig.Links))
	for _, link := range mapConfig.Links {
		if !linksToDelete[link.Name] {
			newLinks = append(newLinks, link)
		} else {
			linkFound = true
		}
	}

	if !linkFound {
		return fmt.Errorf("links not found")
	}

	mapConfig.Links = newLinks
	return s.saveMap(mapName, mapConfig)
}

func (s *MapService) loadMapConfig(mapName string) (*config.Map, error) {
	configPath := filepath.Join(s.configDir, mapName+".yaml")
	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("map not found: %s", mapName)
	}
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Printf("Failed to close file %s: %v\n", configPath, err)
		}
	}()

	return s.parser.ParseYAML(file)
}

func (s *MapService) saveMap(mapName string, mapConfig *config.Map) error {
	if err := s.parser.Validate(mapConfig); err != nil {
		return fmt.Errorf("validation failed before saving: %w", err)
	}
	configPath := filepath.Join(s.configDir, mapName+".yaml")
	data, err := yaml.Marshal(mapConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return os.WriteFile(configPath, data, 0644)
}

func (s *MapService) GetMapVariables(mapName string) (map[string]string, error) {
	mapConfig, err := s.loadMapConfig(mapName)
	if err != nil {
		return nil, err
	}

	if mapConfig.Variables == nil {
		return make(map[string]string), nil
	}

	return mapConfig.Variables, nil
}

func (s *MapService) UpdateMapVariables(mapName string, variables map[string]string) error {
	mapConfig, err := s.loadMapConfig(mapName)
	if err != nil {
		return err
	}

	mapConfig.Variables = variables
	return s.saveMap(mapName, mapConfig)
}

func (s *MapService) ListIcons() ([]config.IconInfo, error) {
	if err := os.MkdirAll(s.iconsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create icons directory: %w", err)
	}

	files, err := filepath.Glob(filepath.Join(s.iconsDir, "*.svg"))
	if err != nil {
		return nil, fmt.Errorf("failed to read icons directory: %w", err)
	}

	icons := make([]config.IconInfo, 0, len(files))
	for _, file := range files {
		baseName := filepath.Base(file)
		ext := filepath.Ext(baseName)
		name := baseName[:len(baseName)-len(ext)]

		category := "other"
		switch {
		case slices.Contains([]string{
			"router", "switch", "firewall", "loadbalancer", "load_balancer", "lb",
			"gw", "gateway", "core", "access", "distribution", "edge", "border",
			"ix", "directconnect", "pni",
		}, name):
			category = "network"
		case slices.Contains([]string{
			"server", "database", "storage", "nas", "san", "dns", "dhcp", "ntp",
		}, name):
			category = "servers"
		case slices.Contains([]string{
			"cloud", "aws", "azure", "gcp", "digitalocean",
		}, name):
			category = "cloud"
		case slices.Contains([]string{
			"pc", "laptop", "phone", "tablet", "mobile", "desktop", "workstation", "monitor",
		}, name):
			category = "endpoints"
		}

		icons = append(icons, config.IconInfo{
			Name:        baseName,
			DisplayName: formatDisplayName(name),
			Category:    category,
		})
	}

	return icons, nil
}

func (s *MapService) GetIconFile(iconName string) ([]byte, string, error) {
	iconPath := filepath.Join(s.iconsDir, iconName)
	if data, err := os.ReadFile(iconPath); err == nil {
		return data, "image/svg+xml", nil
	}

	return nil, "", fmt.Errorf("icon not found: %s", iconName)
}

func formatDisplayName(name string) string {
	words := strings.Split(name, "_")
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
		}
	}
	return strings.Join(words, " ")
}
