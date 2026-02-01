# ⛩️ PolyGate: High-Performance Execution Gateway for Polymarket

[![Go Report Card](https://goreportcard.com/badge/github.com/GoPolymarket/polygate)](https://goreportcard.com/report/github.com/GoPolymarket/polygate)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**PolyGate** is a self-hosted, high-frequency trading sidecar designed for algorithmic traders and builders on Polymarket. 

It abstracts away the complexities of the EVM (signatures, nonces, chain interactions) and provides a clean, standardized HTTP REST API for placing orders.

> 🚀 **Built for the Polymarket Builders Program.**  
> Seamlessly integrated with `go-polymarket-sdk` for maximum performance and reliability.

---

## ✨ Features

- **⚡ HTTP-to-Chain Abstraction**: Trade on Polymarket using simple JSON payloads. No need to manage EIP-712 signatures or EOA wallets in your strategy code.
- **🔌 Automatic Builder Attribution**: Earn rewards effortlessly. PolyGate automatically signs orders with your Builder API Key.
- **🛡️ Embedded Risk Engine**: Pre-trade checks for limits and slippage to prevent costly "fat-finger" errors.
- **🐳 Docker Ready**: Deploy in seconds with a single `docker-compose up` command.
- **🏗️ Builder Friendly**: Perfect for Python, JS, or Rust strategies that need a reliable execution layer.
- **🔐 Non-Custodial Option**: Support client-side signing and typed data generation.
- **🧾 Audit Trail**: In-memory audit log with a simple query endpoint.
- **🗄️ Postgres Ready**: Optional persistence for audit/risk/idempotency.

---

## 🚀 Quick Start

### 1. Configuration

Copy the example config and fill in your credentials:

```bash
cp config.yaml.example config.yaml
```

Edit `config.yaml`:

```yaml
database:
  dsn: "postgres://postgres:postgres@localhost:5432/polygate?sslmode=disable"

chain:
  # Needed for Safe/EIP-1271 verification
  rpc_url: "https://polygon-rpc.com"

polymarket:
  api_key: "YOUR_L2_API_KEY"
  api_secret: "YOUR_L2_API_SECRET"
  api_passphrase: "YOUR_L2_PASSPHRASE"
  private_key: "YOUR_POLYGON_PRIVATE_KEY" # 0x...

risk:
  max_order_value: 500 # Safety limit (USDC)
```

### 2. Run with Docker

```bash
docker-compose up -d
```

### 3. Place Your First Order (Custodial)

Buy 100 shares of "Yes" (Token ID: `123...`) at $0.65:

```bash
curl -X POST http://localhost:8080/v1/orders \
  -H "Content-Type: application/json" \
  -d '{
    "token_id": "21742633143463906290569050155826241533067272736897614382201929731766386979450",
    "side": "BUY",
    "price": 0.65,
    "size": 100
  }'
```

> **Response**:
> ```json
> {
>   "id": "0x8829...",
>   "status": "orders_matched"
> }
> ```

### 4. Typed Data Flow (Non-Custodial)

1) Request typed data:

```bash
curl -X POST http://localhost:8080/v1/orders/typed \
  -H "Content-Type: application/json" \
  -H "X-Gateway-Key: sk-default-12345" \
  -d '{
    "token_id": "21742633143463906290569050155826241533067272736897614382201929731766386979450",
    "side": "BUY",
    "price": 0.65,
    "size": 100,
    "signer": "0xYourSignerAddress"
  }'
```

> 若使用 Safe 签名（`signature_type=2`），需配置 `chain.rpc_url` 才能进行 EIP‑1271 验签。

2) Sign the returned `typed_data`, then submit (include the exact `signable` payload you received):

```bash
curl -X POST http://localhost:8080/v1/orders \
  -H "Content-Type: application/json" \
  -H "X-Gateway-Key: sk-default-12345" \
  -d '{
    "signer": "0xYourSignerAddress",
    "signature": "0x...",
    "signable": { "...": "copy from /v1/orders/typed response" }
  }'
```

### 5. Audit Log (Tenant Scoped)

```bash
curl http://localhost:8080/v1/audit?limit=50 \
  -H "X-Gateway-Key: sk-default-12345"
```

With time filters:

```bash
curl "http://localhost:8080/v1/audit?from=2025-01-01T00:00:00Z&to=2025-01-02T00:00:00Z&limit=50" \
  -H "X-Gateway-Key: sk-default-12345"
```

