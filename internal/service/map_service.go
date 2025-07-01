package service

import (
	"context"
	"fmt"
	"go-weathermap/internal/config"
	"go-weathermap/internal/datasource"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type MapService struct {
	configDir string
	parser    *config.Parser
}

func NewMapService(configDir string) *MapService {
	return &MapService{
		configDir: configDir,
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

func (s *MapService) GetMapWithData(name string) (*config.MapWithData, error) {
	mapConfig, err := s.loadMapConfig(name)
	if err != nil {
		return nil, err
	}

	mapWithData := s.addMockData(mapConfig)
	return mapWithData, nil
}

func (s *MapService) addMockData(mapConfig *config.Map) *config.MapWithData {
	result := &config.MapWithData{
		Map:         mapConfig,
		ProcessedAt: time.Now(),
		LinksData:   make([]config.LinkData, 0, len(mapConfig.Links)),
		Nodes:       mapConfig.Nodes,
		Links:       mapConfig.Links,
	}

	for _, link := range mapConfig.Links {
		linkData := config.LinkData{
			Name:   link.Name,
			Status: "unknown",
		}

		if link.DataSource != nil {
			mock := datasource.NewMockDataSource(nil)
			if traffic, err := mock.GetTraffic(context.Background()); err == nil {
				linkData.Utilization = traffic.Utilization
				linkData.Status = "up"
			} else {
				linkData.Status = "down"
			}
		}

		result.LinksData = append(result.LinksData, linkData)
	}

	return result
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
		mapConfig.Width = int(width)
	}
	if height, ok := updates["height"].(float64); ok {
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
