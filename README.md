# AIMAS API Gateway

The **AIMAS API Gateway** is a Go-based reverse proxy that routes incoming requests to the appropriate backend microservices in the AIMAS ecosystem. It provides **centralized traffic management**, **structured logging**, and **rate limiting**, ensuring reliability and observability across all internal services.

---

## Overview

The gateway acts as the single entry point for all AIMAS backend requests.
It inspects incoming paths, matches them against service prefixes defined in the configuration file, and forwards requests to the correct backend target.

Every forwarded request includes a **unique `X-Request-ID`**, enabling distributed tracing across services.
All transactions are logged with relevant metadata such as latency, target service, and response status.

---

## Core Features

* **Dynamic Service Routing** — Routes traffic based on path prefixes.
* **Structured Logging** — Logs include service name, latency, request ID, and status code.
* **Rate Limiting** — Prevents service overload with per-service request quotas.
* **Tracing Support** — Automatically attaches a unique `X-Request-ID` to every request.
* **Security Headers** — Adds standard HTTP headers for improved safety and observability.
* **Centralized Configuration** — All services are managed through a single YAML file.

---

## Configuration

The gateway uses a single configuration file (`config.yaml`) to register backend services, their prefixes, and their rate limits.

### Example: `config.yaml`

```yaml
services:
  - name: user-service
    host: http://localhost:9001
    prefix: /users
    rate_limit:
      requests_per_minute: 120

  - name: movie-service
    host: http://localhost:9002
    prefix: /movies
    rate_limit:
      requests_per_minute: 100
```

### Configuration Schema

| Key                              | Description                                                    | Example                 |
| -------------------------------- | -------------------------------------------------------------- | ----------------------- |
| `name`                           | Unique identifier for the backend service                      | `user-service`          |
| `host`                           | Full service base URL                                          | `http://localhost:9001` |
| `prefix`                         | URL path prefix used to route requests                         | `/users`                |
| `rate_limit.requests_per_minute` | Maximum number of allowed requests per minute for this service | `120`                   |

---

## How It Works

1. The gateway loads the `config.yaml` file during startup.
2. When a client sends a request, the gateway matches the path prefix (e.g., `/movies`) with the configured services.
3. It forwards the request to the corresponding backend service host.
4. A unique `X-Request-ID` is generated (if missing) and attached.
5. The response is streamed back to the client.
6. The gateway logs all request and response details, including latency, status, and service name.

---

## Running the Gateway

By default, the gateway automatically loads `config.yaml` from the working directory.
No additional flags or parameters are required.

```bash
go run .
```

or after building:

```bash
go build -o aimas-gateway
./aimas-gateway
```

---

## Example Flow

**Request:**

```
GET /movies/latest
```

**Gateway Action:**

* Matches `/movies` prefix → forwards to `movie-service` at `http://localhost:9002/movies/latest`
* Adds `X-Request-ID` header
* Logs request metadata and duration

