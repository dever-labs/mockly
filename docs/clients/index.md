# Client Libraries

Mockly ships official clients for both native-process and Docker-backed test setups, so you can choose between a locally managed binary or a containerized Mockly instance.

| Language | Driver package | Testcontainers package | Install |
|---|---|---|---|
| **Go** | `github.com/dever-labs/mockly/clients/go` | `github.com/dever-labs/mockly/clients/go/testcontainers` | `go get github.com/dever-labs/mockly/clients/go` or `go get github.com/dever-labs/mockly/clients/go/testcontainers` |
| **Node.js / TypeScript** | `@dever-labs/mockly-driver` | `@dever-labs/mockly-testcontainers` | `npm i -D @dever-labs/mockly-driver` or `npm i -D @dever-labs/mockly-testcontainers testcontainers` |
| **Java** | `io.github.dever-labs:mockly-driver` | `io.github.dever-labs:mockly-testcontainers` | See Maven/Gradle below |
| **.NET / C#** | `Mockly.Driver` | `Testcontainers.Mockly` | `dotnet add package Mockly.Driver` or `dotnet add package Testcontainers.Mockly` |
| **Python** | `mockly-driver` | `mockly-testcontainers` | `pip install mockly-driver` or `pip install mockly-testcontainers` |
| **Rust** | `mockly-driver` | `mockly-testcontainers` | `mockly-driver = "0.15.0"` <!-- x-release-please-version --> or `mockly-testcontainers = "0.13.1"` <!-- x-release-please-version --> in `[dev-dependencies]` |

Driver clients:
- Automatically find or install the Mockly binary for the current platform
- Allocate two free ports atomically (no TOCTOU races)
- Retry startup up to 3 times on port conflicts

Testcontainers modules:
- Start `ghcr.io/dever-labs/mockly:latest` in Docker using each language's Testcontainers library
- Require no binary download on the host machine
- Expose the same core concepts: `addMock`, `activateScenario`, `setFault`, `reset`, `stop` / cleanup

## Go

```go
import mocklydriver "github.com/dever-labs/mockly/clients/go"

server, err := mocklydriver.Ensure(mocklydriver.Options{}, mocklydriver.InstallOptions{})
defer server.Stop()

server.AddMock(mocklydriver.Mock{
    ID:       "get-user",
    Request:  mocklydriver.Request{Method: "GET", Path: "/users/1"},
    Response: mocklydriver.Response{Status: 200, Body: `{"id":1}`},
})
// server.HTTPBase = "http://127.0.0.1:<port>"
```

[→ Full Go docs](go.md)

## Node.js / TypeScript

```ts
import { MocklyServer } from '@dever-labs/mockly-driver'

const server = await MocklyServer.ensure()
await server.addMock({
    id: 'get-user',
    request: { method: 'GET', path: '/users/1' },
    response: { status: 200, body: '{"id":1}' },
})
// server.httpBase = "http://127.0.0.1:<port>"
await server.stop()
```

[→ Full Node.js docs](node.md)

## Java

```xml
<dependency>
  <groupId>io.github.dever-labs</groupId>
  <artifactId>mockly-driver</artifactId>
  <version>0.15.0</version> <!-- x-release-please-version -->
  <scope>test</scope>
</dependency>
```

```java
try (MocklyServer server = MocklyServer.ensure(MocklyConfig.builder().build())) {
    server.addMock(Mock.builder("get-user",
        MockRequest.builder("GET", "/users/1").build(),
        MockResponse.builder(200).body("{\"id\":1}").build()
    ).build());
    // server.httpBase = "http://127.0.0.1:<port>"
}
```

[→ Full Java docs](java.md)

## .NET / C#

```sh
dotnet add package Mockly.Driver
```

```csharp
await using var server = await MocklyServer.CreateAsync();
await server.AddMockAsync(new Mock {
    Id = "get-user",
    Request  = new MockRequest { Method = "GET", Path = "/users/1" },
    Response = new MockResponse { Status = 200, Body = """{"id":1}""" },
});
// server.HttpBase = "http://127.0.0.1:<port>"
```

[→ Full .NET docs](dotnet.md)

