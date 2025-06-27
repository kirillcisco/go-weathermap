package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-weathermap/internal/config"
	"go-weathermap/internal/datasource"

	"gopkg.in/yaml.v3"
)

func main() {
	configDir := "configs"
	if len(os.Args) > 1 {
		configDir = os.Args[1]
	}

	server := NewServer(configDir)

	http.HandleFunc("/maps", server.HandleMaps)
	http.HandleFunc("/maps/", server.HandleMapOperations)
	http.HandleFunc("/health", server.Health)

	fmt.Println("Starting weathermap server on :8080")
	fmt.Println("API endpoints:")
	fmt.Println("  GET    /maps              					- list maps")
	fmt.Println("  POST   /maps              					- create map")
	fmt.Println("  GET    /maps/map-name     					- get map with data")
	fmt.Println("  PUT    /maps/map-name						- update map")
	fmt.Println("  DELETE /maps/map-name      					- delete map")
	fmt.Println("  POST   /maps/map-name/nodes 					- add node")
	fmt.Println("  DELETE /maps/map-name/nodes/node-name		- delete node")
	fmt.Println("  POST   /maps/map-name/links					- add link")
	fmt.Println("  DELETE /maps/map-name/links/link-name		- delete link")

	log.Fatal(http.ListenAndServe(":8080", nil))
}

type Server struct {
	configDir string
	parser    *config.Parser
}

type MapWithData struct {
	*config.Map
	ProcessedAt time.Time  `json:"processed_at"`
	LinksData   []LinkData `json:"links_data"`
}

type LinkData struct {
	Name        string  `json:"name"`
	Utilization float64 `json:"utilization"`
	Status      string  `json:"status"`
}

func NewServer(configDir string) *Server {
	return &Server{
		configDir: configDir,
		parser:    config.NewParser(),
	}
}

// CRUD API OPS

func (s *Server) HandleMaps(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		s.listMaps(w, r)
	case "POST":
		s.createMap(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) ListMaps(w http.ResponseWriter, r *http.Request) {
	files, err := filepath.Glob(filepath.Join(s.configDir, "*.yaml"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var maps []string
	for _, file := range files {
		name := filepath.Base(file)
		name = name[:len(name)-5] // убираем .yaml
		maps = append(maps, name)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]string{"maps": maps})
}

func (s *Server) HandleMapOperations(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/maps/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "map name required", http.StatusBadRequest)
		return
	}

	mapName := parts[0]

	switch len(parts) {
	case 1:
		// /maps/map-name
		switch r.Method {
		case "GET":
			s.getMap(w, r, mapName)
		case "PUT":
			s.updateMap(w, r, mapName)
		case "DELETE":
			s.deleteMap(w, r, mapName)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	case 2:
		// /maps/map-name/nodes or /maps/map-name/links
		switch parts[1] {
		case "nodes":
			if r.Method == "POST" {
				s.addNode(w, r, mapName)
			} else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		case "links":
			if r.Method == "POST" {
				s.addLink(w, r, mapName)
			} else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		default:
			http.Error(w, "unknown endpoint", http.StatusNotFound)
		}
	case 3:
		// /maps/map-name/nodes/node-name or /maps/map-name/links/link-name
		itemName := parts[2]
		switch parts[1] {
		case "nodes":
			if r.Method == "DELETE" {
				s.deleteNode(w, r, mapName, itemName)
			} else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		case "links":
			if r.Method == "DELETE" {
				s.deleteLink(w, r, mapName, itemName)
			} else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		default:
			http.Error(w, "unknown endpoint", http.StatusNotFound)
		}
	default:
		http.Error(w, "invalid path", http.StatusBadRequest)
	}
}

func (s *Server) listMaps(w http.ResponseWriter, r *http.Request) {
	files, err := filepath.Glob(filepath.Join(s.configDir, "*.yaml"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var maps []string
	for _, file := range files {
		name := filepath.Base(file)
		name = name[:len(name)-5]
		maps = append(maps, name)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]string{"maps": maps})
}

// POST /maps
func (s *Server) createMap(w http.ResponseWriter, r *http.Request) {
	var newMap config.Map
	if err := json.NewDecoder(r.Body).Decode(&newMap); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if newMap.Title == "" {
		http.Error(w, "Title is required", http.StatusBadRequest)
		return
	}

	mapName := strings.ToLower(strings.ReplaceAll(newMap.Title, " ", "-"))

	if err := s.saveMap(mapName, &newMap); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "created",
		"name":   mapName,
	})
}

// GET /maps/map-name
func (s *Server) getMap(w http.ResponseWriter, r *http.Request, mapName string) {
	mapConfig, err := s.loadMapConfig(mapName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	mapWithData := s.addMockData(mapConfig)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mapWithData)
}

