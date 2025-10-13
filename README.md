# AIMAS API Gateway

The **AIMAS API Gateway** is a custom-built gateway written in **Go**, designed to serve as the central entry point for the **AIMAS** distributed application monitoring platform.
It provides **intelligent routing**, **service discovery**, **rate limiting**, **authentication**, **observability**, and **security enforcement** for all backend microservices.

---

## Key Features

* **YAML-Based Configuration** — Define services, routes, and rate limits using simple `.yaml` files.
* **Dynamic Routing** — Automatically routes requests to the correct backend service based on configuration or service discovery.
* **Service Discovery** — Supports both static YAML configuration and dynamic service registry lookups.
* **Security Headers** — Automatically injects strict HTTP security headers to protect against common web exploits.
* **Gateway Signature Headers** — Each request includes a signed timestamp and gateway signature to verify authenticity.
* **Rate Limiting** — Configurable per-service or per-route throttling.
* **Metrics & Observability** — Built-in metrics for monitoring service health, latency, and traffic volume.
* **Structured Logging** — Centralized JSON logs for all requests and responses.
* **Graceful Error Handling** — Returns structured JSON errors for invalid routes, timeouts, and failures.

---

## Configuration Structure (`.yaml`)

The gateway uses YAML configuration files to define services, routes, and limits.
Service discovery can dynamically register or override these configurations at runtime.

### Example Configuration

```yaml
services:
  - name: user-service
    host: user.api.internal
    version: v1
    prefix: /api/v1
    protocol: http
    description: Handles user registration and authentication
    port: 8080
    rate_limit:
      requests_per_minute: 100
    routes:
      - path: /users
        methods: [GET, POST]

  - name: log-service
    host: log.api.internal
    version: v1
    prefix: /api/v1
    protocol: http
    description: Collects application logs
    port: 9090
    rate_limit:
      requests_per_minute: 60
    routes:
      - path: /logs
        methods: [POST]
```

---

## Configuration Schema

| Key                              | Description                      | Required                 | Example               |
| -------------------------------- | -------------------------------- | ------------------------ | --------------------- |
| `services`                       | List of backend services         | Yes                      | —                     |
| `name`                           | Service identifier               | Yes                      | `user-service`        |
| `host`                           | Backend host (no trailing slash) | Yes                      | `auth.api.internal`   |
| `port`                           | Service port                     | Optional                 | `8080`                |
| `protocol`                       | `http` or `https`                | Optional (default: http) | `https`               |
| `version`                        | Service version                  | Optional                 | `v1`                  |
| `prefix`                         | Route prefix for the service     | Optional                 | `/api/v1`             |
| `description`                    | Human-readable description       | Optional                 | `User management API` |
| `rate_limit.requests_per_minute` | Max requests per minute          | Optional                 | `100`                 |
| `routes.path`                    | Endpoint path                    | Yes                      | `/users`              |
| `routes.methods`                 | Allowed HTTP methods             | Optional                 | `[GET, POST]`         |

---

## Gateway Security Headers (Request → Service)

Every request forwarded from the gateway includes metadata headers for identification and verification:

| Header                | Description                                          |
| --------------------- | ---------------------------------------------------- |
| `X-Forwarded-Proto`   | Original request protocol (`http` or `https`)        |
| `User-Agent`          | Identifies requests as coming from AIMAS Gateway     |
| `X-Request-ID`        | Unique UUID for tracing each request                 |
| `Authorization`       | Gateway-issued internal bearer token                 |
| `X-Gateway-Timestamp` | Unix timestamp (prevents replay attacks)             |
| `X-Gateway-Signature` | HMAC-SHA256 signature of the timestamp               |
| `X-Gateway-Service`   | Name of the gateway instance (e.g., `aimas-gateway`) |

Downstream services verify these headers to ensure requests originated from the trusted gateway and are recent (for example, within 5 minutes).

---

## Response Security Headers

All responses include the following headers to enhance security:

| Header                         | Description                                          |
| ------------------------------ | ---------------------------------------------------- |
| `Strict-Transport-Security`    | Enforces HTTPS connections                           |
| `X-Content-Type-Options`       | Prevents MIME-type sniffing                          |
| `X-XSS-Protection`             | Enables XSS filter in browsers                       |
| `X-Frame-Options`              | Prevents clickjacking (`DENY`)                       |
| `Content-Security-Policy`      | Restricts content loading to same origin             |
| `Referrer-Policy`              | Controls referrer information sent to external sites |
| `Cache-Control`                | Prevents caching of sensitive data                   |
| `Cross-Origin-Opener-Policy`   | Isolates browsing context                            |
| `Cross-Origin-Resource-Policy` | Restricts resource sharing to same origin            |

---

## Metrics and Observability

The gateway collects metrics for:

* Request count, latency, and throughput per route
* Error rates and response codes
* Rate limit and signature validation violations
* Service health and response times

Metrics can be exported to **Prometheus**, **Grafana**, or the **AIMAS Monitoring Dashboard**.

---

## Error Handling

The gateway returns standardized JSON errors for:

| Error Type          | Status Code | Example Message                                 |
| ------------------- | ----------- | ----------------------------------------------- |
| Missing Route       | `404`       | `{"message": "404 route not found"}`            |
| Unauthorized        | `401`       | `{"message": "invalid or missing credentials"}` |
| Forbidden           | `403`       | `{"message": "access denied"}`                  |
| Rate Limit Exceeded | `429`       | `{"message": "too many requests"}`              |
| Internal Error      | `500`       | `{"error": "internal server error"}`            |

---

## Running the Gateway

```bash
go run . --config=config.yaml | go run . --addr=http://configfile-hosted-here #use while running hosted config file
```

Or build and run:

```bash
go build -o aimas-gateway
./aimas-gateway --config=./configs/aimas.yaml |

 go build -o aimas-gateway
./aimas-gateway --config=./configs/aimas.yaml  # use while runnng hosted config file
```

