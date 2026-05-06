# MySQL 转 PostgreSQL 迁移教程

本文档介绍如何将 dujiaoka 的 `orders` 表从 MySQL 迁移到 PostgreSQL，包括类型映射、DDL 转换、数据迁移和常见陷阱。

---

## 1. 类型映射表

orders 表中每一列从 MySQL 到 PostgreSQL 的类型映射：

| MySQL 类型 | PostgreSQL 类型 | 说明 |
|---|---|---|
| `int(10) unsigned AUTO_INCREMENT` | `BIGINT GENERATED ALWAYS AS IDENTITY` | 现代标识列；unsigned 通过 BIGINT 范围覆盖 |
| `varchar(32)` | `VARCHAR(32)` | 直接映射 |
| `varchar(64)` | `VARCHAR(64)` | 直接映射 |
| `varchar(255)` | `VARCHAR(255)` | 直接映射 |
| `decimal(10,2)` | `NUMERIC(10,2)` | NUMERIC 是 PostgreSQL 标准写法；DECIMAL 是别名 |
| `int(10) unsigned` | `INT` 或 `BIGINT` | 如果值可能超过 2^31-1，使用 BIGINT |
| `tinyint(4)` | `VARCHAR(32) + CHECK` | 替换为字符串枚举，可读性和可扩展性更好 |
| `longtext` | `TEXT` | PostgreSQL TEXT 无长度限制 |
| `timestamp NULL` | `TIMESTAMPTZ` | 带时区，正确处理 UTC |

### 状态值转换

MySQL 的 `tinyint` 状态字段替换为 VARCHAR + CHECK 约束：

| MySQL 值 | 含义 | PostgreSQL 值 |
|---|---|---|
| `1` | 待支付 | `'pending_payment'` |
| `3` | 已支付 / 已完成 | `'paid'` |
| `4` | 已过期 / 已关闭 | `'expired'` |

---

## 2. DDL 转换对照

### MySQL（原始 dujiaoka）

```sql
CREATE TABLE `orders` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `order_sn` varchar(32) NOT NULL COMMENT '订单号',
  `trade_no` varchar(64) DEFAULT NULL COMMENT '第三方支付交易号',
  `pay_id` int(10) unsigned NOT NULL COMMENT '支付网关 ID',
  `search_pwd` varchar(255) NOT NULL COMMENT '查询密码',
  `total_price` decimal(10,2) NOT NULL COMMENT '原始总价',
  `actual_price` decimal(10,2) NOT NULL COMMENT '实际支付价',
  `goods_price` decimal(10,2) NOT NULL COMMENT '单价',
  `buy_amount` int(10) unsigned NOT NULL COMMENT '购买数量',
  `buy_ip` varchar(64) NOT NULL COMMENT '买家 IP',
  `email` varchar(255) NOT NULL COMMENT '买家邮箱',
  `info` longtext COMMENT '卡密内容',
  `status` tinyint(4) NOT NULL DEFAULT '1' COMMENT '1=待支付, 3=已支付, 4=已过期',
  `gd_name` varchar(255) NOT NULL COMMENT '商品名称',
  `gp_name` varchar(255) NOT NULL COMMENT '商品分组名',
  `coupon_id` int(10) unsigned DEFAULT NULL COMMENT '优惠券 ID',
  `coupon_discount_price` decimal(10,2) DEFAULT NULL COMMENT '优惠券折扣',
  `wholesale_discount_price` decimal(10,2) DEFAULT NULL COMMENT '批发折扣',
  `buy_limit_num` int(10) unsigned DEFAULT NULL COMMENT '购买限制',
  `buy_prompt` varchar(255) DEFAULT NULL COMMENT '购买提示',
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `orders_order_sn_unique` (`order_sn`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='订单表';
```

### PostgreSQL（转换后）