// PUT /maps/map-name
func (s *Server) updateMap(w http.ResponseWriter, r *http.Request, mapName string) {
	var updatedMap config.Map
	if err := json.NewDecoder(r.Body).Decode(&updatedMap); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if err := s.saveMap(mapName, &updatedMap); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

// DELETE /maps/map-name
func (s *Server) deleteMap(w http.ResponseWriter, r *http.Request, mapName string) {
	configPath := filepath.Join(s.configDir, mapName+".yaml")

	if err := os.Remove(configPath); err != nil {
		http.Error(w, "Map not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// POST /maps/map-name/nodes
func (s *Server) addNode(w http.ResponseWriter, r *http.Request, mapName string) {
	var newNode config.Node
	if err := json.NewDecoder(r.Body).Decode(&newNode); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if newNode.Name == "" {
		http.Error(w, "Node name is required", http.StatusBadRequest)
		return
	}

	mapConfig, err := s.loadMapConfig(mapName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	for _, node := range mapConfig.Nodes {
		if node.Name == newNode.Name {
			http.Error(w, "Node already exists", http.StatusConflict)
			return
		}
	}

	mapConfig.Nodes = append(mapConfig.Nodes, newNode)

	if err := s.saveMap(mapName, mapConfig); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "node added"})
}

// DELETE /maps/map-name/nodes/node-name
func (s *Server) deleteNode(w http.ResponseWriter, r *http.Request, mapName, nodeName string) {
	mapConfig, err := s.loadMapConfig(mapName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
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
		http.Error(w, "Node not found", http.StatusNotFound)
		return
	}

	// del all links related to this node
	newLinks := make([]config.Link, 0, len(mapConfig.Links))
	for _, link := range mapConfig.Links {
		if link.From != nodeName && link.To != nodeName {
			newLinks = append(newLinks, link)
		}
	}

	mapConfig.Nodes = newNodes
	mapConfig.Links = newLinks

	if err := s.saveMap(mapName, mapConfig); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "node deleted"})
}

// POST /maps/map-name/links
func (s *Server) addLink(w http.ResponseWriter, r *http.Request, mapName string) {
	var newLink config.Link
	if err := json.NewDecoder(r.Body).Decode(&newLink); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if newLink.Name == "" || newLink.From == "" || newLink.To == "" {
		http.Error(w, "Link name, from and to are required", http.StatusBadRequest)
		return
	}

	mapConfig, err := s.loadMapConfig(mapName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	for _, link := range mapConfig.Links {
		if link.Name == newLink.Name {
			http.Error(w, "Link already exists", http.StatusConflict)
			return
		}
	}

	nodeExists := func(name string) bool {
		for _, node := range mapConfig.Nodes {
			if node.Name == name {
				return true
			}
		}
		return false
	}

	if !nodeExists(newLink.From) {
		http.Error(w, "From node does not exist", http.StatusBadRequest)
		return
	}

	if !nodeExists(newLink.To) {
		http.Error(w, "To node does not exist", http.StatusBadRequest)
		return
	}

	mapConfig.Links = append(mapConfig.Links, newLink)

	if err := s.saveMap(mapName, mapConfig); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "link added"})
}

// DELETE /maps/map-name/links/link-name
func (s *Server) deleteLink(w http.ResponseWriter, r *http.Request, mapName, linkName string) {
	mapConfig, err := s.loadMapConfig(mapName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
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
		http.Error(w, "Link not found", http.StatusNotFound)
		return
	}

	mapConfig.Links = newLinks

	if err := s.saveMap(mapName, mapConfig); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "link deleted"})
}

func (s *Server) HandleMapRequest(w http.ResponseWriter, r *http.Request) {
	mapName := r.URL.Path[len("/maps/"):]
	if mapName == "" {
		http.Error(w, "map name required", http.StatusBadRequest)
		return
	}

	configPath := filepath.Join(s.configDir, mapName+".yaml")
	file, err := os.Open(configPath)
	if err != nil {
		http.Error(w, "map not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	mapConfig, err := s.parser.ParseYAML(file)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to parse map: %v", err), http.StatusInternalServerError)
		return
	}

	mapWithData := s.addMockData(mapConfig)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mapWithData)
}

func (s *Server) loadMapConfig(mapName string) (*config.Map, error) {
	configPath := filepath.Join(s.configDir, mapName+".yaml")
	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("map not found: %s", mapName)
	}
	defer file.Close()

	return s.parser.ParseYAML(file)
}

func (s *Server) saveMap(mapName string, mapConfig *config.Map) error {
	configPath := filepath.Join(s.configDir, mapName+".yaml")

	data, err := yaml.Marshal(mapConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(configPath, data, 0644)
}

func (s *Server) addMockData(mapConfig *config.Map) *MapWithData {
	result := &MapWithData{
		Map:         mapConfig,
		ProcessedAt: time.Now(),
		LinksData:   make([]LinkData, 0, len(mapConfig.Links)),
	}

	for _, link := range mapConfig.Links {
		linkData := LinkData{
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

func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