### 6. Redis / Postgres 可选持久化

如果配置了 `database.dsn`，审计/风控/幂等会落到 Postgres。  
如果未配置 DB 但配置了 `redis.addr`，则使用 Redis 做幂等与审计回退。

### 7. 租户管理（Admin）

需要在 `auth.admin_key` 中设置管理密钥，并通过 `X-Admin-Key` 调用。
租户接口默认会对密钥字段脱敏；如需查看完整凭证，需配置 `auth.admin_secret_key` 并调用专用接口。
审计日志会对 `/v1/tenants`、`/v1/orders`、`/v1/account` 的请求/响应自动脱敏，避免密钥落库。

```bash
curl -X POST http://localhost:8080/v1/tenants \
  -H "Content-Type: application/json" \
  -H "X-Admin-Key: YOUR_ADMIN_KEY" \
  -d '{
    "id": "tenant-a",
    "name": "Team A",
    "api_key": "sk-tenant-a",
    "creds": {
      "l2_api_key": "TENANT_L2_KEY",
      "l2_api_secret": "TENANT_L2_SECRET",
      "l2_api_passphrase": "TENANT_L2_PASSPHRASE"
    },
    "risk": {
      "max_order_value": 1000,
      "max_daily_orders": 100
    }
  }'
```

查看完整凭证（需要额外的 Admin Secret Key）:

```bash
curl http://localhost:8080/v1/tenants/tenant-a/secret \
  -H "X-Admin-Key: YOUR_ADMIN_KEY" \
  -H "X-Admin-Secret: YOUR_ADMIN_SECRET_KEY"
```

仅更新租户凭证:

```bash
curl -X PUT http://localhost:8080/v1/tenants/tenant-a/creds \
  -H "Content-Type: application/json" \
  -H "X-Admin-Key: YOUR_ADMIN_KEY" \
  -H "X-Admin-Secret: YOUR_ADMIN_SECRET_KEY" \
  -d '{
    "creds": {
      "l2_api_key": "TENANT_L2_KEY",
      "l2_api_secret": "TENANT_L2_SECRET",
      "l2_api_passphrase": "TENANT_L2_PASSPHRASE"
    }
  }'
```

---

## 🛠️ Architecture

PolyGate acts as a sidecar to your trading bot:

Simplified:
```mermaid
graph LR
  A[Client/Bot] -->|HTTP JSON| B[PolyGate API]
  B --> C[Polymarket CLOB]
  B --> D[Risk Engine]
  B --> E[Audit Logs]
```

Detailed:
```mermaid
graph TD
  Client[Client/Bot] -->|HTTP JSON| API[PolyGate API (Gin)]
  API --> Health[/health]

  API --> MW[Middleware]
  MW --> Auth[Auth (X-Gateway-Key)]
  MW --> Rate[Rate Limit]
  MW --> Idem[Idempotency]
  MW --> Audit[Audit]

  API --> Handlers[Handlers]
  Handlers --> Orders[OrderHandler]
  Handlers --> Account[AccountHandler]

  Orders --> Gateway[GatewayService]
  Account --> AccountSvc[AccountService]

  Gateway --> TenantMgr[TenantManager]
  Gateway --> Risk[RiskEngine]
  Gateway --> SDK[Polymarket SDK/CLOB]
  Gateway --> EIP1271[EIP-1271 Verifier] --> RPC[Polygon RPC]

  AccountSvc --> Relayer[Builder Relayer]

  Risk --> UsageRepo[Usage Repo] --> PG[(Postgres)]
  UsageRepo --> Redis[(Redis)]
  UsageRepo --> Mem[(In-Memory)]

  Idem --> IdemStore[Idempotency Store] --> PG
  IdemStore --> Redis
  IdemStore --> Mem

  Audit --> AuditSvc[AuditService] --> Logs[(local jsonl)]
  AuditSvc --> PG
  AuditSvc --> Redis

  TenantMgr --> TenantRepo[Tenant Repo] --> PG
  TenantMgr --> Config[config.yaml / Env]
```

## 💰 Monetization (For Developers)

By default, PolyGate is configured to help **you** monetize your tools. 

1. Go to [builders.polymarket.com](https://builders.polymarket.com) and get your Builder API Key.
2. Hardcode your keys in `internal/config/config.go` (or set via `config.yaml`).
3. Distribute this gateway to your users.
4. **Profit**: Every trade routed through this gateway counts towards your volume.

## 📜 License

MIT License. Open source and free to use.
