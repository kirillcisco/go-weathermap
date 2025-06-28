package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"go-weathermap/internal/config"
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
	switch r.Method {
	case "GET":
		// /maps/{mapName}
		s.GetMap(w, r)
	case "PATCH":
		// /maps/{mapName} for map edits
		// /maps/{mapName}/nodes/{nodeName} for node edits
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/maps/"), "/")
		if len(parts) == 3 && parts[1] == "nodes" {
			s.EditNode(w, r)
		} else if len(parts) == 1 {
			s.EditMap(w, r)
		} else {
			http.NotFound(w, r)
		}
	case "POST":
		// /maps/{mapName}/nodes, /maps/{mapName}/links, etc.
		if strings.HasSuffix(r.URL.Path, "/nodes") {
			s.AddNode(w, r)
		} else if strings.HasSuffix(r.URL.Path, "/links") {
			s.AddLink(w, r)
		} else if strings.HasSuffix(r.URL.Path, "/nodes/bulk") {
			s.AddNodesBulk(w, r)
		} else {
			http.NotFound(w, r)
		}
	case "DELETE":
		// /maps/{mapName}, /maps/{mapName}/nodes/{nodeName}, etc.
		if strings.Contains(r.URL.Path, "/nodes/") {
			if strings.HasSuffix(r.URL.Path, "/bulk") {
				s.DeleteNodesBulk(w, r)
			} else {
				s.DeleteNode(w, r)
			}
		} else if strings.Contains(r.URL.Path, "/links/") {
			s.DeleteLink(w, r)
		} else {
			s.DeleteMap(w, r)
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) ListMaps(w http.ResponseWriter, r *http.Request) {
	maps, err := s.mapService.ListMaps()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, map[string][]string{"maps": maps})
}

func (s *Server) CreateMap(w http.ResponseWriter, r *http.Request) {
	var newMap config.Map
	if err := json.NewDecoder(r.Body).Decode(&newMap); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if newMap.Width <= 0 || newMap.Height <= 0 {
		respondWithError(w, http.StatusBadRequest, "Invalid map size")
		return
	}

	mapName := strings.ToLower(strings.ReplaceAll(newMap.Title, " ", "-"))

	if err := s.mapService.CreateMap(&newMap, mapName); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusCreated, map[string]string{
		"status": "map created",
		"name":   mapName,
	})
}

func (s *Server) GetMap(w http.ResponseWriter, r *http.Request) {
	mapName := strings.TrimPrefix(r.URL.Path, "/maps/")
	mapWithData, err := s.mapService.GetMapWithData(mapName)
	if err != nil {
		if strings.Contains(err.Error(), "map not found") {
			respondWithError(w, http.StatusNotFound, err.Error())
		} else if strings.Contains(err.Error(), "validation failed") {
			respondWithError(w, http.StatusInternalServerError, err.Error())
		} else {
			respondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	respondWithJSON(w, http.StatusOK, mapWithData)
}

func (s *Server) AddNode(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	mapName := parts[2]
	var node config.Node
	if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	if err := s.mapService.AddNode(mapName, &node); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			respondWithError(w, http.StatusConflict, err.Error())
		} else if strings.Contains(err.Error(), "out of map bounds") {
			respondWithError(w, http.StatusBadRequest, err.Error())
		} else if strings.Contains(err.Error(), "validation failed") {
			respondWithError(w, http.StatusBadRequest, err.Error())
		} else {
			respondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "node added"})
}

func (s *Server) AddLink(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	mapName := parts[2]
	var link config.Link
	if err := json.NewDecoder(r.Body).Decode(&link); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	if err := s.mapService.AddLink(mapName, &link); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			respondWithError(w, http.StatusConflict, err.Error())
		} else if strings.Contains(err.Error(), "validation failed") {
			respondWithError(w, http.StatusBadRequest, err.Error())
		} else {
			respondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "link added"})
}

func (s *Server) EditMap(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/maps/"), "/")
	mapName := parts[0]

	var mapUpdates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&mapUpdates); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON for map edit")
		return
	}
	if err := s.mapService.EditMap(mapName, mapUpdates); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "map updated"})
}

func (s *Server) EditNode(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/maps/"), "/")
	mapName := parts[0]
	nodeName := parts[2]

	var nodeUpdates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&nodeUpdates); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON for node edit")
		return
	}

	if err := s.mapService.EditNode(mapName, nodeName, nodeUpdates); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"status": "node updated"})
}

func (s *Server) DeleteNode(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	mapName := parts[2]
	nodeName := parts[4]
	if err := s.mapService.DeleteNode(mapName, nodeName); err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondWithError(w, http.StatusNotFound, err.Error())
		} else {
			respondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "node deleted"})
}

func (s *Server) DeleteLink(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	mapName := parts[2]
	linkName := parts[4]
	if err := s.mapService.DeleteLink(mapName, linkName); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "link deleted"})
}

func (s *Server) DeleteMap(w http.ResponseWriter, r *http.Request) {
	mapName := strings.TrimPrefix(r.URL.Path, "/maps/")
	if err := s.mapService.DeleteMap(mapName); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "map deleted"})
}

func (s *Server) AddNodesBulk(w http.ResponseWriter, r *http.Request) {
	mapName := strings.Split(r.URL.Path, "/")[2]
	var nodes []config.Node
	if err := json.NewDecoder(r.Body).Decode(&nodes); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	if err := s.mapService.AddNodesBulk(mapName, nodes); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			respondWithError(w, http.StatusConflict, err.Error())
		} else if strings.Contains(err.Error(), "out of map bounds") {
			respondWithError(w, http.StatusBadRequest, err.Error())
		} else if strings.Contains(err.Error(), "validation failed") {
			respondWithError(w, http.StatusBadRequest, err.Error())
		} else {
			respondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "nodes added in bulk"})
}

type DeleteNodesBulkPayload struct {
	Nodes []string `json:"nodes"`
}

func (s *Server) DeleteNodesBulk(w http.ResponseWriter, r *http.Request) {
	mapName := strings.Split(r.URL.Path, "/")[2]
	var payload DeleteNodesBulkPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	if err := s.mapService.DeleteNodesBulk(mapName, payload.Nodes); err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondWithError(w, http.StatusNotFound, err.Error())
		} else {
			respondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "nodes deleted in bulk"})
}
