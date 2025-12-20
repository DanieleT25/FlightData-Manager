# Alert System Service

**Alert System** is an event-driven microservice responsible for monitoring flight data changes. It evaluates incoming data against user-defined thresholds and generates notifications when specific conditions are met (e.g., low traffic or high congestion).

The project is structured following the principles of **Hexagonal Architecture (Ports and Adapters)**, ensuring that the alerting logic remains decoupled from the messaging infrastructure and database.

## Project Structure

```plaintext
alert-system/
├── cmd/
│   └── main.go
├── config/
│   └── config.go
├── internal/
│   ├── adapters/
│   │   ├── kafka_consumer/
│   │   ├── kafka_producer/
│   │   └── repository/
│   ├── application/
│   │   └── core/
│   │       ├── api/
│   │       └── domain/
│   └── ports/
│       └── service.go
├── Dockerfile
└── README.md
```

## Compass to Components

### 1\. The Core (Business Logic)

Everything residing here is agnostic regarding external technology (Kafka, Neo4j, etc.).

| Directory | Description |
| :--- | :--- |
| `internal/application/core/domain` | **Pure Entities**. Fundamental data structures ( `Notification`, `UserInterest` structs). |
| `internal/application/core/api` | **Service Layer**. Implements the `AlertService` interface. It receives airport data, fetches interested users, evaluates thresholds, and triggers notifications. |

### 2\. The Ports (Interfaces)

These define the interaction contracts between the Core and the outside world.

| File | Type | Description |
| :--- | :--- | :--- |
| `internal/ports/service.go` | **Input Port** | `AlertService`: Defines the core use case to evaluate flight metrics against user-defined thresholds (`CheckThresholds`). |
| `internal/ports/service.go` | **Output Port** | ``InterestRepository``: Defines the contract for retrieving users interested in a specific airport (`GetUsersByAirport`). |
| `internal/ports/service.go` | **Output Port** | ``NotificationProducer``: Defines the contract for publishing the generated alert event to the messaging system (`SendNotification`). |


### 3\. The Adapters (Infrastructure)

Concrete implementations that communicate with the ports. This is where the division between **Driver** and **Driven** occurs.

| Directory | Role | Description |
| :--- | :--- | :--- |
| `adapters/kafka_consumer` | **Driver** | Listens to the `to-alert-system` topic. When a message arrives, it **invokes** the Core (`CheckThresholds`). |
| `adapters/repository` | **Driven** | **Neo4j** implementation. Retrieves users interested in a specific airport. |
| `adapters/kafka_producer`| **Driven** | **Kafka** producer. If the Core decides an alert is needed, this adapter sends the `Notification` to the `to-notifier` topic. |
