package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"go-weathermap/internal/config"
	"go-weathermap/internal/utils"
)

func (s *Server) HandleMaps(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		s.ListMaps(w, r)
	case "POST":
		s.CreateMap(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) HandleMapOperations(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/maps/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Map name is required")
		return
	}
	mapName := parts[0]

	switch r.Method {
	case "GET":
		if len(parts) == 2 && parts[1] == "nodes" {
			s.ListMapNodes(w, r, mapName)
			return
		}
		if len(parts) == 2 && parts[1] == "links" {
			s.ListMapLinks(w, r, mapName)
			return
		}
		if len(parts) == 2 && parts[1] == "variables" {
			s.GetMapVariables(w, r, mapName)
			return
		}
		s.GetMap(w, r)
	case "PATCH":
		if len(parts) == 3 && parts[1] == "nodes" {
			s.EditNode(w, r)
			return
		}
		if len(parts) == 3 && parts[1] == "links" {
			s.EditLink(w, r)
			return
		}
		if len(parts) == 2 && parts[1] == "variables" {
			s.UpdateMapVariables(w, r, mapName)
			return
		}
		if len(parts) == 1 {
			s.EditMap(w, r)
			return
		}
		http.NotFound(w, r)
	case "POST":
		if len(parts) == 2 && parts[1] == "nodes" {
			s.AddNode(w, r)
			return
		}
		if len(parts) == 2 && parts[1] == "links" {
			s.AddLink(w, r)
			return
		}
		if len(parts) == 3 && parts[1] == "nodes" && parts[2] == "bulk" {
			s.AddNodesBulk(w, r)
			return
		}
		if len(parts) == 3 && parts[1] == "links" && parts[2] == "bulk" {
			s.AddLinksBulk(w, r)
			return
		}
		http.NotFound(w, r)
	case "DELETE":
		if len(parts) == 3 && parts[1] == "nodes" && parts[2] == "bulk" {
			s.DeleteNodesBulk(w, r)
			return
		}
		if len(parts) == 3 && parts[1] == "links" && parts[2] == "bulk" {
			s.DeleteLinksBulk(w, r)
			return
		}
		if len(parts) == 3 && parts[1] == "nodes" {
			s.DeleteNode(w, r)
			return
		}
		if len(parts) == 3 && parts[1] == "links" {
			s.DeleteLink(w, r)
			return
		}
		if len(parts) == 1 {
			s.DeleteMap(w, r)
			return
		}
		http.NotFound(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) ListMapNodes(w http.ResponseWriter, r *http.Request, mapName string) {
	if mapName == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Map name is required")
		return
	}
	mapWithData, err := s.mapService.GetMapWithData(mapName, s.dataSourceService)
	if err != nil {
		if strings.Contains(err.Error(), "map not found") {
			utils.RespondWithError(w, http.StatusNotFound, err.Error())
		} else {
			utils.RespondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	searchQuery := r.URL.Query().Get("search")
	if searchQuery == "" {
		utils.RespondWithJSON(w, http.StatusOK, mapWithData.Nodes)
		return
	}

	var filteredNodes []config.Node
	for _, node := range mapWithData.Nodes {
		if strings.Contains(strings.ToLower(node.Name), strings.ToLower(searchQuery)) {
			filteredNodes = append(filteredNodes, node)
		}
	}
	utils.RespondWithJSON(w, http.StatusOK, filteredNodes)
}

func (s *Server) ListMapLinks(w http.ResponseWriter, r *http.Request, mapName string) {
	if mapName == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Map name is required")
		return
	}
	mapWithData, err := s.mapService.GetMapWithData(mapName, s.dataSourceService)
	if err != nil {
		if strings.Contains(err.Error(), "map not found") {
			utils.RespondWithError(w, http.StatusNotFound, err.Error())
		} else {
			utils.RespondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	statusQuery := r.URL.Query().Get("status")
	nodeQuery := r.URL.Query().Get("node")
	if statusQuery == "" && nodeQuery == "" {
		utils.RespondWithJSON(w, http.StatusOK, mapWithData.LinksData)
		return
	}

	var filteredLinks []config.LinkData
	nodeQueryLower := strings.ToLower(nodeQuery)
	for i, link := range mapWithData.LinksData {
		match := true
		if statusQuery != "" && !strings.EqualFold(link.Status, statusQuery) {
			match = false
		}
		if nodeQuery != "" {
			linkObj := mapWithData.Links[i]
			if strings.ToLower(linkObj.From) != nodeQueryLower && strings.ToLower(linkObj.To) != nodeQueryLower {
				match = false
			}
		}
		if match {
			filteredLinks = append(filteredLinks, link)
		}
	}
	utils.RespondWithJSON(w, http.StatusOK, filteredLinks)
}

func (s *Server) ListMaps(w http.ResponseWriter, r *http.Request) {
	maps, err := s.mapService.ListMaps()
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.RespondWithJSON(w, http.StatusOK, map[string][]string{"maps": maps})
}

func (s *Server) CreateMap(w http.ResponseWriter, r *http.Request) {
	var newMap config.Map
	if err := json.NewDecoder(r.Body).Decode(&newMap); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if newMap.Width <= 0 || newMap.Height <= 0 {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid map size")
		return
	}

	mapName := strings.ToLower(strings.ReplaceAll(newMap.Title, " ", "-"))

	if err := s.mapService.CreateMap(&newMap, mapName); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.RespondWithJSON(w, http.StatusCreated, map[string]string{
		"status": "map created",
		"name":   mapName,
	})
}

func (s *Server) GetMap(w http.ResponseWriter, r *http.Request) {
	mapName := strings.TrimPrefix(r.URL.Path, "/maps/")
	mapWithData, err := s.mapService.GetMapWithData(mapName, s.dataSourceService)
	if err != nil {
		if strings.Contains(err.Error(), "map not found") {
			utils.RespondWithError(w, http.StatusNotFound, err.Error())
		} else if strings.Contains(err.Error(), "validation failed") {
			utils.RespondWithError(w, http.StatusInternalServerError, err.Error())
		} else {
			utils.RespondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	include := r.URL.Query().Get("include")
	if include == "" {
		utils.RespondWithJSON(w, http.StatusOK, mapWithData)
		return
	}

	fields := strings.Split(include, ",")
	filteredData := make(map[string]any)

	for _, field := range fields {
		switch field {
		case "width":
			filteredData["width"] = mapWithData.Width
		case "height":
			filteredData["height"] = mapWithData.Height
		case "title":
			filteredData["title"] = mapWithData.Title
		case "nodes":
			filteredData["nodes"] = mapWithData.Nodes
		case "links":
			filteredData["links"] = mapWithData.Links
		case "bgcolor":
			filteredData["bgcolor"] = mapWithData.BGColor
		}
	}

	utils.RespondWithJSON(w, http.StatusOK, filteredData)
}

func (s *Server) AddNode(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	mapName := parts[2]
	var node config.Node
	if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	if err := s.mapService.AddNode(mapName, &node); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			utils.RespondWithError(w, http.StatusConflict, err.Error())
		} else if strings.Contains(err.Error(), "out of map bounds") {
			utils.RespondWithError(w, http.StatusBadRequest, err.Error())
		} else if strings.Contains(err.Error(), "validation failed") {
			utils.RespondWithError(w, http.StatusBadRequest, err.Error())
		} else {
			utils.RespondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"status": "node added"})
}

func (s *Server) AddLink(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	mapName := parts[2]
	var link config.Link
	if err := json.NewDecoder(r.Body).Decode(&link); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	if err := s.mapService.AddLink(mapName, &link); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			utils.RespondWithError(w, http.StatusConflict, err.Error())
		} else if strings.Contains(err.Error(), "validation failed") {
			utils.RespondWithError(w, http.StatusBadRequest, err.Error())
		} else {
			utils.RespondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"status": "link added"})
}

func (s *Server) EditMap(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/maps/"), "/")
	mapName := parts[0]

	var mapUpdates map[string]any
	if err := json.NewDecoder(r.Body).Decode(&mapUpdates); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid JSON for map edit")
		return
	}

	if err := s.mapService.EditMap(mapName, mapUpdates); err != nil {
		if strings.Contains(err.Error(), "map not found") {
			utils.RespondWithError(w, http.StatusNotFound, err.Error())
		} else if strings.Contains(err.Error(), "must be greater than 0") {
			utils.RespondWithError(w, http.StatusBadRequest, err.Error())
		} else {
			utils.RespondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"status": "map updated"})
}

func (s *Server) EditNode(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/maps/"), "/")
	mapName := parts[0]
	nodeName := parts[2]

	var nodeUpdates map[string]any
	if err := json.NewDecoder(r.Body).Decode(&nodeUpdates); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid JSON for node edit")
		return
	}

	if err := s.mapService.EditNode(mapName, nodeName, nodeUpdates); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"status": "node updated"})
}

