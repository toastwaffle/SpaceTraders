# SpaceTraders

Samuel's implementation of a [SpaceTraders](https://spacetraders.io/) agent.

## Supported features

 * Agent registration
 * Ship purchasing
 * Viewing assorted data
 * Contract management
 * Automated procurement contract activity for command ships and mining drones
   * Including using surveying to optimise mining
   * Automated extract -> travel -> deliver -> travel -> extract cycle

## Running

Run the following commands from the package directory

```shell
openapi-generator \
  generate \
  -i https://stoplight.io/api/v1/projects/spacetraders/spacetraders/nodes/reference/SpaceTraders.json\?fromExportButton\=true\&snapshotType\=http_service\&deref\=optimizedBundle \
  -o api \
  -g go \
  --additional-properties=enumClassPrefix=true \
  --additional-properties=isGoSubmodule=true \
  --additional-properties=packageName="api"
rm api/go.mod api/go.sum
go run cmd/spacetraders/spacetraders.go
```

State data and your agent's auth token are stored in `$XDG_CONFIG_DIR/spacetraders` (typically ~/.config/spacetraders)
