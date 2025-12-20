# User Manager Service

**User Manager** is a microservice responsible for user management, including registration, authentication, and profile management. The project is structured following the principles of **Hexagonal Architecture (Ports and Adapters)** to ensure separation between business logic (Core) and infrastructural details (HTTPS, gRPC, Redis).

User operations are implemented with an **at-most-once** policy.

## Project Structure

```plaintext
user-manager/
├── cmd/
│   └── main.go
├── config/
│   └── config.go
├── internal/
│   ├── adapters/
│   │   ├── grpc_server/
│   │   ├── huma_api/
│   │   └── repository/
│   ├── application/
│   │   └── core/
│   │       ├── api/
│   │       ├── apperrors/
│   │       └── domain/
│   ├── middleware/
│   └── ports/
│       ├── api.go
│       └── db.go
├── Dockerfile
└── README.md
```

### 1\. The Core (Business Logic)

Everything residing here is agnostic regarding external technology (DB, HTTP, etc.).

| Directory | Description |
| :--- | :--- |
| `internal/application/core/domain` | **Pure Entities**. Fundamental data structures (`User` struct). |
| `internal/application/core/api` | **Service Layer**. Implements the business logic defined in `ports/api.go`. Orchestrates validations and calls the DB. |
| `internal/application/core/apperrors` | Domain-specific errors subsequently mapped by adapters (e.g., `ErrUserNotFound`). |

### 2\. The Ports (Interfaces)

These define the interaction contracts between the Core and the outside world.

| File | Type | Description |
| :--- | :--- | :--- |
| `internal/ports/api.go` | **Input Port** | Defines the methods exposed by the Core (e.g., `RegisterUser`, `DeleteUser`). |
| `internal/ports/db.go` | **Output Port** | Defines the methods the Core needs to save data (e.g., `Save`, `Delete`). |

### 3\. The Adapters (Infrastructure)

Concrete implementations that communicate with the ports. This is where the division between **Driver** and **Driven** occurs.

| Directory | Role | Description |
| :--- | :--- | :--- |
| `adapters/huma_api` | **Driver** | **Huma**-based HTTP Server. |
| `adapters/grpc_server` | **Driver** | **gRPC** Server. |
| `adapters/repository` | **Driven** | **Redis** Database. |