func (s *Server) EditLink(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/maps/"), "/")
	mapName := parts[0]
	linkName := parts[2]

	var linkUpdates map[string]any
	if err := json.NewDecoder(r.Body).Decode(&linkUpdates); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid JSON for link edit")
		return
	}

	if err := s.mapService.EditLink(mapName, linkName, linkUpdates); err != nil {
		if strings.Contains(err.Error(), "not found") {
			utils.RespondWithError(w, http.StatusNotFound, err.Error())
		} else {
			utils.RespondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]string{
		"status": "link updated",
		"name":   linkName,
	})
}

func (s *Server) DeleteNode(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	mapName := parts[2]
	nodeName := parts[4]
	if err := s.mapService.DeleteNode(mapName, nodeName); err != nil {
		if strings.Contains(err.Error(), "not found") {
			utils.RespondWithError(w, http.StatusNotFound, err.Error())
		} else {
			utils.RespondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"status": "node deleted"})
}

func (s *Server) DeleteLink(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	mapName := parts[2]
	linkName := parts[4]
	if err := s.mapService.DeleteLink(mapName, linkName); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"status": "link deleted"})
}

func (s *Server) DeleteMap(w http.ResponseWriter, r *http.Request) {
	mapName := strings.TrimPrefix(r.URL.Path, "/maps/")
	if err := s.mapService.DeleteMap(mapName); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"status": "map deleted"})
}

