# go-weathermap

This is a little backend service for personal use that's kind of like [PHP Weathermap](http://www.network-weathermap.com/manual/). It gives you an HTTP API to create, read, update, and delete "weather maps" for a network, and also manage their nodes and links between them.

Map configurations are stored in `.yaml` files in a `maps` folder.

## Running the server

To run the server, type this command:

```bash
go run cmd/weathermap/main.go
```
It'll be listening on port 8080.

## API

You can use this service to manage maps via an RESTful API (request body is limit to 1MB)

### Health Check

*   **GET /health**

    Checks if the service is up.

    **Example response:**
    ```json
    {
      "status": "ok"
    }
    ```

### Maps

#### Listing all maps

*   **GET /maps**

    Returns a list of all available maps.

    **Example response:**
    ```json
    {
      "maps": [
        "example-map",
        "test-networkmap"
      ]
    }
    ```

#### Creating a new map

*   **POST /maps**

    To create a new map, you need to provide a title for the map. The title will be used to generate the name of the file.

    **Request body (JSON):**
    ```json
    {
      "title": "Example Map New",
      "width": 800,
      "height": 600,
      "nodes": [],
      "links": []
    }
    ```

    **Example response:**
    ```json
    {
      "status": "map created",
      "name": "example-map-new"
    }
    ```

#### Get map configuration

*   **GET /maps/{map-name}**

    This will return the full configuration of the map, including traffic data. (At this moment data is mock).

    **Query parameters:** 
    * `include` (string, optional): separated list of fields to include in the response (e.g., `width,height,title,nodes`). 

    **Example:**  
    `GET /maps/{map-name}?include=width,title`  
    
    **Example response:**
    ```json
    {
      "width": 800,
      "height": 600,
      "title": "example-map",
      "nodes": [
        {
          "name": "router1",
          "label": "Core Router 1",
          "position": { "x": 100, "y": 100 }
        }
      ],
      "links": [
        {
          "name": "link1",
          "from": "router1",
          "to": "router2",
          "bandwidth": "1G"
        }
      ],
      "processed_at": "2025-10-27T10:00:00Z",
      "links_data": [
        {
          "name": "link1",
          "utilization": 45.5,
          "status": "up"
        }
      ]
    }
    ```

#### Edit map configuration

*   **PATCH /maps/{map-name}**

    Edit the configuration of the existing map. 

    **Request body (JSON):**
    ```json
    {
      "title": "example-map",
      "width": 1024,
      "height": 1024
    }
    ```

    **Example response:**
    ```json
    {
      "status": "map updated",
      "name": "example-map"
    }
    ```

*   **PUT /maps/{map-name}**

    Replace the configuration of the existing map. 

    **Request body (JSON):**
    ```json
    {
      "title": "example-map",
      "width": 1024,
      "height": 768,
      "nodes": [
        {
          "name": "router1",
          "label": "Core Router 1",
          "position": { "x": 150, "y": 150 }
        }
      ],
      "links": []
    }
    ```

    **Example response:**
    ```json
    {
      "status": "map replaced",
      "name": "example-map"
    }
    ```

#### Delete map

*   **DELETE /maps/{map-name}**

    Delete the map

    **Example response:**
    ```json
    {
      "status": "map deleted",
      "name": "{map-name}"
    }
    ```

### Map Variables

#### Get map variables
*   **GET /maps/{map-name}/variables**

    Retrieve the global variables for a specific map.

    **Example response:**
    ```json
    {
      "zabbix_url": "http://zabbix.example.com",
      "zabbix_user": "admin",
      "zabbix_password": "secret"
    }
    ```

#### Update map variables
*   **PATCH /maps/{map-name}/variables**

    Update the global variables for a specific map. The request body should be a JSON object with key-value pairs.

    **Request body (JSON):**
    ```json
    {
      "zabbix_url": "http://zabbix.example.com",
      "zabbix_user": "admin",
      "zabbix_password": "secret"
    }
    ```

    **Example response:**
    ```json
    {
      "status": "variables updated"
    }
    ```

### Nodes

#### List all nodes for a specific map
*   **GET /maps/{map-name}/nodes**

    Retrieve all nodes associated with a specified map, optionally filtered by name.

    **Query parameters:** 
    * `search` (string, optional): Filters nodes whose names partially match the provided value.
        
    **Example:**  
    `GET /maps/{map-name}/nodes?search=core-router`

    **Example response (JSON array):**
    ```json
    [
      {
        "name": "core-router1",
        "label": "Core Router 1",
        "position": { "x": 100, "y": 100 },
        "icon": "router.png",
        "monitoring": true
      },
      {
        "name": "core-router2",
        "label": "Core Router 2",
        "position": { "x": 300, "y": 100 },
        "icon": "router.png",
        "monitoring": true
      }
    ]
    ```

#### Add node

