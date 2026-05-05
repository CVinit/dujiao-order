# dujiao-order

Go web application for querying orders from a dujiaoka-style system backed by PostgreSQL.

## Quick Start

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
