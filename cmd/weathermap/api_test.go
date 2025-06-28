package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go-weathermap/internal/config"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestHealth(t *testing.T) {
	request, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	server := NewServer("maps")
	handler := http.HandlerFunc(server.Health)

	handler.ServeHTTP(rr, request)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	expected := `{"status":"ok"}` + "\n"
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestAPI(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "maps-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	server := NewServer(tempDir)
	mapName := "full-mesh-test"

	t.Run("CreateMap", func(t *testing.T) {
		mapConfig := fmt.Sprintf(`{"title": "%s", "width": 500, "height": 500}`, mapName)
		request := httptest.NewRequest("POST", "/maps", bytes.NewBufferString(mapConfig))
		rr := httptest.NewRecorder()

		server.HandleMaps(rr, request)

		if rr.Code != http.StatusOK {
			t.Fatalf("CreateMap failed: expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("CreateMapWithInvalidSize", func(t *testing.T) {
		mapConfig := `{"title": "invalid-size-map", "width": -1, "height": -1}`
		request := httptest.NewRequest("POST", "/maps", bytes.NewBufferString(mapConfig))
		rr := httptest.NewRecorder()

		server.HandleMaps(rr, request)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("Expected status %d for invalid map size, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	nodes := []string{"node1", "node2", "node3", "node4"}
	t.Run("AddNodes", func(t *testing.T) {
		for i, nodeName := range nodes {
			nodeConfig := fmt.Sprintf(`{"name": "%s", "position": {"x": %d, "y": %d}}`, nodeName, 100*(i+1), 100)
			request := httptest.NewRequest("POST", "/maps/"+mapName+"/nodes", bytes.NewBufferString(nodeConfig))
			rr := httptest.NewRecorder()
			server.HandleMapOperations(rr, request)

			if rr.Code != http.StatusOK {
				t.Fatalf("AddNode %s failed: status %d, body: %s", nodeName, rr.Code, rr.Body.String())
			}
		}
	})

	t.Run("AddNodeOutsideMapBounds", func(t *testing.T) {
		nodeConfig := `{"name": "out-of-bounds-node", "position": {"x": 600, "y": 600}}`
		request := httptest.NewRequest("POST", "/maps/"+mapName+"/nodes", bytes.NewBufferString(nodeConfig))
		rr := httptest.NewRecorder()
		server.HandleMapOperations(rr, request)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("Expected status %d for out-of-bounds node, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("AddLinksFullMesh", func(t *testing.T) {
		for i := 0; i < len(nodes); i++ {
			for j := i + 1; j < len(nodes); j++ {
				linkName := fmt.Sprintf("link-%s-%s", nodes[i], nodes[j])
				linkConfig := fmt.Sprintf(`{"name": "%s", "from": "%s", "to": "%s"}`, linkName, nodes[i], nodes[j])
				request := httptest.NewRequest("POST", "/maps/"+mapName+"/links", bytes.NewBufferString(linkConfig))
				rr := httptest.NewRecorder()
				server.HandleMapOperations(rr, request)
				if rr.Code != http.StatusOK {
					t.Fatalf("AddLink %s failed: status %d, body: %s", linkName, rr.Code, rr.Body.String())
				}
			}
		}
	})

	t.Run("AddLinkWithInvalidBandwidth", func(t *testing.T) {
		linkConfig := `{"name": "invalid-bw-link", "from": "node1", "to": "node2", "bandwidth": "100 M"}`
		request := httptest.NewRequest("POST", "/maps/"+mapName+"/links", bytes.NewBufferString(linkConfig))
		rr := httptest.NewRecorder()
		server.HandleMapOperations(rr, request)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("Expected status %d for invalid bandwidth, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	var createdMap MapWithData
	t.Run("VerifyMapCreation", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/maps/"+mapName, nil)
		rr := httptest.NewRecorder()
		server.HandleMapOperations(rr, request)

		if rr.Code != http.StatusOK {
			t.Fatalf("GetMap failed: status %d, body: %s", rr.Code, rr.Body.String())
		}
		if err := json.NewDecoder(rr.Body).Decode(&createdMap); err != nil {
			t.Fatalf("Failed to decode map response: %v", err)
		}
		if len(createdMap.Nodes) != 4 {
			t.Errorf("Expected 4 nodes, got %d", len(createdMap.Nodes))
		}
		if len(createdMap.Links) != 6 {
			t.Errorf("Expected 6 links for full mesh, got %d", len(createdMap.Links))
		}
	})

	t.Run("EditMap", func(t *testing.T) {
		editMapPayload := map[string]interface{}{
			"title":  "Updated Title",
			"width":  1024,
			"height": 1024,
		}
		editMapBody, _ := json.Marshal(editMapPayload)

		editRequest := httptest.NewRequest("PATCH", "/maps/"+mapName, bytes.NewBuffer(editMapBody))
		editRR := httptest.NewRecorder()
		server.HandleMapOperations(editRR, editRequest)

		if editRR.Code != http.StatusOK {
			t.Fatalf("EditMap failed: status %d, body: %s", editRR.Code, editRR.Body.String())
		}

		// Verify map update
		getRequest := httptest.NewRequest("GET", "/maps/"+mapName, nil)
		getRR := httptest.NewRecorder()
		server.HandleMapOperations(getRR, getRequest)
		if getRR.Code != http.StatusOK {
			t.Fatalf("GetMap after edit failed: status %d, body: %s", getRR.Code, getRR.Body.String())
		}
		var updatedMap config.Map
		if err := json.NewDecoder(getRR.Body).Decode(&updatedMap); err != nil {
			t.Fatalf("Failed to decode updated map: %v", err)
		}

		if updatedMap.Title != "Updated Title" {
			t.Errorf("Expected title to be 'Updated Title', got '%s'", updatedMap.Title)
		}
		if updatedMap.Width != 1024 {
			t.Errorf("Expected width is 1024, got %d", updatedMap.Width)
		}
		if updatedMap.Height != 1024 {
			t.Errorf("Expected height is 1024, got %d", updatedMap.Height)
		}
	})

	nodeToTest := "node4"
	t.Run("EditNodeThanDeleteNode", func(t *testing.T) {
		// Edit Node
		editNodePayload := map[string]interface{}{
			"label":    "Updated Label for node4",
			"position": config.Position{X: 450, Y: 450},
		}
		editNodeBody, _ := json.Marshal(editNodePayload)

		editNodeRequest := httptest.NewRequest("PATCH", fmt.Sprintf("/maps/%s/nodes/%s", mapName, nodeToTest), bytes.NewBuffer(editNodeBody))
		editRR := httptest.NewRecorder()
		server.HandleMapOperations(editRR, editNodeRequest)

		if editRR.Code != http.StatusOK {
			t.Fatalf("EditNode failed: status %d, body: %s", editRR.Code, editRR.Body.String())
		}

		// Delete Node
		deleteNodeRequest := httptest.NewRequest("DELETE", fmt.Sprintf("/maps/%s/nodes/%s", mapName, nodeToTest), nil)
		deleteRR := httptest.NewRecorder()
		server.HandleMapOperations(deleteRR, deleteNodeRequest)
		if deleteRR.Code != http.StatusOK {
			t.Fatalf("DeleteNode failed: status %d, body: %s", deleteRR.Code, deleteRR.Body.String())
		}
	})

	t.Run("VerifyNodeDeletion", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/maps/"+mapName, nil)
		rr := httptest.NewRecorder()
		server.HandleMapOperations(rr, request)
		if rr.Code != http.StatusOK {
			t.Fatalf("GetMap failed: status %d, body: %s", rr.Code, rr.Body.String())
		}
		var currentMap MapWithData
		json.NewDecoder(rr.Body).Decode(&currentMap)
		if len(currentMap.Nodes) != 3 {
			t.Errorf("Expected 3 nodes after deletion, got %d", len(currentMap.Nodes))
		}
		for _, node := range currentMap.Nodes {
			if node.Name == nodeToTest {
				t.Errorf("Deleted node %s is still present", nodeToTest)
			}
		}
		if len(currentMap.Links) != 3 {
			t.Errorf("Expected 3 links after node deletion, got %d", len(currentMap.Links))
		}
	})

	t.Run("DeleteNonExistentNode", func(t *testing.T) {
		request := httptest.NewRequest("DELETE", fmt.Sprintf("/maps/%s/nodes/%s", mapName, "non-existent-node"), nil)
		rr := httptest.NewRecorder()
		server.HandleMapOperations(rr, request)
		if rr.Code != http.StatusNotFound {
			t.Errorf("Expected status %d for deleting non-existent node, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("DeleteAllLinks", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/maps/"+mapName, nil)
		rr := httptest.NewRecorder()
		server.HandleMapOperations(rr, request)
		var currentMap config.Map
		json.NewDecoder(rr.Body).Decode(&currentMap)

		for _, link := range currentMap.Links {
			deleteReq := httptest.NewRequest("DELETE", fmt.Sprintf("/maps/%s/links/%s", mapName, link.Name), nil)
			deleteRR := httptest.NewRecorder()
			server.HandleMapOperations(deleteRR, deleteReq)
			if deleteRR.Code != http.StatusOK {
				t.Fatalf("DeleteLink %s failed: status %d, body: %s", link.Name, deleteRR.Code, deleteRR.Body.String())
			}
		}
	})

	t.Run("VerifyLinksDeletion", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/maps/"+mapName, nil)
		rr := httptest.NewRecorder()
		server.HandleMapOperations(rr, request)
		var currentMap MapWithData
		json.NewDecoder(rr.Body).Decode(&currentMap)
		if len(currentMap.Links) != 0 {
			t.Errorf("Expected 0 links after deletion, got %d", len(currentMap.Links))
		}
	})

	t.Run("DeleteAllNodes", func(t *testing.T) {
		remainingNodes := []string{"node1", "node2", "node3"}
		for _, nodeName := range remainingNodes {
			request := httptest.NewRequest("DELETE", fmt.Sprintf("/maps/%s/nodes/%s", mapName, nodeName), nil)
			rr := httptest.NewRecorder()
			server.HandleMapOperations(rr, request)
			if rr.Code != http.StatusOK {
				t.Fatalf("DeleteNode %s failed: status %d, body: %s", nodeName, rr.Code, rr.Body.String())
			}
		}
	})

	t.Run("VerifyNodesDeletion", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/maps/"+mapName, nil)
		rr := httptest.NewRecorder()
		server.HandleMapOperations(rr, request)
		var currentMap MapWithData
		json.NewDecoder(rr.Body).Decode(&currentMap)
		if len(currentMap.Nodes) != 0 {
			t.Errorf("Expected 0 nodes after deletion, got %d", len(currentMap.Nodes))
		}
	})

	t.Run("DeleteMap", func(t *testing.T) {
		request := httptest.NewRequest("DELETE", "/maps/"+mapName, nil)
		rr := httptest.NewRecorder()
		server.HandleMapOperations(rr, request)
		if rr.Code != http.StatusOK {
			t.Fatalf("DeleteMap failed: status %d, body: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("VerifyMapDeletion", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/maps", nil)
		rr := httptest.NewRecorder()
		server.HandleMaps(rr, request)
		if rr.Code != http.StatusOK {
			t.Fatalf("ListMaps failed: status %d, body: %s", rr.Code, rr.Body.String())
		}
		if strings.Contains(rr.Body.String(), mapName) {
			t.Errorf("Map %s was not deleted, found in list", mapName)
		}
	})

	/*
		TODO: large body requests
		t.Run("CreateTooLargeBodyRequest", func(t *testing.T) {
			largeTitleBody := fmt.Sprintf(`{"title": "%s"}`, strings.Repeat("a", 1048577))
			request := httptest.NewRequest("POST", "/maps", bytes.NewBufferString(largeTitleBody))
			rr := httptest.NewRecorder()
		})
	*/
}