```sql
CREATE TABLE orders (
    id                   BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    order_no             VARCHAR(32) NOT NULL,
    trade_no             VARCHAR(64),
    pay_id               BIGINT NOT NULL,
    order_password       VARCHAR(255) NOT NULL,
    total_price          NUMERIC(10,2) NOT NULL,
    actual_price         NUMERIC(10,2) NOT NULL,
    goods_price          NUMERIC(10,2) NOT NULL,
    buy_amount           INT NOT NULL DEFAULT 1,
    client_ip            VARCHAR(64) NOT NULL,
    email                VARCHAR(255) NOT NULL,
    info                 TEXT,
    status               VARCHAR(32) NOT NULL DEFAULT 'pending_payment'
                         CHECK (status IN ('pending_payment', 'paid', 'expired')),
    gd_name              VARCHAR(255) NOT NULL,
    gp_name              VARCHAR(255),
    coupon_id            BIGINT,
    coupon_discount      NUMERIC(10,2) DEFAULT 0,
    wholesale_discount   NUMERIC(10,2) DEFAULT 0,
    buy_limit_num        INT,
    buy_prompt           VARCHAR(255),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (order_no)
);

CREATE INDEX idx_orders_email ON orders (email);

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_orders_updated_at
    BEFORE UPDATE ON orders
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
```

### 列名变更

| MySQL 列名 | PostgreSQL 列名 | 变更原因 |
|---|---|---|
| `order_sn` | `order_no` | Dujiao-Next 命名规范 |
| `search_pwd` | `order_password` | 语义更清晰 |
| `buy_ip` | `client_ip` | Dujiao-Next 命名规范 |
| `coupon_discount_price` | `coupon_discount` | 缩短命名 |
| `wholesale_discount_price` | `wholesale_discount` | 缩短命名 |

---

## 3. 关键差异说明

### AUTO_INCREMENT → GENERATED ALWAYS AS IDENTITY

MySQL 使用 `AUTO_INCREMENT` 自动生成递增整数。PostgreSQL 提供两种方式：

- **SERIAL / BIGSERIAL**：旧版快捷方式，内部创建一个序列。
- **GENERATED ALWAYS AS IDENTITY**：SQL 标准方式（PostgreSQL 10+）。除非使用 `OVERRIDING SYSTEM VALUE`，否则不允许手动插入 ID 值。

新项目推荐使用 `GENERATED ALWAYS AS IDENTITY`，这是 SQL 标准且提供更好的保护。

### 无符号整数（Unsigned）

PostgreSQL 没有 `UNSIGNED` 修饰符。常见处理方式：

- `INT UNSIGNED`（0 到 4,294,967,295）映射为 `BIGINT` 以覆盖完整的无符号范围，或使用 `INT` + `CHECK >= 0` 约束仅确保非负。
- 主键使用 `BIGINT GENERATED ALWAYS AS IDENTITY`，完全覆盖 `INT UNSIGNED` 范围。

### tinyint 状态 → VARCHAR + CHECK

MySQL 通常使用 `tinyint` 表示状态字段，在应用层定义常量（1=待支付, 3=已支付, 4=已过期）。PostgreSQL 替代方案：

1. **自定义 ENUM 类型**（`CREATE TYPE ... AS ENUM`）：类型安全性最强，但增删值需要 `ALTER TYPE`。
2. **VARCHAR + CHECK 约束**：最灵活，添加新状态只需修改 CHECK 约束。本项目采用此方案。

### longtext → TEXT

PostgreSQL 的 `TEXT` 类型无长度限制，各种大小都高效。无需区分 TINYTEXT、MEDIUMTEXT、LONGTEXT。

### ON UPDATE CURRENT_TIMESTAMP → 触发器函数

MySQL 支持列级 `ON UPDATE CURRENT_TIMESTAMP`。PostgreSQL 没有此功能，标准替代方案是触发器函数：

```sql
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_orders_updated_at
    BEFORE UPDATE ON orders
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
```

该触发器在每次 UPDATE 之前自动将 `updated_at` 设为当前时间戳。

### 存储引擎和字符集

MySQL 的 `ENGINE=InnoDB` 和 `CHARSET=utf8mb4` 声明在 PostgreSQL 中没有对应项。PostgreSQL 使用单一存储引擎，字符编码在数据库级别设置（默认 UTF-8）。

