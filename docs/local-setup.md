# Local Setup (Docker Compose)

This guide covers running the whole system locally with Docker Compose — the fastest path for development and testing, without provisioning a Kubernetes cluster.

## 1. Prerequisites

- **Docker** & **Docker Compose**
- **Go 1.25+** (optional, only for local development outside containers)
- **Restish** (CLI for testing APIs):

**macOS:**
```bash
# Homebrew
brew tap danielgtaylor/restish
brew install restish

# Go (requires Go 1.18+)
go install [github.com/danielgtaylor/restish@latest](https://github.com/danielgtaylor/restish@latest)
```

**Linux:**
```bash
# Go (requires Go 1.18+)
go install [github.com/danielgtaylor/restish@latest](https://github.com/danielgtaylor/restish@latest)

# Homebrew for Linux
brew tap danielgtaylor/restish
brew install restish
```

**Windows:**
```bash
# Go (requires Go 1.18+)
go install [github.com/danielgtaylor/restish@latest](https://github.com/danielgtaylor/restish@latest)
```

## 2. Configuration (.env)

Create a `.env` file in the root directory. You can copy the example below:

```ini
# --- OPENSKY NETWORK (Required for Data Collector) ---
# Use your API Client credentials (recommended) or website login
OPENSKY_USER=your-client-id
OPENSKY_PASSWORD=your-client-secret

# --- NEO4J ---
NEO4J_USER=neo4j
NEO4J_PASSWORD=secret_password

# --- REDIS (idempotency keys only) ---
REDIS_PASSWORD=
REDIS_DB=0

# --- POSTGRES (user records) ---
POSTGRES_USER=user_manager
POSTGRES_PASSWORD=secret_password
POSTGRES_DB=user_manager
```

## 3. Generate Certificates (mTLS)

Before running, generate the TLS certificates for internal gRPC communication and HTTPS:

```bash
chmod +x pkg/scripts/gen_certs.sh
./pkg/scripts/gen_certs.sh
```

## 4. Run the System

Build and start all containers using Docker Compose:

```bash
docker-compose up --build
```

The system will be available via the **API Gateway (Nginx)** at `https://localhost:3443`.

## Testing the API

- **User Manager Docs:** https://localhost:3443/docs/user
- **Data Collector Docs:** https://localhost:3443/docs/data

> [!note]
> See [`docs/demo.md`](demo.md) for a full walkthrough of the main API flows.

## Development & Debugging

- **Postgres CLI:** `docker exec -it postgres psql -U user_manager -d user_manager`
- **Redis CLI:** `docker exec -it redis redis-cli`
- **Neo4j Cypher Shell:** `docker exec -it neo4j cypher-shell -u neo4j -p <password>`
- **Forcing Data Collection:** The worker runs every 12h by default. To force an immediate run:
  `docker-compose restart data-collector`

> [!tip]
>
> If you prefer to use Neo4j Browser, in the `neo4j` service in `docker-compose.yml`:
>
> ```yaml
> neo4j:
>   ...
>   #ports:
>   #  - "7474:7474" # Browser UI
>   #  - "7687:7687"
>   ...
> ```
>
> Remove the comment.
>
> Now you can use Neo4j Browser at http://localhost:7474/browser/

## Tests

Run the full automated suite from the repository root:

```bash
go test ./...
```

Each microservice has unit tests for its core logic and component-integration tests for its public adapter boundary: HTTP/Huma for User Manager and Data Collector, Kafka payload-to-service processing for Alert System and Alert Notifier. The deploy workflow runs this suite before building an image.

## Future implementation

- **JWT Authentication:** Implement token-based auth to avoid sending credentials on every request to the Data Collector.
- **Microservices Testing:** Implement comprehensive unit and integration tests for each microservice (more testing).
