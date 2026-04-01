# 🟢 Pulsemon

A production-grade, multi-tenant backend system built in Go that continuously probes HTTP/HTTPS endpoints, tracks latency histograms, monitors SLA compliance, inspects SSL certificates, and delivers email alerts when things go wrong.

---

## What It Does

Pulsemon watches your HTTP/HTTPS services so you don't have to. Register an endpoint, choose a probe interval, and the system takes care of the rest — probing on schedule, recording results, detecting failures, and alerting you before your users notice something is wrong.

---

## Features

- **HTTP & HTTPS Probing** — Continuously probes registered endpoints on a configurable schedule
- **Latency Tracking** — Records avg, p95, and p99 latency per service
- **Failure Streak Detection** — Alerts when a service fails 3 or more consecutive times
- **SLA Compliance Monitoring** — Tracks uptime percentage against your configured SLA target
- **SSL Certificate Inspection** — Monitors certificate validity and expiry for HTTPS endpoints
- **Email Alerts via Resend** — Notifies you on failure streaks, SLA breaches, SSL expiry, and recovery
- **Alert Cooldown** — Prevents alert spam with a 30-minute cooldown window per service
- **Multi-Tenant** — Each user owns and sees only their own data
- **Service Quota** — Each user can monitor up to 3 active services (slot-based)
- **30-Day Data Retention** — Probe results automatically purged after 30 days
- **REST API** — Clean, documented API for integration with any frontend

---

## Tech Stack

| Concern | Technology |
|---|---|
| Language | Go |
| HTTP Framework | Gin |
| Database | PostgreSQL |
| ORM & Migrations | GORM |
| Authentication | JWT |
| Email | Resend API |
| SSL Inspection | Go `crypto/tls` |
| Configuration | Environment Variables |

---

## Project Structure

```
pulsemon/
│
├── cmd/
│   └── api/
│       └── main.go               # Entry point — wires everything together
│
├── internal/
│   ├── auth/                     # Registration, login, JWT issuance
│   │   ├── handler.go
│   │   ├── service.go
│   │   └── repository.go
│   │
│   ├── services/                 # Service CRUD + 3-service quota enforcement
│   │   ├── handler.go
│   │   ├── service.go
│   │   └── repository.go
│   │
│   ├── scheduler/                # Ticker goroutines per service
│   │   └── scheduler.go
│   │
│   ├── worker/                   # HTTP probe + SSL inspection worker pool
│   │   ├── pool.go
│   │   └── probe.go
│   │
│   ├── processor/                # Stores results, updates streaks, calculates SLA
│   │   └── processor.go
│   │
│   ├── alerts/                   # Email alert engine with cooldown logic
│   │   └── engine.go
│   │
│   └── dashboard/                # Read-only health data endpoints
│       ├── handler.go
│       └── repository.go
│
├── pkg/
│   ├── database/                 # PostgreSQL connection via GORM
│   │   └── postgres.go
│   │
│   ├── middleware/               # JWT auth middleware for Gin
│   │   └── auth.go
│   │
│   ├── models/                   # Shared domain models (User, Service, etc.)
│   │   └── models.go
│   │
│   └── config/                   # Environment variable loading
│       └── config.go
│
├── .env.example                  # Environment variable template
├── go.mod
└── go.sum
```

---

## Getting Started

### Prerequisites

- Go 1.21+
- PostgreSQL 14+
- A [Resend](https://resend.com) account and API key

### Installation

**1. Clone the repository**
```bash
git clone https://github.com/hayordeji/pulsemon.git
```

**2. Install dependencies**
```bash
go mod tidy
```

**3. Set up environment variables**
```bash
cp .env.example .env
```

Open `.env` and fill in your values (see Environment Variables section below).

**4. Create the PostgreSQL database**
```bash
createdb pulsemon
```

**5. Run the application**
```bash
go run cmd/api/main.go
```

The server starts on `http://localhost:8080` by default.

---

## Environment Variables

```env
# Server
APP_PORT=8080
APP_ENV=development

# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=yourpassword
DB_NAME=service_monitor

# JWT
JWT_SECRET=your_jwt_secret_key
JWT_EXPIRY_HOURS=24

# Resend Email
RESEND_API_KEY=re_your_api_key
RESEND_FROM_EMAIL=alerts@yourdomain.com

# Worker Pool
WORKER_POOL_SIZE=20

# Alert Cooldown (minutes)
ALERT_COOLDOWN_MINUTES=30
```

---

## API Reference

All protected routes require:
```
Authorization: Bearer <jwt_token>
```

### Auth

| Method | Route | Description | Auth |
|---|---|---|---|
| POST | `/auth/register` | Create a new account | No |
| POST | `/auth/login` | Login and receive JWT | No |

### Services

| Method | Route | Description | Auth |
|---|---|---|---|
| GET | `/services` | List all services (summary) | Yes |
| POST | `/services` | Register a new service | Yes |
| GET | `/services/:id` | Get full service details | Yes |
| PUT | `/services/:id` | Update service configuration | Yes |
| DELETE | `/services/:id` | Delete a service permanently | Yes |

### Dashboard

| Method | Route | Description | Auth |
|---|---|---|---|
| GET | `/dashboard/:service_id` | Health overview for a service | Yes |
| GET | `/dashboard/:service_id/alerts` | Alert history for a service | Yes |

---

## Business Rules

- A user may have at most **3 active services** at any time
- Deleting a service **frees the slot** — it is slot-based, not lifetime-limited
- Service **URLs are immutable** after creation — delete and recreate to change a URL
- Probe results are **retained for 30 days** then automatically purged
- **Failure streak alert** triggers at 3 or more consecutive failures
- **SLA breach alert** triggers when `sla_percentage` drops below `sla_target`
- **SSL expiry alert** triggers when certificate has fewer than 30 days remaining
- **Alert cooldown** is 30 minutes — no repeated alerts within that window
- All service routes return `404` for both missing and unauthorized resources — other users' data is never revealed


## Alert Types

| Alert | Trigger Condition |
|---|---|
| `failure_streak` | 3 or more consecutive probe failures |
| `sla_breach` | SLA percentage drops below configured target |
| `ssl_expiry` | SSL certificate expires in fewer than 30 days |
| `recovery` | Service recovers after a failure streak |

---

---

## Author

Built by Ayodeji Shoga (https://github.com/hayordeji) as a side project.