## Python

```sh
pip install mockly-driver
```

```python
from mockly_driver import MocklyServer, Mock, MockRequest, MockResponse

server = MocklyServer.ensure()
server.add_mock(Mock(
    id="get-user",
    request=MockRequest(method="GET", path="/users/1"),
    response=MockResponse(status=200, body='{"id":1}'),
))
# server.http_base = "http://127.0.0.1:<port>"
server.stop()
```

[→ Full Python docs](python.md)

## Rust

```toml
[dev-dependencies]
mockly-driver = "0.15.0" # x-release-please-version
```

```rust
let mut server = MocklyServer::ensure(ServerOptions::default(), Default::default()).unwrap();
server.add_mock(&Mock {
    id: "get-user".into(),
    request: Request { method: "GET".into(), path: "/users/1".into(), ..Default::default() },
    response: Response { status: 200, body: Some(r#"{"id":1}"#.into()), ..Default::default() },
}).unwrap();
// server.http_base = "http://127.0.0.1:<port>"
```

[→ Full Rust docs](rust.md)

## Testcontainers

Every supported language also has Docker-backed Testcontainers support. These modules run `ghcr.io/dever-labs/mockly:latest`, avoid a host binary download, and keep the same Mockly control surface for mocks, scenarios, reset, and fault injection.

### Go

```go
ctx := context.Background()
container, _ := testcontainersmockly.Run(ctx)
defer container.Terminate(ctx)
container.AddMock(ctx, mocklydriver.Mock{
    ID: "ping",
    Request: mocklydriver.MockRequest{Method: http.MethodGet, Path: "/ping"},
    Response: mocklydriver.MockResponse{Status: 200, Body: `{"ok":true}`},
})
httpBase, _ := container.HTTPBase(ctx)
resp, _ := http.Get(httpBase + "/ping")
```

### Node.js / TypeScript

```ts
const container = await new MocklyContainerBuilder().start()
try {
  await container.addMock({
    id: 'ping',
    request: { method: 'GET', path: '/ping' },
    response: { status: 200, body: '{"ok":true}' },
  })
  const response = await fetch(`${container.getHttpBase()}/ping`)
} finally {
  await container.stop()
}
```

### Java

```java
MocklyContainer container = new MocklyContainer();
container.start();
try {
    container.addMock(Mock.builder(
            "ping",
            MockRequest.builder("GET", "/ping").build(),
            MockResponse.builder(200).body("{\"ok\":true}").build()
    ).build());
    HttpResponse<String> response = HttpClient.newHttpClient().send(
            HttpRequest.newBuilder(URI.create(container.getHttpBase() + "/ping")).GET().build(),
            HttpResponse.BodyHandlers.ofString());
} finally {
    container.stop();
}
```

### .NET / C#

```csharp
await using var container = new MocklyBuilder().Build();
await container.StartAsync();
await container.AddMockAsync(new Mock(
    "ping",
    new MockRequest("GET", "/ping"),
    new MockResponse(200, """{"ok":true}""")));
using var http = new HttpClient();
var response = await http.GetAsync($"{container.GetHttpBaseAddress()}/ping");
```

### Python

```python
with MocklyContainer() as container:
    container.add_mock(Mock(
        id="ping",
        request=MockRequest(method="GET", path="/ping"),
        response=MockResponse(status=200, body='{"ok":true}'),
    ))
    response = urllib.request.urlopen(f"{container.get_http_base()}/ping")
```

### Rust

```rust
let container = MocklyContainer::new(MocklyImage::default().start()?);
container.add_mock(&Mock {
    id: "ping".into(),
    request: MockRequest { method: "GET".into(), path: "/ping".into(), headers: Default::default() },
    response: MockResponse { status: 200, body: Some(r#"{"ok":true}"#.into()), headers: Default::default(), delay: None },
})?;
let response = reqwest::blocking::get(format!("{}/ping", container.http_base()))?;
```

Full references:
- [Go](go.md)
- [Node.js / TypeScript](node.md)
- [Java](java.md)
- [.NET / C#](dotnet.md)
- [Python](python.md)
- [Rust](rust.md)

---
