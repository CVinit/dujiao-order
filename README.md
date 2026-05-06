# dujiao-order

基于 Go 语言的独角数卡（dujiaoka）订单查询页面，使用 PostgreSQL 数据库。

## 功能特性

- 双模式查询：邮箱 + 查询密码（查询所有订单）/ 订单号 + 查询密码（查询单条订单）
- 卡密内容展示，一键复制
- Cloudflare Turnstile 人机验证
- 基于 IP 的令牌桶限流
- 连续查询失败自动封禁（到期自动解封）
- 时序攻击防护（密码在 SQL WHERE 子句中比对）
- 统一错误提示，不泄露邮箱或订单号是否存在

## Docker Compose 部署（推荐）

### 1. 克隆并配置

```bash
git clone https://github.com/CVinit/dujiao-order.git
cd dujiao-order
```

创建 `.env` 文件（或直接导出环境变量）：

```bash
# 必填：设置一个安全的数据库密码
PG_PASSWORD=your_secure_password

# 可选：Cloudflare Turnstile 密钥（留空则跳过验证）
TURNSTILE_SITE_KEY=0x4AAAAAAAxxxxxxxxxxxx
TURNSTILE_SECRET_KEY=0x4AAAAAAAxxxxxxxxxxxxxxxxxxxxxxxxxxxx

# 可选：限流和封禁参数，默认值适用于大多数场景
# RATE_LIMIT_RPS=1
# RATE_LIMIT_BURST=3
# BAN_THRESHOLD=10
# BAN_DURATION=15m
```

### 2. 启动服务

```bash
docker compose up -d
```

启动内容包括：
- **app** — dujiao-order 应用，监听 8080 端口
- **db** — PostgreSQL 16，首次启动自动创建 orders 表

迁移 SQL 通过 `docker-entrypoint-initdb.d/` 挂载，数据库初始化时自动创建 `orders` 表。

### 3. 访问页面

浏览器打开 `http://localhost:8080`。

### 4.（可选）从 MySQL 导入数据

如果你从已有的 dujiaoka MySQL 数据库迁移，请参考 [MySQL→PostgreSQL 迁移教程](docs/mysql-to-postgresql-migration.md)。

### 管理部署

```bash
# 查看日志
docker compose logs -f app

# 停止服务
docker compose down

# 停止并删除数据（重置数据库）
docker compose down -v

# 更新到最新镜像
docker compose pull app
docker compose up -d app
```

## Docker 独立部署

如果你已有 PostgreSQL 实例：

```bash
docker run -d --name dujiao-order \
  -p 8080:8080 \
  -e DATABASE_URL="postgres://user:password@db-host:5432/dujiao_order?sslmode=disable" \
  -e TURNSTILE_SITE_KEY="your-site-key" \
  -e TURNSTILE_SECRET_KEY="your-secret-key" \
  ghcr.io/cvinit/dujiao-order:main
```

手动执行数据库迁移：

```bash
# 安装 goose
go install github.com/pressly/goose/v3/cmd/goose@latest

# 执行迁移
goose postgres "postgres://user:password@db-host:5432/dujiao_order?sslmode=disable" up
```

## 本地开发

1. 复制 `.env.example` 为 `.env` 并填入配置值。
2. 执行 PostgreSQL 迁移：
   ```bash
   goose postgres "$DATABASE_URL" up
   ```
3. 启动服务：
   ```bash
   go run .
   ```
4. 浏览器打开 `http://localhost:8080`。

## 配置项

所有配置通过环境变量管理，详见 `.env.example`。

| 变量 | 默认值 | 说明 |
|---|---|---|
| `DATABASE_URL` | （必填） | PostgreSQL 连接字符串 |
| `LISTEN_ADDR` | `:8080` | 服务监听地址 |
| `TURNSTILE_SITE_KEY` | （空） | Cloudflare Turnstile 站点密钥 |
| `TURNSTILE_SECRET_KEY` | （空） | Cloudflare Turnstile 密钥 |
| `RATE_LIMIT_RPS` | `1` | 每 IP 每秒请求数 |
| `RATE_LIMIT_BURST` | `3` | 每 IP 突发允许量 |
| `BAN_THRESHOLD` | `10` | 连续失败次数达到此值后封禁 IP |
| `BAN_DURATION` | `15m` | IP 封禁时长 |

## 迁移教程

详见 [MySQL→PostgreSQL 迁移教程](docs/mysql-to-postgresql-migration.md)。

## 许可证

MIT
