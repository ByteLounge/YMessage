# YMessage
> Private. Fast. Beautiful.

YMessage is a complete, production-ready, end-to-end encrypted messaging platform. Inspired by the clean visual aesthetics of Apple's iMessage, YMessage is engineered from the ground up to support millions of concurrent users using a modular monorepo structure.

---

## 1. System Architecture

```text
                                +-----------------------------+
                                |    Client Application UI    |
                                | (Web React / Flutter Mobile) |
                                +--------------+--------------+
                                               |
                                               | (HTTPS / WS Protocols)
                                               v
                                +--------------+--------------+
                                |      Nginx reverse proxy    |
                                +--------------+--------------+
                                               |
                                               |
                       +-----------------------+-----------------------+
                       |                                               |
                       v                                               v
        +--------------+--------------+                 +--------------+--------------+
        |   Go API Gateway / Auth     |                 |  Go Chat WebSocket Hub      |
        +--------------+--------------+                 +--------------+--------------+
                       |                                               |
        +--------------+--------------+                 +--------------+--------------+
        |     PostgreSQL DB           |                 |  Redis PubSub Messaging     |
        | (Users, Prekeys, Message Log)|                 | (Live WebSocket Router)     |
        +-----------------------------+                 +-----------------------------+
```

---

## 2. Directory Layout

```text
YMessage/
├── .github/                  # CI/CD Workflows
│   └── workflows/ci.yml      # GitHub Actions automated build and test pipeline
├── backend/                  # Go-based api gateway and chat backend
│   ├── cmd/server/           # Backend application bootstrap (main.go)
│   ├── internal/             # Domain logic modules
│   │   ├── admin/            # Administrative moderation and telemetry metrics
│   │   ├── auth/             # JWT-based device session and profiles
│   │   ├── chat/             # WS server, connection hub, and message history
│   │   ├── crypto/           # E2EE key distribution (X3DH)
│   │   ├── database/         # Database pools and ORM configurations
│   │   ├── models/           # Shared database schema models
│   │   └── media/            # S3 file uploads and image thumbnail compression
│   ├── Dockerfile            # Multi-stage optimized Docker runner
│   └── go.mod                # Go module dependency manifest
├── frontend/                 # Next.js React Web App and Admin Panel
│   ├── pages/                # Pages Router (index, login, admin)
│   ├── components/           # Glassmorphic Tailwind UI components
│   ├── styles/               # Global css rules & tailwind directives
│   ├── utils/                # Web Crypto API client-side E2EE helpers
│   ├── Dockerfile            # Frontend image compilation
│   └── package.json          # Node package configurations
├── mobile/                   # Flutter App (Mobile and Desktop)
│   ├── lib/
│   │   ├── services/         # E2EE cryptography, websockets, and http API clients
│   │   ├── views/            # Cupertino screens (Login, Chats list, Chat room)
│   │   └── main.dart         # Flutter application bootstrap
│   └── pubspec.yaml          # Flutter package configurations
├── docker/                   # Docker deployment configurations
│   ├── docker-compose.yml    # Full service stack orchestration
│   └── Nginx.conf            # Gateway routing rules
├── k8s/                      # Kubernetes deployment templates
│   └── ymessage-production.yaml # Consolidated StatefulSet and Ingress manifest
└── terraform/                # Infrastructure as Code
    ├── main.tf               # AWS networks, database, cache, and storage configurations
    └── variables.tf          # Terraform environment variables
```

---

## 3. End-to-End Encryption (E2EE) Specification

YMessage guarantees complete user privacy. Messages are encrypted on the sending device and can only be decrypted by the recipient. The server stores only base64-encoded encrypted payloads and initialization vectors.

### Extended Triple Diffie-Hellman (X3DH) Key Agreement
To establish a secure session:
1. **Keys Generation:** Every client device generates:
   - **Identity Key ($IK$):** A long-term ECDH key pair.
   - **Signed Prekey ($SPK$):** A medium-term ECDH key pair signed by $IK$.
   - **One-Time Prekeys ($OPK$):** A list of single-use ECDH key pairs.
2. **Publishing:** Public keys are uploaded to the backend server. Private keys never leave the device.
3. **Session Derivation:** When Alice messages Bob:
   - Alice fetches Bob's prekey bundle from the server, consuming one of Bob's $OPK$.
   - Alice derives a shared symmetric key ($SK$) via ECDH math:
     $$SK = \text{HKDF}(DH(IK_A, SPK_B) \mathbin{\Vert} DH(EK_A, IK_B) \mathbin{\Vert} DH(EK_A, SPK_B) \mathbin{\Vert} DH(EK_A, OPK_B))$$
   - Alice encrypts the message using **AES-256-GCM** with $SK$.
   - Bob receives the payload, pulls Alice's public keys, computes the same $SK$, and decrypts.

---

## 4. API Documentation

### 1. Authentication
* **`POST /api/auth/register`**
  - Registers a new user.
  - Body: `{ "username": "...", "email": "...", "password": "...", "display_name": "..." }`
* **`POST /api/auth/login`**
  - Authenticates a user and registers active device.
  - Body: `{ "username": "...", "password": "...", "platform": "web/mobile", "device_name": "...", "identity_key": "..." }`
  - Returns: `{ "access_token": "...", "refresh_token": "...", "user": { ... } }`
* **`POST /api/auth/refresh`**
  - Generates new token pair using refresh token.
  - Body: `{ "refresh_token": "..." }`
* **`GET /api/auth/profile`**
  - Retrieves authenticated user profile. (Requires Authorization header).

### 2. Cryptographic Prekeys (E2EE)
* **`POST /api/crypto/prekey`**
  - Uploads device public prekey bundle.
  - Body: `{ "identity_key": "...", "signed_prekey": "...", "signed_prekey_id": 1, "signature": "...", "one_time_prekeys": [...] }`
* **`GET /api/crypto/prekey/:userId`**
  - Retrieves E2EE prekey bundle for starting chat with recipient. Marks a one-time prekey as used.

### 3. Messaging & History
* **`GET /api/chat/chats`**
  - Retrieves all active direct and group conversations.
* **`GET /api/chat/messages`**
  - Retrieves cursor-paginated messages for a conversation.
  - Query parameters: `chat_id` (UUID), `cursor` (ISO Timestamp)
* **`GET /api/chat/ws`**
  - Upgrades request connection to WebSocket protocol for real-time delivery.

---

## 5. Deployment Guide

### Docker Compose
Run the entire stack locally in one command:
```bash
docker-compose -f docker/docker-compose.yml up --build
```
Once healthy, access:
* **Next.js Web App:** `http://localhost`
* **Go Backend APIs:** `http://localhost/api`

### Kubernetes
Apply the production cluster manifest:
```bash
kubectl apply -f k8s/ymessage-production.yaml
```

### Terraform
Deploy cloud infrastructure (VPC, Subnets, RDS database, S3 object storage, Redis cluster) on AWS:
```bash
cd terraform
terraform init
terraform plan -out=tfplan
terraform apply tfplan
```