func (s *Server) AddNodesBulk(w http.ResponseWriter, r *http.Request) {
	mapName := strings.Split(r.URL.Path, "/")[2]
	var nodes []config.Node
	if err := json.NewDecoder(r.Body).Decode(&nodes); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	if err := s.mapService.AddNodesBulk(mapName, nodes); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			utils.RespondWithError(w, http.StatusConflict, err.Error())
		} else if strings.Contains(err.Error(), "out of map bounds") {
			utils.RespondWithError(w, http.StatusBadRequest, err.Error())
		} else if strings.Contains(err.Error(), "validation failed") {
			utils.RespondWithError(w, http.StatusBadRequest, err.Error())
		} else {
			utils.RespondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"status": "nodes added in bulk"})
}

type DeleteNodesBulkPayload struct {
	Nodes []string `json:"nodes"`
}

func (s *Server) DeleteNodesBulk(w http.ResponseWriter, r *http.Request) {
	mapName := strings.Split(r.URL.Path, "/")[2]
	var payload DeleteNodesBulkPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	if err := s.mapService.DeleteNodesBulk(mapName, payload.Nodes); err != nil {
		if strings.Contains(err.Error(), "not found") {
			utils.RespondWithError(w, http.StatusNotFound, err.Error())
		} else {
			utils.RespondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"status": "nodes deleted in bulk"})
}

func (s *Server) GetMapVariables(w http.ResponseWriter, r *http.Request, mapName string) {
	if mapName == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Map name is required")
		return
	}

	variables, err := s.mapService.GetMapVariables(mapName)
	if err != nil {
		if strings.Contains(err.Error(), "map not found") {
			utils.RespondWithError(w, http.StatusNotFound, err.Error())
		} else {
			utils.RespondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, variables)
}

func (s *Server) UpdateMapVariables(w http.ResponseWriter, r *http.Request, mapName string) {
	if mapName == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Map name is required")
		return
	}

	var variables map[string]string
	if err := json.NewDecoder(r.Body).Decode(&variables); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if err := s.mapService.UpdateMapVariables(mapName, variables); err != nil {
		if strings.Contains(err.Error(), "map not found") {
			utils.RespondWithError(w, http.StatusNotFound, err.Error())
		} else {
			utils.RespondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"status": "variables updated"})
}

func (s *Server) AddLinksBulk(w http.ResponseWriter, r *http.Request) {
	mapName := strings.Split(r.URL.Path, "/")[2]
	var links []config.Link
	if err := json.NewDecoder(r.Body).Decode(&links); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	if err := s.mapService.AddLinksBulk(mapName, links); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			utils.RespondWithError(w, http.StatusConflict, err.Error())
		} else if strings.Contains(err.Error(), "validation failed") {
			utils.RespondWithError(w, http.StatusBadRequest, err.Error())
		} else {
			utils.RespondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	utils.RespondWithJSON(w, http.StatusOK, map[string]any{"status": "links added in bulk", "links_count": len(links)})
}

func (s *Server) DeleteLinksBulk(w http.ResponseWriter, r *http.Request) {
	mapName := strings.Split(r.URL.Path, "/")[2]
	var linkNames []string
	if err := json.NewDecoder(r.Body).Decode(&linkNames); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	if err := s.mapService.DeleteLinksBulk(mapName, linkNames); err != nil {
		if strings.Contains(err.Error(), "not found") {
			utils.RespondWithError(w, http.StatusNotFound, err.Error())
		} else {
			utils.RespondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	utils.RespondWithJSON(w, http.StatusOK, map[string]any{"status": "links deleted in bulk", "deleted_count": len(linkNames)})
}

func (s *Server) HandleIcons(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		s.ListIcons(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) HandleIconFile(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		s.GetIconFile(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) ListIcons(w http.ResponseWriter, r *http.Request) {
	icons, err := s.mapService.ListIcons()
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.RespondWithJSON(w, http.StatusOK, icons)
}

func (s *Server) GetIconFile(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/icons/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Icon name is required")
		return
	}

	iconName := parts[0]
	iconData, contentType, err := s.mapService.GetIconFile(iconName)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			utils.RespondWithError(w, http.StatusNotFound, err.Error())
		} else {
			utils.RespondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=2592000") // http browser cache
	_, _ = w.Write(iconData)
}
