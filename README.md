# go-weathermap

This is a little backend service for personal use that's kind of like [PHP Weathermap](https://network-weathermap.com/). It gives you an HTTP API to create, read, update, and delete "weather maps" for a network, and also manage their nodes and links between them.

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

    You can get information about a specific map by using the `/maps/{map_name}` endpoint. This will return the full configuration of the map, including traffic data. (At this moment data is mock).

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

#### Update map configuration

*   **PUT /maps/{map-name}**

    Updates the configuration of the existing map. 

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
      "status": "map updated",
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

### Nodes

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

### Links

#### Add link

*   **POST /maps/{map-name}/links**

    Add a new link between two nodes.

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

#### Remove link

*   **DELETE /maps/{map-name}/links/{link-name}**

    Remove link from map.

    **Example respone:**
    ```json
    {
      "status": "link deleted",
      "name": "{link-name}"
    }
