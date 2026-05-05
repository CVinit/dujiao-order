# dujiao-order

Go web application for querying orders from a dujiaoka-style system backed by PostgreSQL.

## Features

- Two query modes: email + order password (list all orders) / order number + order password (single order)
- Card secret (卡密) display with one-click copy
- Cloudflare Turnstile human verification
- Per-IP rate limiting (token bucket)
- Failed attempt tracking with auto-expiring IP bans
- Timing attack prevention (password verified in SQL WHERE clause)
- Unified error messages that never leak email/order existence

## Deploy with Docker Compose (Recommended)

### 1. Clone and configure

```bash
git clone https://github.com/CVinit/dujiao-order.git
cd dujiao-order
```

Create a `.env` file (or export environment variables):

```bash
# Required: set a strong database password
PG_PASSWORD=your_secure_password

# Optional: Cloudflare Turnstile (leave empty to skip)
TURNSTILE_SITE_KEY=0x4AAAAAAAxxxxxxxxxxxx
TURNSTILE_SECRET_KEY=0x4AAAAAAAxxxxxxxxxxxxxxxxxxxxxxxxxxxx

# Optional: rate limiting and ban defaults are fine for most cases
# RATE_LIMIT_RPS=1
# RATE_LIMIT_BURST=3
# BAN_THRESHOLD=10
# BAN_DURATION=15m
```

### 2. Start services

```bash
docker compose up -d
```

This starts:
- **app** — dujiao-order on port 8080
- **db** — PostgreSQL 16 with the orders table auto-created on first run

The migration SQL is mounted into PostgreSQL's `docker-entrypoint-initdb.d/`, so the `orders` table is created automatically when the database initializes.

### 3. Open the page

Visit `http://localhost:8080` in your browser.

### 4. (Optional) Import data from MySQL

If you're migrating from an existing dujiaoka MySQL database, see [docs/mysql-to-postgresql-migration.md](docs/mysql-to-postgresql-migration.md) for the step-by-step guide.

### Manage the deployment

```bash
# View logs
docker compose logs -f app

# Stop
docker compose down

# Stop and remove data (resets the database)
docker compose down -v

# Update to latest image
docker compose pull app
docker compose up -d app
```

## Deploy with Docker (standalone)

If you already have a PostgreSQL instance:

```bash
docker run -d --name dujiao-order \
  -p 8080:8080 \
  -e DATABASE_URL="postgres://user:password@db-host:5432/dujiao_order?sslmode=disable" \
  -e TURNSTILE_SITE_KEY="your-site-key" \
  -e TURNSTILE_SECRET_KEY="your-secret-key" \
  ghcr.io/cvinit/dujiao-order:main
```

Run the migration manually:

```bash
# Install goose
go install github.com/pressly/goose/v3/cmd/goose@latest

# Apply migration
goose postgres "postgres://user:password@db-host:5432/dujiao_order?sslmode=disable" up
```

## Local Development

1. Copy `.env.example` to `.env` and fill in the values.
2. Run the PostgreSQL migration:
   ```bash
   goose postgres "$DATABASE_URL" up
   ```
3. Start the server:
   ```bash
   go run .
   ```
4. Open `http://localhost:8080` in your browser.

## Configuration

All configuration is via environment variables. See `.env.example` for the full list and defaults.

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | (required) | PostgreSQL connection string |
| `LISTEN_ADDR` | `:8080` | Server listen address |
| `TURNSTILE_SITE_KEY` | (empty) | Cloudflare Turnstile site key |
| `TURNSTILE_SECRET_KEY` | (empty) | Cloudflare Turnstile secret key |
| `RATE_LIMIT_RPS` | `1` | Requests per second per IP |
| `RATE_LIMIT_BURST` | `3` | Burst allowance per IP |
| `BAN_THRESHOLD` | `10` | Failed attempts before IP ban |
| `BAN_DURATION` | `15m` | Duration of IP ban |

## Migration Tutorial

See [docs/mysql-to-postgresql-migration.md](docs/mysql-to-postgresql-migration.md) for the full MySQL to PostgreSQL migration guide.

## License

MIT