### decimal → NUMERIC

MySQL 的 `DECIMAL(p,s)` 直接映射为 PostgreSQL 的 `NUMERIC(p,s)`。在 PostgreSQL 中 `DECIMAL` 实际上是 `NUMERIC` 的别名，两种写法均可。`NUMERIC` 是 PostgreSQL 中的惯例写法。

---

## 4. 数据迁移步骤

### 方案 A：手动导出/导入（适用于小表，推荐）

#### 第 1 步：从 MySQL 导出数据

```bash
mysqldump -u root -p \
    --no-create-info \
    --complete-insert \
    --skip-extended-insert \
    dujiaoka orders > orders_data.sql
```

- `--no-create-info`：跳过 CREATE TABLE（我们有自己的 PostgreSQL DDL）。
- `--complete-insert`：INSERT 语句包含列名。
- `--skip-extended-insert`：每行一条 INSERT（方便编辑）。

#### 第 2 步：创建 PostgreSQL 表结构

```bash
goose postgres "DATABASE_URL" up
```

或手动执行：

```bash
psql -d dujiao_order -f migrations/001_create_orders.sql
```

#### 第 3 步：转换 SQL 数据

mysqldump 输出在 PostgreSQL 中运行前需要几项转换：

1. **修改列名**：INSERT 语句中的列名改为新 schema 对应名称（如 `order_sn` → `order_no`，`search_pwd` → `order_password`）。
2. **转换状态值**：`1` → `'pending_payment'`，`3` → `'paid'`，`4` → `'expired'`。
3. **转换零日期**：`'0000-00-00 00:00:00'` → `NULL`。
4. **去除反引号引用**：将 `` `column` `` 替换为 `column`。
5. **处理 NULL 值**：确保 `DEFAULT NULL` 列使用 `NULL` 而非 `''`。

状态转换示例（sed 脚本）：

```bash
# 仅供参考，实际转换取决于你的数据格式
sed -i "s/, 1,/, 'pending_payment',/g" orders_data.sql
sed -i "s/, 3,/, 'paid',/g" orders_data.sql
sed -i "s/, 4,/, 'expired',/g" orders_data.sql
```

#### 第 4 步：导入数据到 PostgreSQL

```bash
psql -d dujiao_order -f orders_data.sql
```

#### 第 5 步：重置标识序列

```sql
SELECT setval('orders_id_seq', (SELECT MAX(id) FROM orders));
```

### 方案 B：pgLoader（直接传输）

pgLoader 可同时连接两个数据库，自动转换类型并直接传输数据。

```bash
pgloader mysql://root:password@localhost/dujiaoka \
         postgresql://postgres@localhost/dujiao_order
```

更可控的迁移可使用配置文件：

```
LOAD DATABASE
  FROM      mysql://root:password@localhost/dujiaoka
  INTO      postgresql://postgres@localhost/dujiao_order

WITH include drop, create tables, create indexes, reset sequences,
     workers = 8, concurrency = 1

CAST type datetime to timestamptz,
     type tinyint to smallint

INCLUDING ONLY TABLE NAMES MATCHING ~/orders/
```

**注意**：pgLoader 会创建自己的表结构。如果你想使用自定义 schema（含重命名列和 VARCHAR 状态），应该：

1. 先创建 PostgreSQL 表结构（使用 goose 或手动执行）。
2. 使用 pgLoader 时禁用 `with include drop, create tables`。
3. 或使用手动方案以获得完全控制。

### 方案 C：CSV 导出/导入

```bash
# 从 MySQL 导出
mysql -u root -p -e "
  SELECT id, order_sn AS order_no, trade_no, pay_id,
         search_pwd AS order_password, total_price, actual_price,
         goods_price, buy_amount, buy_ip AS client_ip, email, info,
         CASE status
           WHEN 1 THEN 'pending_payment'
           WHEN 3 THEN 'paid'
           WHEN 4 THEN 'expired'
         END AS status,
         gd_name, gp_name, coupon_id,
         coupon_discount_price AS coupon_discount,
         wholesale_discount_price AS wholesale_discount,
         buy_limit_num, buy_prompt, created_at, updated_at
  FROM orders
  INTO OUTFILE '/tmp/orders.csv'
  FIELDS TERMINATED BY ','
  OPTIONALLY ENCLOSED BY '\"'
  LINES TERMINATED BY '\n';
" dujiaoka

# 导入到 PostgreSQL
psql -d dujiao_order -c "\COPY orders FROM '/tmp/orders.csv' WITH (FORMAT csv, HEADER false);"
```

