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
client := livebox.NewClient("<admin-password>")

// Client with custom HTTP client
client := livebox.NewClientWithHTTPClient("<admin-password>", &http.Client{})
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
