package main

import (
	"fmt"
	"os"

	"go-weathermap/internal/api"
	"go-weathermap/internal/service"
)

func main() {
	configDir := "maps"
	if len(os.Args) > 1 {
		configDir = os.Args[1]
	}

	datasources, err := service.LoadAllDataSources(configDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error while load datasource: %v\n", err)
		os.Exit(1)
	}
	dsService := service.NewDataSourceService(datasources)
	dsService.Start()

	mapService := service.NewMapService(configDir)

	server := api.NewServer(mapService, dsService)

	fmt.Println("Starting weathermap server on :8080")
	fmt.Println("API endpoints:")
	fmt.Println("  GET    /health           				- Check service health")
	fmt.Println("  GET    /maps              				- list maps")
	fmt.Println("  POST   /maps              				- create map")
	fmt.Println("  GET    /maps/{mapName}     				- get map with data")
	fmt.Println("  DELETE /maps/{mapName}      				- delete map")
	fmt.Println("  PATCH  /maps/{mapName}      				- edit map properties")
	fmt.Println("  POST   /maps/{mapName}/nodes 			- add node")
	fmt.Println("  POST   /maps/{mapName}/nodes/bulk 		- add multiple nodes")
	fmt.Println("  DELETE /maps/{mapName}/nodes/{nodeName} 	- delete node")
	fmt.Println("  DELETE /maps/{mapName}/nodes/bulk 		- delete multiple nodes")
	fmt.Println("  PATCH  /maps/{mapName}/nodes/{nodeName} 	- edit node")
	fmt.Println("  POST   /maps/{mapName}/links 			- add link")
	fmt.Println("  DELETE /maps/{mapName}/links/{linkName} 	- delete link")

	server.Start(":8080")
}
