# AIMAS API Gateway

The **AIMAS API Gateway** is a custom-built gateway written in **Go**, designed to serve as the entry point for AIMAS  a Sentry-like distributed application monitoring system. It provides centralized routing, service discovery, authentication, metrics, and observability across multiple microservices.

---

## Key Features

* **YAML-Based Configuration** — Simple and structured `.yaml` configuration for defining services and routes.
* **Dynamic Routing** — Automatically maps routes to backend services.
* **Protocol Support** — Supports **HTTP**, **HTTPS**, and **gRPC** communication.
* **Authentication Middleware** — Built-in middleware for JWT and key-based authentication.
* **Rate Limiting** — Configurable rate limits per service or per route.
* **Metrics & Monitoring** — Built-in metrics collection for service health and performance monitoring.
* **Request Logging** — Centralized structured logging for all incoming requests.
* **Graceful Error Handling** — Handles service failures, invalid configs, and bad routes gracefully.

---

## Configuration Structure (`.yaml`)

The gateway uses a YAML configuration file to define how services and routes should be handled.

### Example Configuration

```yaml
services:
  - name: user-service
    host: user.api.internal
    port: 8080
    protocol: http
    secret_key: "user_service_secret"
    routes:
      - path: /users
        methods: [GET, POST]
        description: Handles user registration and listing
        rate_limit: 100
        auth_required: true

  - name: log-service
    host: log.api.internal
    protocol: grpc
    description: Collects application logs for monitoring
    routes:
      - path: /logs
        methods: [POST]
        auth_required: false

timeouts:
  client_timeout: 30m
```

---

## Configuration Key Reference

| Key             | Description                                | Required                   | Example                        |
| --------------- | ------------------------------------------ | -------------------------- | ------------------------------ |
| `services`      | List of all configured microservices       | x                          | —                              |
| `name`          | Unique identifier for the service          | x                          | `user-service`                 |
| `host`          | Service host (no trailing slash)           | x                          | `auth.api.internal`            |
| `port`          | Service port                               | Optional                   | `8080`                         |
| `protocol`      | Service protocol (`http`, `https`, `grpc`) | Optional                   | `http`                         |
| `routes`        | List of route configurations               | x                          | —                              |
| `path`          | URL path pattern                           | x                          | `/users`                       |
| `methods`       | Allowed HTTP verbs                         | Optional (defaults to GET) | `[GET, POST]`                  |
| `description`   | Optional route description                 | Optional                   | `Handles user CRUD operations` |
| `rate_limit`    | Requests per minute limit                  | Optional                   | `100`                          |
| `auth_required` | Enable JWT verification                    | Optional                   | `true`                         |
| `headers`       | Custom headers to add or forward           | Optional                   | `{ "X-Trace-ID": "..." }`      |
| `timeouts`      | Client timeout configuration               | Optional                   | `30m`                          |
| `secret_key`    | Secret key for JWT validation              | Optional                   | `mysecretkey`                  |

---

## Metrics and Monitoring

The gateway exposes internal metrics for:

* Request rate and latency per route
* Error and status code counts
* Active connections and throughput
* Authentication and rate limit violations

These can be integrated into Prometheus, Grafana, or AIMAS' internal dashboard.

---

## Authentication Middleware

The built-in authentication middleware supports:

* **JWT Verification** using service-specific `SecretKey`
* **Header-based API Keys**
* **Custom Claims Validation**

Each route can independently specify `auth_required: true` in the configuration to enforce authentication.

---

## Error Handling

The gateway returns descriptive and structured JSON error responses for:

* Invalid or unreachable backend services
* Malformed configuration files
* Unauthorized access (401/403)
* Method not allowed (405)

---

## Running the Gateway

```bash
go run main.go --config=config.yaml
```

Or build and run:

```bash
go build -o aimas-gateway
./aimas-gateway --config=./configs/aimas.yaml
```

---

## Future Enhancements

* Service discovery integration
* Centralized configuration reloads
* Advanced circuit breaking and retries
* gRPC streaming support