---

## 5. 验证清单

迁移完成后，请验证以下项目：

- [ ] 行数一致：`SELECT COUNT(*) FROM orders;`（与 MySQL 源对比）
- [ ] 抽查特定订单：`SELECT * FROM orders WHERE order_no = '...';`
- [ ] 状态值为字符串：`SELECT DISTINCT status FROM orders;` 应返回 `'pending_payment'`、`'paid'`、`'expired'`
- [ ] 无零日期：`SELECT COUNT(*) FROM orders WHERE created_at IS NULL;`（应为 0 或预期数量）
- [ ] 索引存在：psql 中 `\d orders` 应显示 `idx_orders_email` 和 `order_no` 唯一约束
- [ ] 触发器正常：`UPDATE orders SET gd_name = gd_name WHERE id = 1;` 然后检查 `updated_at` 已更新
- [ ] 序列已重置：`INSERT INTO orders (order_no, ...) VALUES ('test', ...) RETURNING id;` 应产生正确的下一个 ID
- [ ] 邮箱查询正常：`SELECT * FROM orders WHERE email = 'test@example.com';`
- [ ] order_no 唯一性：`SELECT order_no, COUNT(*) FROM orders GROUP BY order_no HAVING COUNT(*) > 1;` 应返回 0 行

---

## 6. 常见陷阱

### 零日期（Zero Dates）

MySQL 允许 `'0000-00-00 00:00:00'` 作为有效时间戳，PostgreSQL 会拒绝。如果 MySQL 数据中包含零日期，导入前必须转换为 `NULL`。pgLoader 通过 `zero-dates-to-null` 选项自动处理。

### 大小写敏感

MySQL 字符串比较默认不区分大小写（取决于排序规则），PostgreSQL 区分大小写。这对 `email` 列很重要：如果应用需要不区分大小写的邮箱匹配，在查询中使用 `LOWER(email)` 或创建不区分大小写的索引：

```sql
CREATE INDEX idx_orders_email_lower ON orders (LOWER(email));
```

### 无符号整数溢出

如果 MySQL `INT UNSIGNED` 列有超过 2,147,483,647 的值，映射到 PostgreSQL `INT` 会导致溢出。应使用 `BIGINT` 或添加 `CHECK (column >= 0)` 约束。

### 状态值跳跃

dujiaoka 使用状态值 1、3、4（跳过 2）。转换为字符串值时，确保映射覆盖数据中实际存在的所有值。在 MySQL 源上执行 `SELECT DISTINCT status FROM orders;` 确认。

### 序列起始值

导入带有显式 `id` 值的数据后，PostgreSQL 标识序列不会自动推进。必须手动重置：

```sql
SELECT setval('orders_id_seq', (SELECT COALESCE(MAX(id), 0) FROM orders));
```

否则新插入会因主键冲突而失败。

### 触发器归属

如果删除 `orders` 表后重新创建，必须同时重新创建触发器。goose 的 `Down` 迁移会先删除触发器再删除表。

### 编码

MySQL `utf8mb4` 是正确的现代编码。但 MySQL 的 `utf8`（不带 `mb4`）实际是 3 字节子集，无法存储所有 Unicode 字符。确认 MySQL 源使用 `utf8mb4` 而非 `utf8`。PostgreSQL 默认 UTF-8 编码等同于 MySQL 的 `utf8mb4`。

### 标识符长度

PostgreSQL 标识符限制为 63 字节，MySQL 允许 64 字符。对于 orders 表这不是问题，但对于约束名较长的其他表可能需要缩短。
