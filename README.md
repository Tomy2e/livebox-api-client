# Livebox API client

This Go library makes it easy to communicate with Livebox's API. This API is
usually available at `http://192.168.1.1/ws`. Authentication is handled by
the library, set the `admin` password and start sending requests.

It was tested with a Livebox 5. Other Livebox models might not be supported.

## Usage

Get the library by running the following command:

```console
go get -u github.com/Tomy2e/livebox-api-client
```

Import the library in your source file:

```golang
import "github.com/Tomy2e/livebox-api-client"
import "github.com/Tomy2e/livebox-api-client/api/request"
```

Create a new client:

```golang
// Client with default HTTP client
client, _ := livebox.NewClient("<admin-password>")

// Client with custom HTTP client
client, _ := livebox.NewClient("<admin-password>", livebox.WithHTTPClient(&http.Client{}))
```

Send requests using the client:

```golang
var r json.RawMessage

_ = client.Request(
    context.Background(),
    request.New("TopologyDiagnostics", "buildTopology", map[string]interface{}{"SendXmlFile": false}),
    &r,
)

fmt.Println(string(r))
```

## Livebox CLI Usage

The `livebox-cli` tool allows to easily send requests to the Livebox API. It writes the JSON responses to stdout.

Pre-built binaries are available in the [Releases](https://github.com/Tomy2e/livebox-api-client/releases) section.
If you have Go installed, you can run it with:

```console
go run github.com/Tomy2e/livebox-api-client/cmd/livebox-cli
```

### Options

The tool accepts the following command-line options:

| Name     | Description                  | Default value |
| -------- | ---------------------------- | ------------- |
| -service | Livebox service              |               |
| -method  | Method to use                |               |
| -params  | Optional JSON-encoded params |               |

The tool reads the following environment variables:

| Name           | Description                           | Default value |
| -------------- | ------------------------------------- | ------------- |
| ADMIN_PASSWORD | Password of the Livebox "admin" user. |               |
