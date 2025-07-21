package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"go-weathermap/internal/config"
	"go-weathermap/internal/service"
)

func TestHealth(t *testing.T) {
	request, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	mapService := service.NewMapService("maps")
	server := NewServer(mapService, nil)
	handler := http.HandlerFunc(server.Health)

	handler.ServeHTTP(rr, request)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	expected := `{"status":"ok"}`
	if strings.TrimSpace(rr.Body.String()) != expected {
		t.Errorf("Handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestAPI(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "maps-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp dir %s: %v", tempDir, err)
		}
	}()

	dsService := service.NewDataSourceService(nil)
	mapService := service.NewMapService(tempDir)
	server := NewServer(mapService, dsService)
	mapName := "full-mesh-test"

	t.Run("CreateMap", func(t *testing.T) {
		mapConfig := fmt.Sprintf(`{"title": "%s", "width": 500, "height": 500}`, mapName)
		request := httptest.NewRequest("POST", "/maps", bytes.NewBufferString(mapConfig))
		rr := httptest.NewRecorder()

		server.ServeHTTP(rr, request)

		if rr.Code != http.StatusCreated {
			t.Fatalf("CreateMap failed: expected status 201, got %d. Body: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("TestValidationErrors", func(t *testing.T) {
		testCases := []struct {
			name           string
			method         string
			path           string
			body           string
			expectedStatus int
		}{
			{"InvalidMapSize", "POST", "/maps", `{"title":"invalid-size-map","width":-1,"height":-1}`, http.StatusBadRequest},
			{"NodeOutOfBounds", "POST", "/maps/" + mapName + "/nodes", `{"name":"out-of-bounds-node","position":{"x":600,"y":600}}`, http.StatusBadRequest},
			{"InvalidBandwidth", "POST", "/maps/" + mapName + "/links", `{"name":"invalid-bw-link","from":"node1","to":"node2","bandwidth":"100   M"}`, http.StatusBadRequest},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
				rec := httptest.NewRecorder()
				server.ServeHTTP(rec, req)

				if rec.Code != tc.expectedStatus {
					t.Errorf("Expected status %d, got %d", tc.expectedStatus, rec.Code)
				}
			})
		}
	})

	nodes := []string{"node1", "node2", "node3", "node4"}
	t.Run("AddNodes", func(t *testing.T) {
		for i, nodeName := range nodes {
			nodeConfig := fmt.Sprintf(`{"name": "%s", "position": {"x": %d, "y": %d}}`, nodeName, 100*(i+1), 100)
			request := httptest.NewRequest("POST", "/maps/"+mapName+"/nodes", bytes.NewBufferString(nodeConfig))
			rr := httptest.NewRecorder()
			server.ServeHTTP(rr, request)

			if rr.Code != http.StatusOK {
				t.Fatalf("AddNode %s failed: status %d, body: %s", nodeName, rr.Code, rr.Body.String())
			}
		}
	})

	t.Run("AddLinksFullMesh", func(t *testing.T) {
		for i := range nodes {
			for j := i + 1; j < len(nodes); j++ {
				linkName := fmt.Sprintf("link-%s-%s", nodes[i], nodes[j])
				linkConfig := fmt.Sprintf(`{"name": "%s", "from": "%s", "to": "%s", "bandwidth": "100M"}`, linkName, nodes[i], nodes[j])
				request := httptest.NewRequest("POST", "/maps/"+mapName+"/links", bytes.NewBufferString(linkConfig))
				rr := httptest.NewRecorder()
				server.ServeHTTP(rr, request)
				if rr.Code != http.StatusOK {
					t.Fatalf("AddLink %s failed: status %d, body: %s", linkName, rr.Code, rr.Body.String())
				}
			}
		}
	})

	var createdMap config.MapWithData
	t.Run("VerifyFullMapCreation", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/maps/"+mapName, nil)
		rr := httptest.NewRecorder()
		server.ServeHTTP(rr, request)

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

	t.Run("ListMapNodes", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/maps/"+mapName+"/nodes", nil)
		rr := httptest.NewRecorder()
		server.ServeHTTP(rr, request)
		if rr.Code != http.StatusOK {
			t.Fatalf("ListMapNodes failed: status %d", rr.Code)
		}
		var gotNodes []config.Node
		if err := json.NewDecoder(rr.Body).Decode(&gotNodes); err != nil {
			t.Fatalf("Failed to decode nodes: %v", err)
		}
		if len(gotNodes) != len(nodes) {
			t.Errorf("Expected %d nodes, got %d", len(nodes), len(gotNodes))
		}

		searchRequest := httptest.NewRequest("GET", "/maps/"+mapName+"/nodes?search="+nodes[0], nil)
		searchRR := httptest.NewRecorder()
		server.ServeHTTP(searchRR, searchRequest)
		if searchRR.Code != http.StatusOK {
			t.Fatalf("ListMapNodes with search failed: status %d", searchRR.Code)
		}
		var filteredNodes []config.Node
		if err := json.NewDecoder(searchRR.Body).Decode(&filteredNodes); err != nil {
			t.Fatalf("Failed to decode filtered nodes: %v", err)
		}
		if len(filteredNodes) != 1 || filteredNodes[0].Name != nodes[0] {
			t.Errorf("Expected 1 node named '%s', got %+v", nodes[0], filteredNodes)
		}
	})

	t.Run("ListMapLinks", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/maps/"+mapName+"/links", nil)
		rr := httptest.NewRecorder()
		server.ServeHTTP(rr, request)
		if rr.Code != http.StatusOK {
			t.Fatalf("ListMapLinks failed: status %d", rr.Code)
		}
		var gotLinks []config.LinkData
		if err := json.NewDecoder(rr.Body).Decode(&gotLinks); err != nil {
			t.Fatalf("Failed to decode links: %v", err)
		}
		if len(gotLinks) != (len(nodes)*(len(nodes)-1))/2 {
			t.Errorf("Expected %d links, got %d", (len(nodes)*(len(nodes)-1))/2, len(gotLinks))
		}

		searchRequest := httptest.NewRequest("GET", "/maps/"+mapName+"/links?status=up", nil)
		statusRR := httptest.NewRecorder()
		server.ServeHTTP(statusRR, searchRequest)
		if statusRR.Code != http.StatusOK {
			t.Fatalf("ListMapLinks with status failed: status %d", statusRR.Code)
		}
		var filteredLinks []config.LinkData
		if err := json.NewDecoder(statusRR.Body).Decode(&filteredLinks); err != nil {
			t.Fatalf("Failed to decode filtered links: %v", err)
		}

		nodeName := nodes[0]
		nodeRequest := httptest.NewRequest("GET", "/maps/"+mapName+"/links?node="+nodeName, nil)
		nodeRR := httptest.NewRecorder()
		server.ServeHTTP(nodeRR, nodeRequest)
		if nodeRR.Code != http.StatusOK {
			t.Fatalf("ListMapLinks with node filter failed: status %d", nodeRR.Code)
		}
		var nodeLinks []config.LinkData
		if err := json.NewDecoder(nodeRR.Body).Decode(&nodeLinks); err != nil {
			t.Fatalf("Failed to decode node-filtered links: %v", err)
		}
		if len(nodeLinks) != len(nodes)-1 {
			t.Errorf("Expected %d links for node %s, got %d", len(nodes)-1, nodeName, len(nodeLinks))
		}
	})

	t.Run("VerifyMapFiltering", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/maps/"+mapName+"?include=title,width,nodes", nil)
		rr := httptest.NewRecorder()
		server.ServeHTTP(rr, request)

		if rr.Code != http.StatusOK {
			t.Fatalf("GetMap with include failed: status %d, body: %s", rr.Code, rr.Body.String())
		}

		var filteredMap map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&filteredMap); err != nil {
			t.Fatalf("Failed to decode filtered map response: %v", err)
		}

		if title, ok := filteredMap["title"]; !ok || title != mapName {
			t.Errorf("Expected 'title' to be '%s', got '%v'", mapName, title)
		}

		if width, ok := filteredMap["width"]; !ok || width.(float64) != 500 { // float64 from JSON
			t.Errorf("Expected 'width' to be 500, got '%v'", width)
		}

		if _, ok := filteredMap["nodes"]; !ok {
			t.Error("Expected 'nodes' field in filtered response")
		}

		if _, ok := filteredMap["height"]; ok {
			t.Error("Unexpected 'height' field in filtered response")
		}
	})

	t.Run("EditMap", func(t *testing.T) {
		editMapPayload := map[string]any{
			"title":  "Updated Title",
			"width":  1024,
			"height": 1024,
		}
		editMapBody, _ := json.Marshal(editMapPayload)

		editRequest := httptest.NewRequest("PATCH", "/maps/"+mapName, bytes.NewBuffer(editMapBody))
		editRR := httptest.NewRecorder()
		server.ServeHTTP(editRR, editRequest)

		if editRR.Code != http.StatusOK {
			t.Fatalf("EditMap failed: status %d, body: %s", editRR.Code, editRR.Body.String())
		}

		// Verify map update
		getRequest := httptest.NewRequest("GET", "/maps/"+mapName, nil)
		getRR := httptest.NewRecorder()
		server.ServeHTTP(getRR, getRequest)
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

	t.Run("TestEditMapValidationErrors", func(t *testing.T) {
		testCases := []struct {
			name           string
			payload        map[string]any
			expectedStatus int
		}{
			{"InvalidWidth", map[string]any{"width": -1}, http.StatusBadRequest},
			{"InvalidHeight", map[string]any{"height": 0}, http.StatusBadRequest},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				editMapBody, _ := json.Marshal(tc.payload)
				editRequest := httptest.NewRequest("PATCH", "/maps/"+mapName, bytes.NewBuffer(editMapBody))
				editRR := httptest.NewRecorder()
				server.ServeHTTP(editRR, editRequest)

				if editRR.Code != tc.expectedStatus {
					t.Errorf("Expected status %d, got %d", tc.expectedStatus, editRR.Code)
				}
			})
		}
	})

	t.Run("EditMapPartialUpdate", func(t *testing.T) {
		editMapPayload := map[string]any{
			"title": "Partial Update Test",
		}
		editMapBody, _ := json.Marshal(editMapPayload)

		editRequest := httptest.NewRequest("PATCH", "/maps/"+mapName, bytes.NewBuffer(editMapBody))
		editRR := httptest.NewRecorder()
		server.ServeHTTP(editRR, editRequest)

		if editRR.Code != http.StatusOK {
			t.Fatalf("EditMap partial update failed: status %d, body: %s", editRR.Code, editRR.Body.String())
		}

		getRequest := httptest.NewRequest("GET", "/maps/"+mapName, nil)
		getRR := httptest.NewRecorder()
		server.ServeHTTP(getRR, getRequest)
		if getRR.Code != http.StatusOK {
			t.Fatalf("GetMap after partial update failed: status %d, body: %s", getRR.Code, getRR.Body.String())
		}
		var updatedMap config.MapWithData
		if err := json.NewDecoder(getRR.Body).Decode(&updatedMap); err != nil {
			t.Fatalf("Failed to decode updated map: %v", err)
		}

		if updatedMap.Title != "Partial Update Test" {
			t.Errorf("Expected title to be 'Partial Update Test', got '%s'", updatedMap.Title)
		}
		if updatedMap.Width != 1024 {
			t.Errorf("Expected width to remain 1024, got %d", updatedMap.Width)
		}
		if updatedMap.Height != 1024 {
			t.Errorf("Expected height to remain 1024, got %d", updatedMap.Height)
		}
	})

	t.Run("GetMapVariables", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/maps/"+mapName+"/variables", nil)
		rr := httptest.NewRecorder()
		server.ServeHTTP(rr, request)

		if rr.Code != http.StatusOK {
			t.Fatalf("GetMapVariables failed: status %d, body: %s", rr.Code, rr.Body.String())
		}

		var variables map[string]string
		if err := json.NewDecoder(rr.Body).Decode(&variables); err != nil {
			t.Fatalf("Failed to decode variables: %v", err)
		}
		if variables == nil {
			t.Error("Expected variables map, got nil")
		}
	})

	t.Run("UpdateMapVariables", func(t *testing.T) {
		variables := map[string]string{
			"zabbix_url":      "http://zabbix.example.com",
			"zabbix_user":     "admin",
			"zabbix_password": "secret",
		}

		data, err := json.Marshal(variables)
		if err != nil {
			t.Fatalf("Failed to marshal variables: %v", err)
		}

		request := httptest.NewRequest("PATCH", "/maps/"+mapName+"/variables", bytes.NewBuffer(data))
		request.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		server.ServeHTTP(rr, request)

		if rr.Code != http.StatusOK {
			t.Fatalf("UpdateMapVariables failed: status %d, body: %s", rr.Code, rr.Body.String())
		}

		var response map[string]string
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		if response["status"] != "variables updated" {
			t.Errorf("Expected status 'variables updated', got '%s'", response["status"])
		}

		getRequest := httptest.NewRequest("GET", "/maps/"+mapName+"/variables", nil)
		getRR := httptest.NewRecorder()
		server.ServeHTTP(getRR, getRequest)

		if getRR.Code != http.StatusOK {
			t.Fatalf("GetMapVariables after update failed: status %d", getRR.Code)
		}

		var updatedVariables map[string]string
		if err := json.NewDecoder(getRR.Body).Decode(&updatedVariables); err != nil {
			t.Fatalf("Failed to decode updated variables: %v", err)
		}

		for key, value := range variables {
			if updatedVariables[key] != value {
				t.Errorf("Variable %s: expected '%s', got '%s'", key, value, updatedVariables[key])
			}
		}
	})

	t.Run("TestVariablesNotFound", func(t *testing.T) {
		testCases := []struct {
			name           string
			method         string
			path           string
			body           string
			expectedStatus int
		}{
			{"GetVariablesNotFound", "GET", "/maps/non-existent/variables", "", http.StatusNotFound},
			{"UpdateVariablesNotFound", "PATCH", "/maps/non-existent/variables", `{"test":"value"}`, http.StatusNotFound},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				var req *http.Request
				if tc.body != "" {
					req = httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
					req.Header.Set("Content-Type", "application/json")
				} else {
					req = httptest.NewRequest(tc.method, tc.path, nil)
				}
				rec := httptest.NewRecorder()
				server.ServeHTTP(rec, req)

				if rec.Code != tc.expectedStatus {
					t.Errorf("Expected status %d, got %d", tc.expectedStatus, rec.Code)
				}
			})
		}
	})

	nodeToTest := "node4"
	t.Run("EditNodeThanDeleteNode", func(t *testing.T) {
		editNodePayload := map[string]any{
			"label":    "Updated Label for node4",
			"position": config.Position{X: 450, Y: 450},
		}
		editNodeBody, _ := json.Marshal(editNodePayload)

		editNodeRequest := httptest.NewRequest("PATCH", fmt.Sprintf("/maps/%s/nodes/%s", mapName, nodeToTest), bytes.NewBuffer(editNodeBody))
		editRR := httptest.NewRecorder()
		server.ServeHTTP(editRR, editNodeRequest)

		if editRR.Code != http.StatusOK {
			t.Fatalf("EditNode failed: status %d, body: %s", editRR.Code, editRR.Body.String())
		}

		deleteNodeRequest := httptest.NewRequest("DELETE", fmt.Sprintf("/maps/%s/nodes/%s", mapName, nodeToTest), nil)
		deleteRR := httptest.NewRecorder()
		server.ServeHTTP(deleteRR, deleteNodeRequest)
		if deleteRR.Code != http.StatusOK {
			t.Fatalf("DeleteNode failed: status %d, body: %s", deleteRR.Code, deleteRR.Body.String())
		}
	})

	t.Run("EditLink", func(t *testing.T) {
		editLinkPayload := map[string]any{
			"bandwidth": "10G",
		}
		editLinkBody, _ := json.Marshal(editLinkPayload)

		linkToEdit := "link-node1-node2"
		editLinkRequest := httptest.NewRequest("PATCH", fmt.Sprintf("/maps/%s/links/%s", mapName, linkToEdit), bytes.NewBuffer(editLinkBody))
		editRR := httptest.NewRecorder()
		server.ServeHTTP(editRR, editLinkRequest)

		if editRR.Code != http.StatusOK {
			t.Fatalf("EditLink failed: status %d, body: %s", editRR.Code, editRR.Body.String())
		}

		var editResponse map[string]string
		if err := json.NewDecoder(editRR.Body).Decode(&editResponse); err != nil {
			t.Fatalf("Failed to decode edit response: %v", err)
		}
		if editResponse["status"] != "link updated" {
			t.Errorf("Expected status 'link updated', got '%s'", editResponse["status"])
		}
		if editResponse["name"] != linkToEdit {
			t.Errorf("Expected name '%s', got '%s'", linkToEdit, editResponse["name"])
		}

		editLinkViaPayload := map[string]any{
			"via": []map[string]any{
				{"x": 250, "y": 150},
			},
		}
		editLinkViaBody, _ := json.Marshal(editLinkViaPayload)

		editLinkViaRequest := httptest.NewRequest("PATCH", fmt.Sprintf("/maps/%s/links/%s", mapName, linkToEdit), bytes.NewBuffer(editLinkViaBody))
		editViaRR := httptest.NewRecorder()
		server.ServeHTTP(editViaRR, editLinkViaRequest)

		if editViaRR.Code != http.StatusOK {
			t.Fatalf("EditLink via failed: status %d, body: %s", editViaRR.Code, editViaRR.Body.String())
		}

		// Verify link updates
		getRequest := httptest.NewRequest("GET", "/maps/"+mapName, nil)
		getRR := httptest.NewRecorder()
		server.ServeHTTP(getRR, getRequest)
		if getRR.Code != http.StatusOK {
			t.Fatalf("GetMap after link edit failed: status %d, body: %s", getRR.Code, getRR.Body.String())
		}
		var updatedMap config.MapWithData
		if err := json.NewDecoder(getRR.Body).Decode(&updatedMap); err != nil {
			t.Fatalf("Failed to decode updated map: %v", err)
		}

		linkFound := false
		for _, link := range updatedMap.Links {
			if link.Name == linkToEdit {
				if link.Bandwidth != "10G" {
					t.Errorf("Expected bandwidth to be '10G', got '%s'", link.Bandwidth)
				}
				if len(link.Via) != 1 || link.Via[0].X != 250 || link.Via[0].Y != 150 {
					t.Errorf("Expected via to be [{x:250, y:150}], got %+v", link.Via)
				}
				linkFound = true
				break
			}
		}
		if !linkFound {
			t.Errorf("Link %s not found in updated map", linkToEdit)
		}
	})

	t.Run("RemoveViaFromLink", func(t *testing.T) {
		addViaPayload := map[string]any{
			"via": []map[string]any{
				{"x": 250, "y": 150},
			},
		}
		addViaBody, _ := json.Marshal(addViaPayload)

		linkToEdit := "link-node1-node2"
		addViaRequest := httptest.NewRequest("PATCH", fmt.Sprintf("/maps/%s/links/%s", mapName, linkToEdit), bytes.NewBuffer(addViaBody))
		addViaRR := httptest.NewRecorder()
		server.ServeHTTP(addViaRR, addViaRequest)

		if addViaRR.Code != http.StatusOK {
			t.Fatalf("AddVia failed: status %d, body: %s", addViaRR.Code, addViaRR.Body.String())
		}

		removeViaPayload := map[string]any{
			"via": []map[string]any{},
		}
		removeViaBody, _ := json.Marshal(removeViaPayload)

		removeViaRequest := httptest.NewRequest("PATCH", fmt.Sprintf("/maps/%s/links/%s", mapName, linkToEdit), bytes.NewBuffer(removeViaBody))
		removeViaRR := httptest.NewRecorder()
		server.ServeHTTP(removeViaRR, removeViaRequest)

		if removeViaRR.Code != http.StatusOK {
			t.Fatalf("RemoveVia failed: status %d, body: %s", removeViaRR.Code, removeViaRR.Body.String())
		}

		getRequest := httptest.NewRequest("GET", "/maps/"+mapName, nil)
		getRR := httptest.NewRecorder()
		server.ServeHTTP(getRR, getRequest)
		if getRR.Code != http.StatusOK {
			t.Fatalf("GetMap after via removal failed: status %d, body: %s", getRR.Code, getRR.Body.String())
		}
		var updatedMap config.MapWithData
		if err := json.NewDecoder(getRR.Body).Decode(&updatedMap); err != nil {
			t.Fatalf("Failed to decode updated map: %v", err)
		}

		linkFound := false
		for _, link := range updatedMap.Links {
			if link.Name == linkToEdit {
				if len(link.Via) > 0 {
					t.Errorf("Expected via to be removed, but got %+v", link.Via)
				}
				linkFound = true
				break
			}
		}
		if !linkFound {
			t.Errorf("Link %s not found in updated map", linkToEdit)
		}
	})

	t.Run("VerifyNodeDeletion", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/maps/"+mapName, nil)
		rr := httptest.NewRecorder()
		server.ServeHTTP(rr, request)
		if rr.Code != http.StatusOK {
			t.Fatalf("GetMap failed: status %d, body: %s", rr.Code, rr.Body.String())
		}
		var currentMap config.MapWithData
		if err := json.NewDecoder(rr.Body).Decode(&currentMap); err != nil {
			t.Fatalf("Failed to decode map response: %v", err)
		}
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

	t.Run("TestNotFoundErrors", func(t *testing.T) {
		testCases := []struct {
			name           string
			method         string
			path           string
			body           string
			expectedStatus int
		}{
			{"DeleteNonExistentNode", "DELETE", fmt.Sprintf("/maps/%s/nodes/%s", mapName, "non-existent-node"), "", http.StatusNotFound},
			{"EditNonExistentLink", "PATCH", "/maps/" + mapName + "/links/non-existent-link", `{"bandwidth":"10G"}`, http.StatusNotFound},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				var req *http.Request
				if tc.body != "" {
					req = httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
				} else {
					req = httptest.NewRequest(tc.method, tc.path, nil)
				}
				rec := httptest.NewRecorder()
				server.ServeHTTP(rec, req)

				if rec.Code != tc.expectedStatus {
					t.Errorf("Expected status %d, got %d", tc.expectedStatus, rec.Code)
				}
			})
		}
	})

	t.Run("AddNodesBulk", func(t *testing.T) {
		nodesPayload := `[
	           {"name": "bulk-node1", "position": {"x": 50, "y": 50}},
	           {"name": "bulk-node2", "position": {"x": 150, "y": 150}}
	       ]`
		request := httptest.NewRequest("POST", "/maps/"+mapName+"/nodes/bulk", bytes.NewBufferString(nodesPayload))
		rr := httptest.NewRecorder()
		server.ServeHTTP(rr, request)

		if rr.Code != http.StatusOK {
			t.Fatalf("AddNodesBulk failed: status %d, body: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("VerifyBulkNodesAddition", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/maps/"+mapName, nil)
		rr := httptest.NewRecorder()
		server.ServeHTTP(rr, request)
		var currentMap config.MapWithData
		if err := json.NewDecoder(rr.Body).Decode(&currentMap); err != nil {
			t.Fatalf("Failed to decode map response: %v", err)
		}
		if len(currentMap.Nodes) != 5 {
			t.Errorf("Expected 5 nodes after bulk addition, got %d", len(currentMap.Nodes))
		}
	})

	t.Run("TestBulkNodesErrors", func(t *testing.T) {
		testCases := []struct {
			name           string
			body           string
			expectedStatus int
		}{
			{"AddNodesBulkAlreadyExists", `[{"name": "bulk-node1", "position": {"x": 50, "y": 50}}]`, http.StatusConflict},
			{"AddNodesBulkOutOfMap", `[{"name": "out-of-bounds-bulk", "position": {"x": 2000, "y": 2000}}]`, http.StatusBadRequest},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				request := httptest.NewRequest("POST", "/maps/"+mapName+"/nodes/bulk", bytes.NewBufferString(tc.body))
				rr := httptest.NewRecorder()
				server.ServeHTTP(rr, request)

				if rr.Code != tc.expectedStatus {
					t.Errorf("Expected status %d, got %d", tc.expectedStatus, rr.Code)
				}
			})
		}
	})

	t.Run("DeleteNodesBulk", func(t *testing.T) {
		deletePayload := `{"nodes": ["bulk-node1", "bulk-node2"]}`
		request := httptest.NewRequest("DELETE", "/maps/"+mapName+"/nodes/bulk", bytes.NewBufferString(deletePayload))
		rr := httptest.NewRecorder()
		server.ServeHTTP(rr, request)

		if rr.Code != http.StatusOK {
			t.Fatalf("DeleteNodesBulk failed: status %d, body: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("VerifyBulkNodesDeletion", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/maps/"+mapName, nil)
		rr := httptest.NewRecorder()
		server.ServeHTTP(rr, request)
		var currentMap config.MapWithData
		if err := json.NewDecoder(rr.Body).Decode(&currentMap); err != nil {
			t.Fatalf("Failed to decode map response: %v", err)
		}
		if len(currentMap.Nodes) != 3 {
			t.Errorf("Expected 3 nodes after bulk deletion, got %d", len(currentMap.Nodes))
		}
	})

	t.Run("DeleteAllLinks", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/maps/"+mapName, nil)
		rr := httptest.NewRecorder()
		server.ServeHTTP(rr, request)
		var currentMap config.MapWithData
		if err := json.NewDecoder(rr.Body).Decode(&currentMap); err != nil {
			t.Fatalf("Failed to decode map response: %v", err)
		}

		for _, link := range currentMap.Links {
			deleteReq := httptest.NewRequest("DELETE", fmt.Sprintf("/maps/%s/links/%s", mapName, link.Name), nil)
			deleteRR := httptest.NewRecorder()
			server.ServeHTTP(deleteRR, deleteReq)
			if deleteRR.Code != http.StatusOK {
				t.Fatalf("DeleteLink %s failed: status %d, body: %s", link.Name, deleteRR.Code, deleteRR.Body.String())
			}
		}
	})

	t.Run("AddLinksBulk", func(t *testing.T) {
		linksPayload := `[
		   {"name": "bulk-link1", "from": "node1", "to": "node2", "bandwidth": "100M"},
		   {"name": "bulk-link2", "from": "node2", "to": "node3", "bandwidth": "1G"}
		 ]`
		request := httptest.NewRequest("POST", "/maps/"+mapName+"/links/bulk", bytes.NewBufferString(linksPayload))
		rr := httptest.NewRecorder()
		server.ServeHTTP(rr, request)

		if rr.Code != http.StatusOK {
			t.Fatalf("AddLinksBulk failed: status %d, body: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("VerifyBulkLinksAddition", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/maps/"+mapName, nil)
		rr := httptest.NewRecorder()
		server.ServeHTTP(rr, request)
		var currentMap config.MapWithData
		if err := json.NewDecoder(rr.Body).Decode(&currentMap); err != nil {
			t.Fatalf("Failed to decode map response: %v", err)
		}
		found := 0
		for _, link := range currentMap.Links {
			if link.Name == "bulk-link1" || link.Name == "bulk-link2" {
				found++
			}
		}
		if found != 2 {
			t.Errorf("Expected 2 bulk links after addition, got %d", found)
		}
	})

	t.Run("TestBulkLinksErrors", func(t *testing.T) {
		testCases := []struct {
			name           string
			body           string
			expectedStatus int
		}{
			{"AddLinksBulkAlreadyExists", `[{"name": "bulk-link1", "from": "node1", "to": "node2", "bandwidth": "100M"}]`, http.StatusConflict},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				request := httptest.NewRequest("POST", "/maps/"+mapName+"/links/bulk", bytes.NewBufferString(tc.body))
				rr := httptest.NewRecorder()
				server.ServeHTTP(rr, request)

				if rr.Code != tc.expectedStatus {
					t.Errorf("Expected status %d, got %d", tc.expectedStatus, rr.Code)
				}
			})
		}
	})

	t.Run("DeleteLinksBulk", func(t *testing.T) {
		deletePayload := `["bulk-link1", "bulk-link2"]`
		request := httptest.NewRequest("DELETE", "/maps/"+mapName+"/links/bulk", bytes.NewBufferString(deletePayload))
		rr := httptest.NewRecorder()
		server.ServeHTTP(rr, request)

		if rr.Code != http.StatusOK {
			t.Fatalf("DeleteLinksBulk failed: status %d, body: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("VerifyBulkLinksDeletion", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/maps/"+mapName, nil)
		rr := httptest.NewRecorder()
		server.ServeHTTP(rr, request)
		var currentMap config.MapWithData
		if err := json.NewDecoder(rr.Body).Decode(&currentMap); err != nil {
			t.Fatalf("Failed to decode map response: %v", err)
		}
		for _, link := range currentMap.Links {
			if link.Name == "bulk-link1" || link.Name == "bulk-link2" {
				t.Errorf("Bulk link %s was not deleted", link.Name)
			}
		}
	})

	t.Run("TestBulkDeleteNotFound", func(t *testing.T) {
		testCases := []struct {
			name           string
			method         string
			path           string
			body           string
			expectedStatus int
		}{
			{"DeleteNodesBulkNotExists", "DELETE", "/maps/" + mapName + "/nodes/bulk", `{"nodes": ["non-existent-bulk"]}`, http.StatusNotFound},
			{"DeleteLinksBulkNotExists", "DELETE", "/maps/" + mapName + "/links/bulk", `["non-existent-bulk-link"]`, http.StatusNotFound},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				request := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
				rr := httptest.NewRecorder()
				server.ServeHTTP(rr, request)

				if rr.Code != tc.expectedStatus {
					t.Errorf("Expected status %d, got %d", tc.expectedStatus, rr.Code)
				}
			})
		}
	})

	t.Run("VerifyLinksDeletion", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/maps/"+mapName, nil)
		rr := httptest.NewRecorder()
		server.ServeHTTP(rr, request)
		var currentMap config.MapWithData
		if err := json.NewDecoder(rr.Body).Decode(&currentMap); err != nil {
			t.Fatalf("Failed to decode map response: %v", err)
		}
		if len(currentMap.Links) != 0 {
			t.Errorf("Expected 0 links after deletion, got %d", len(currentMap.Links))
		}
	})

	t.Run("DeleteAllNodes", func(t *testing.T) {
		remainingNodes := []string{"node1", "node2", "node3"}
		for _, nodeName := range remainingNodes {
			request := httptest.NewRequest("DELETE", fmt.Sprintf("/maps/%s/nodes/%s", mapName, nodeName), nil)
			rr := httptest.NewRecorder()
			server.ServeHTTP(rr, request)
			if rr.Code != http.StatusOK {
				t.Fatalf("DeleteNode %s failed: status %d, body: %s", nodeName, rr.Code, rr.Body.String())
			}
		}
	})

	t.Run("VerifyNodesDeletion", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/maps/"+mapName, nil)
		rr := httptest.NewRecorder()
		server.ServeHTTP(rr, request)
		var currentMap config.MapWithData
		if err := json.NewDecoder(rr.Body).Decode(&currentMap); err != nil {
			t.Fatalf("Failed to decode map response: %v", err)
		}
		if len(currentMap.Nodes) != 0 {
			t.Errorf("Expected 0 nodes after deletion, got %d", len(currentMap.Nodes))
		}
	})

	t.Run("DeleteMap", func(t *testing.T) {
		request := httptest.NewRequest("DELETE", "/maps/"+mapName, nil)
		rr := httptest.NewRecorder()
		server.ServeHTTP(rr, request)
		if rr.Code != http.StatusOK {
			t.Fatalf("DeleteMap failed: status %d, body: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("VerifyMapDeletion", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/maps", nil)
		rr := httptest.NewRecorder()
		server.ServeHTTP(rr, request)
		if rr.Code != http.StatusOK {
			t.Fatalf("ListMaps failed: status %d, body: %s", rr.Code, rr.Body.String())
		}
		if strings.Contains(rr.Body.String(), mapName) {
			t.Errorf("Map %s was not deleted, found in list", mapName)
		}
	})

	t.Run("TestIcons", func(t *testing.T) {
		iconsDir := tempDir + "/../internal/assets/icons"
		if err := os.MkdirAll(iconsDir, 0755); err != nil {
			t.Fatalf("Failed to create icons directory: %v", err)
		}

		testIconContent := `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24"><circle cx="12" cy="12" r="10" fill="blue"/></svg>`
		testIconPath := iconsDir + "/test-router.svg"
		if err := os.WriteFile(testIconPath, []byte(testIconContent), 0644); err != nil {
			t.Fatalf("Failed to create test icon: %v", err)
		}

		listRequest := httptest.NewRequest("GET", "/icons", nil)
		listRR := httptest.NewRecorder()
		server.ServeHTTP(listRR, listRequest)

		if listRR.Code != http.StatusOK {
			t.Fatalf("ListIcons failed: expected status 200, got %d. Body: %s", listRR.Code, listRR.Body.String())
		}

		var icons []config.IconInfo
		if err := json.NewDecoder(listRR.Body).Decode(&icons); err != nil {
			t.Fatalf("Failed to decode icons response: %v", err)
		}

		if icons == nil {
			t.Error("Expected icons array, got nil")
		}

		fileRequest := httptest.NewRequest("GET", "/icons/test-router.svg", nil)
		fileRR := httptest.NewRecorder()
		server.ServeHTTP(fileRR, fileRequest)

		if fileRR.Code != http.StatusOK {
			t.Fatalf("GetIconFile failed: expected status 200, got %d. Body: %s", fileRR.Code, fileRR.Body.String())
		}

		contentType := fileRR.Header().Get("Content-Type")
		if contentType != "image/svg+xml" {
			t.Errorf("Expected Content-Type image/svg+xml, got %s", contentType)
		}

		cacheControl := fileRR.Header().Get("Cache-Control")
		if cacheControl != "public, max-age=2592000" {
			t.Errorf("Expected Cache-Control public, max-age=2592000, got %s", cacheControl)
		}

		if fileRR.Body.String() != testIconContent {
			t.Errorf("Expected icon content to match, got different content")
		}
	})

	t.Run("TestIconsErrors", func(t *testing.T) {
		testCases := []struct {
			name           string
			path           string
			method         string
			expectedStatus int
		}{
			{"ListIconsWrongMethod", "/icons", "POST", http.StatusMethodNotAllowed},
			{"GetIconFileNotFound", "/icons/non-existent-icon.svg", "GET", http.StatusNotFound},
			{"GetIconFileWrongMethod", "/icons/test.svg", "POST", http.StatusMethodNotAllowed},
			{"GetIconFileEmptyName", "/icons/", "GET", http.StatusBadRequest},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest(tc.method, tc.path, nil)
				rec := httptest.NewRecorder()
				server.ServeHTTP(rec, req)

				if rec.Code != tc.expectedStatus {
					t.Errorf("Expected status %d, got %d", tc.expectedStatus, rec.Code)
				}
			})
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
