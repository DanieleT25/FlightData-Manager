# Flight Data Manager - Distributed Microservices System

<img src="./images/FlightData-Manager_logo.png" ref="FlightData-Manager logo">

_Project for "Distributed Systems and Big Data" course of Computer Engineering, extended for "Cloud Systems" course of Computer Science_

_[Daniele Tambone](https://www.linkedin.com/in/daniele-tambone-b5733616a/) @ Dept of Math and Computer Science, University of Catania_

## Overview

**Flight Data Manager** is a robust distributed system designed to manage user subscriptions and monitor real-time flight data from OpenSky Network. The project demonstrates a **Microservices Architecture** built with **Go**, adhering to **Hexagonal Architecture (Ports & Adapters)** principles to ensure decoupling, testability, and scalability.

The system is composed of two main services:
1.  **User Manager:** Handles user identity, secure registration, and idempotency.
2.  **Data Collector:** Manages flight data collection, user interests, and statistical analysis.


## Architecture & Design Choices

The project implements advanced distributed systems patterns:

* **Hexagonal Architecture:** Both services separate Domain Logic (Core) from Infrastructure (Adapters) via Interfaces (Ports).
* **Inter-Service Communication:**
    * **gRPC:** Used for internal, high-performance synchronous communication (e.g., verifying user existence).
    * **REST API (Huma):** Used for external communication with clients.
* **Resiliency:**
    * **At-Most-Once Delivery:** Implemented in User Manager using a custom **Idempotency Key mechanism** (IP Hash + Request ID) backed by Redis.
    * **Fault Tolerance:** The Data Collector uses **Retry** and **Circuit Breaker** patterns when calling the User Manager.
* **Data Storage:**
    * **PostgreSQL:** Relational store for User profiles.
    * **Redis:** Key-Value store for Idempotency locks (Speed).
    * **Neo4j:** Graph database for modeling relationships (Users -> Interests -> Flights).
* **Security:**
    * **mTLS Ready:** Infrastructure code supports Mutual TLS for gRPC.
    * **Bcrypt:** Passwords are hashed before storage.
    * **Access Control:** Data access is guarded by cross-service credential verification.

The system ships with two deployment tracks:
* a local one (via Docker Compose *or* Gitea + OpenTofu + Multipass + Ansible + Kubernetes)
* a cloud one (via Github + OpenTofu + AWS services).

## Project Map

| Path | Contents |
|---|---|
| [`microservices/user-manager`](microservices/user-manager) | User identity, registration, and idempotency (Postgres + Redis) |
| [`microservices/data-collector`](microservices/data-collector) | Flight data collection, user interests, statistics (Neo4j) |
| [`microservices/alert-system`](microservices/alert-system) / [`alert-notifier-system`](microservices/alert-notifier-system) | Kafka-driven alert evaluation and notification |
| [`frontend/`](frontend) | Web client |
| [`nginx.conf`](nginx.conf) | API Gateway configuration |
| [`docker-compose.yml`](docker-compose.yml) | Local multi-container stack (no Kubernetes) |
| [`terraform/`](terraform) | OpenTofu IaC for the local Multipass VMs |
| [`aws/terraform/`](aws/terraform) | OpenTofu IaC for the AWS deployment |
| [`.github/workflows/`](.github/workflows) | AWS CI/CD pipelines (infrastructure, destroy) |
| [`ansible/`](ansible) | Playbooks that bootstrap the kubeadm cluster |
| [`k8s/`](k8s) | Kubernetes manifests for the application and its data stores |
| [`k8sadmin/`](k8sadmin) | Metrics Server and kube-prometheus-stack install scripts |
| [`.gitea/workflows/`](.gitea/workflows) | CI/CD pipelines (provision, deploy, destroy) |
| [`pkg/`](pkg) | Shared proto definitions, TLS cert scripts, Gitea and AWS setup scripts |
| [`schema/`](schema) | Architecture diagrams (local and AWS) |
| [`docs/`](docs) | Setup guides, demo walkthrough, and the full project report |
| [`images/`](images) | Images used in the README |

## Documentation

- [Local Setup (Docker Compose)](docs/local-setup.md) — prerequisites, configuration, running and testing the system locally
- [Kubernetes Deployment](docs/kubernetes-deployment.md) — provisioning and deploying the local Gitea/OpenTofu/Multipass/Ansible/Kubernetes stack
- [AWS Deployment](docs/aws-deployment.md) — cost model, and deploying to AWS with GitHub Actions, OIDC and OpenTofu
- [API Demo & Walkthrough](docs/demo.md) — a guided tour of the main API flows
- [Project Report](docs/Flight_Data_Manager.pdf) — full written report