*   **POST /maps/{map-name}/nodes**

    Creates a new node on the map.

    **Request body (JSON):**
    ```json
    {
      "name": "switch1",
      "label": "Access Switch 1",
      "position": { "x": 200, "y": 200 },
      "icon": "switch.png"
    }
    ```

    **Example response:**
    ```json
    {
      "status": "node added",
      "name": "switch1"
    }
    ```

#### Add multiple nodes

*   **POST /maps/{map-name}/nodes/bulk**

    Creates multiple new nodes on the map.

    **Request body (JSON):**
    ```json
    [
      {
        "name": "switch1",
        "label": "Access Switch 1",
        "position": { "x": 200, "y": 200 },
        "icon": "switch.png"
      },
      {
        "name": "switch2",
        "label": "Access Switch 2",
        "position": { "x": 300, "y": 200 },
        "icon": "switch.png"
      }
    ]
    ```

    **Example response:**
    ```json
    {
      "status": "nodes added",
      "nodes_count": 2
    }
    ```

#### Edit node
*  **PATCH /maps/{map-name}/nodes/{node-name}**
    
    Edit node position or label.

    **Request body (JSON):**
    ```json
    {
      "label": "Core Router 1",
      "position": { "x": 120, "y": 120 }
    }
    ```

    **Example response:**
    ```json
    {
      "status": "node updated",
      "name": "Core Router 1"
    }
    ```

#### Remove node

*   **DELETE /maps/{map-name}/nodes/{node-name}**

    This will delete the node and any links it has from the map. 

    **Example response:**
    ```json
    {
      "status": "node deleted",
      "name": "{node-name}"
    }
    ```

#### Remove multiple nodes

*   **DELETE /maps/{map-name}/nodes/bulk**

    This will delete the nodes and any links they have from the map.

    **Request body (JSON):**
    ```json
    {
      "nodes": ["switch1", "switch2"]
    }
    ```

    **Example response:**
    ```json
    {
      "status": "nodes deleted",
      "deleted_count": 2
    }
    ```

### Links

#### List all links for a specific map
*   **GET /maps/{map-name}/links**

    Get all links associated with a specified map, optionally filtered by operational status or by node.

    **Query parameters:**
    * `status` (string, optional): Filters links by their operational status ("up", "down", "unknown").
    * `node` (string, optional): Filters links that are connected to specified node.

    **Example:**  
    `GET /maps/{map-name}/links?status=down`  
    `GET /maps/{map-name}/links?node=core-router1`

    **Example (combined filter):**  
    `GET /maps/example-map/links?node=router1&status=up`

    **Example response (JSON array):**
    ```json
    [
      {"name":"core-link","utilization":97,"status":"up"},
      {"name":"router1-switch1","utilization":36.4,"status":"up"}
    ]
    ```

#### Add link

*   **POST /maps/{map-name}/links**

    Add a new link between two nodes. (bandwidth option: 100M, 10G, 1TB etc..)

    **Request body (JSON):**
    ```json
    {
      "name": "link-r1-s1",
      "from": "router1",
      "to": "switch1",
      "bandwidth": "10G"
    }
    ```

    **Example response:**
    ```json
    {
      "status": "link added",
      "name": "link-r1-s1"
    }
    ```

#### Add multiple links

*   **POST /maps/{map-name}/links/bulk**

    Creates multiple new links on the map.

    **Request body (JSON):**
    ```json
    [
      {
        "name": "link1",
        "from": "router1",
        "to": "switch1",
        "bandwidth": "10G"
      },
      {
        "name": "link2",
        "from": "switch1",
        "to": "router2",
        "bandwidth": "1G"
      }
    ]
    ```

    **Example response:**
    ```json
    {
      "status": "links added in bulk",
      "links_count": 2
    }
    ```

#### Edit link
*  **PATCH /maps/{map-name}/links/{link-name}**
    
    Edit link bandwidth or via points.

    **Request body (JSON):**
    ```json
    {
      "bandwidth": "10G",
      "via": [
        {"x": 250, "y": 150},
        {"x": 300, "y": 200}
      ]
    }
    ```

    **Or remove via points (empty array):**
    ```json
    {
      "via": []
    }
    ```

    **Example response:**
    ```json
    {
      "status": "link updated",
      "name": "{link-name}"
    }
    ```

#### Remove link

*   **DELETE /maps/{map-name}/links/{link-name}**

    Remove link from map.

    **Example response:**
    ```json
    {
      "status": "link deleted",
      "name": "{link-name}"
    }
    ```

#### Remove multiple links

*   **DELETE /maps/{map-name}/links/bulk**

    This will delete the links from the map.

    **Request body (JSON array):**
    ```json
    [
      "link1",
      "link2"
    ]
    ```

    **Example response:**
    ```json
    {
      "status": "links deleted in bulk",
      "deleted_count": 2
    }
    ```
