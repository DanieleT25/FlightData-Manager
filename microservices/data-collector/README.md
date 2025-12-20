# Data Collector Service

**Data Collector** is a microservice responsible for ingesting, normalizing, and storing flight data from external sources (OpenSky Network) and manages user interests (monitored airports). The project is structured following the principles of **Hexagonal Architecture (Ports and Adapters)** to ensure separation between business logic (Core) and infrastructural details.

## Project Structure

```plaintext
data-collector/
├── cmd/
│   └── main.go
├── config/
│   └── config.go
├── internal/
│   ├── adapters/
│   │   ├── grpc_client/
│   │   ├── huma_api/
│   │   ├── kafka_producer/
│   │   ├── opensky/
│   │   ├── repository/
│   │   └── worker/
│   ├── application/
│   │   └── core/
│   │       ├── api/
│   │       ├── apperrors/
│   │       └── domain/
│   └── ports/
│       ├── api.go
│       ├── db.go
│       └── service_client.go
├── Dockerfile
└── README.md
```

## Compass to Components

### 1\. The Core (Business Logic)

Everything residing here is agnostic regarding external technology (Neo4j, Kafka, HTTP, etc).

| Directory | Description |
| :--- | :--- |
| `internal/application/core/domain` | **Pure Entities**. Fundamental data structures (`Flight`, `Interest` structs). |
| `internal/application/core/api` | **Service Layer**. Implements `ports/api.go`. Orchestrates the collection cycle, verifies credentials, and returns information. |
| `internal/application/core/apperrors` | Domain-specific errors mapped by adapters (e.g., `ErrNoDataFound`, `ErrExternalService`). |

### 2\. The Ports (Interfaces)

These define the interaction contracts between the Core and the outside world.

| File | Type | Description |
| :--- | :--- | :--- |
| `internal/ports/api.go` | **Input Port** | Defines methods exposed by the Core (e.g., `RunCollectionCycle`, `GetFlightsAverage`). |
| `internal/ports/db.go` | **Output Port** | Defines methods for data persistence (e.g., `SetFlight`, `GetInterests`). |
| `internal/ports/service_client.go` | **Output Port** | Defines contracts for external services (OpenSky, User Manager). |

### 3\. The Adapters (Infrastructure)

Concrete implementations that communicate with the ports. This is where the division between **Driver** and **Driven** occurs.

| Directory | Role | Description |
| :--- | :--- | :--- |
| `adapters/huma_api` | **Driver** | **Huma**-based HTTP Server. Handles API requests. |
| `adapters/worker` | **Driver** | **Ticker Worker**. Triggers the `RunCollectionCycle` method periodically. |
| `adapters/repository` | **Driven** | **Neo4j** implementation. Stores flights and user interests as a graph. |
| `adapters/grpc_client` | **Driven** | **gRPC** client. Connects to `user-manager` for credential verification. Includes Circuit Breaker logic.|
| `adapters/opensky` | **Driven** | HTTP client for **OpenSky Network**. Includes Circuit Breaker logic. |
| `adapters/kafka_producer`| **Driven** | **Kafka** producer. Publishes `AirportUpdateEvent` after successful cycles. |
