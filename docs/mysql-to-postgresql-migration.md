# MySQL to PostgreSQL Migration Tutorial

This document covers migrating the dujiaoka `orders` table from MySQL to PostgreSQL, including type mapping, DDL conversion, data migration, and common pitfalls.

---

## 1. Type Mapping Table

Every column in the orders table mapped from MySQL to PostgreSQL:

| MySQL Type | PostgreSQL Type | Notes |
|---|---|---|
| `int(10) unsigned AUTO_INCREMENT` | `BIGINT GENERATED ALWAYS AS IDENTITY` | Modern identity column; unsigned handled by BIGINT range |
| `varchar(32)` | `VARCHAR(32)` | Direct mapping |
| `varchar(64)` | `VARCHAR(64)` | Direct mapping |
| `varchar(255)` | `VARCHAR(255)` | Direct mapping |
| `decimal(10,2)` | `NUMERIC(10,2)` | NUMERIC is the PostgreSQL standard; DECIMAL is an alias |
| `int(10) unsigned` | `INT` or `BIGINT` | Use BIGINT if values may exceed 2^31-1 |
| `tinyint(4)` | `VARCHAR(32) + CHECK` | Replaced with string enum for readability and extensibility |
| `longtext` | `TEXT` | PostgreSQL TEXT has no length limit |
| `timestamp NULL` | `TIMESTAMPTZ` | With timezone for correct UTC handling |

### Status Value Conversion

The original MySQL `tinyint` status is replaced with a VARCHAR + CHECK constraint:

| MySQL Value | Meaning | PostgreSQL Value |
|---|---|---|
| `1` | Pending payment | `'pending_payment'` |
| `3` | Paid / completed | `'paid'` |
| `4` | Expired / closed | `'expired'` |

---

## 2. DDL Conversion: Side by Side

### MySQL (original dujiaoka)

```sql
CREATE TABLE `orders` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `order_sn` varchar(32) NOT NULL COMMENT 'Order serial number',
  `trade_no` varchar(64) DEFAULT NULL COMMENT 'Payment transaction number',
  `pay_id` int(10) unsigned NOT NULL COMMENT 'Payment gateway ID',
  `search_pwd` varchar(255) NOT NULL COMMENT 'Query password',
  `total_price` decimal(10,2) NOT NULL COMMENT 'Original total price',
  `actual_price` decimal(10,2) NOT NULL COMMENT 'Actual price paid',
  `goods_price` decimal(10,2) NOT NULL COMMENT 'Unit goods price',
  `buy_amount` int(10) unsigned NOT NULL COMMENT 'Quantity purchased',
  `buy_ip` varchar(64) NOT NULL COMMENT 'Buyer IP address',
  `email` varchar(255) NOT NULL COMMENT 'Buyer email',
  `info` longtext COMMENT 'Card secret content',
  `status` tinyint(4) NOT NULL DEFAULT '1' COMMENT '1=pending, 3=paid, 4=expired',
  `gd_name` varchar(255) NOT NULL COMMENT 'Goods display name',
  `gp_name` varchar(255) NOT NULL COMMENT 'Goods group name',
  `coupon_id` int(10) unsigned DEFAULT NULL COMMENT 'Coupon ID',
  `coupon_discount_price` decimal(10,2) DEFAULT NULL COMMENT 'Coupon discount',
  `wholesale_discount_price` decimal(10,2) DEFAULT NULL COMMENT 'Wholesale discount',
  `buy_limit_num` int(10) unsigned DEFAULT NULL COMMENT 'Purchase limit',
  `buy_prompt` varchar(255) DEFAULT NULL COMMENT 'Purchase note',
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `orders_order_sn_unique` (`order_sn`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Orders table';
```

### PostgreSQL (converted)

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

### Column Name Changes

| MySQL Column | PostgreSQL Column | Reason |
|---|---|---|
| `order_sn` | `order_no` | Dujiao-Next naming convention |
| `search_pwd` | `order_password` | Clearer semantic meaning |
| `buy_ip` | `client_ip` | Dujiao-Next naming convention |
| `coupon_discount_price` | `coupon_discount` | Shortened name |
| `wholesale_discount_price` | `wholesale_discount` | Shortened name |

---

## 3. Key Differences Explained

### AUTO_INCREMENT to GENERATED ALWAYS AS IDENTITY

MySQL uses `AUTO_INCREMENT` on a column to auto-generate sequential integers. PostgreSQL offers two approaches:

- **SERIAL / BIGSERIAL**: Legacy shortcut that creates a sequence behind the scenes.
- **GENERATED ALWAYS AS IDENTITY**: SQL-standard approach (PostgreSQL 10+). Prevents accidental manual inserts unless `OVERRIDING SYSTEM VALUE` is used.

For new schemas, prefer `GENERATED ALWAYS AS IDENTITY` because it is the SQL standard and provides better guardrails.

### Unsigned Integers

PostgreSQL has no `UNSIGNED` modifier. The typical approach:

- `INT UNSIGNED` (0 to 4,294,967,295) maps to `BIGINT` if you need the full unsigned range, or `INT` with a `CHECK >= 0` constraint if you only need non-negative enforcement.
- For primary keys, `BIGINT GENERATED ALWAYS AS IDENTITY` covers the full range of `INT UNSIGNED` and more.

### tinyint Status to VARCHAR with CHECK

MySQL commonly uses `tinyint` for status fields with application-level constants (1=pending, 3=paid, 4=expired). PostgreSQL alternatives:

1. **Custom ENUM type** (`CREATE TYPE ... AS ENUM`): Strictest type safety, but adding or removing values requires `ALTER TYPE`.
2. **VARCHAR + CHECK constraint**: Most flexible. Adding a new status only requires modifying the CHECK constraint. This is the approach used here.

### longtext to TEXT

PostgreSQL's `TEXT` type has no length limit and is efficient for all sizes. There is no need for TINYTEXT, MEDIUMTEXT, or LONGTEXT distinctions.

### ON UPDATE CURRENT_TIMESTAMP to Trigger Function

MySQL supports `ON UPDATE CURRENT_TIMESTAMP` at the column level. PostgreSQL does not have this feature. The standard replacement is a trigger function:

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

This trigger fires automatically before every UPDATE, setting `updated_at` to the current timestamp.

### ENGINE and CHARSET

MySQL's `ENGINE=InnoDB` and `CHARSET=utf8mb4` declarations have no PostgreSQL equivalent. PostgreSQL uses a single storage engine and handles character encoding at the database level (typically UTF-8 by default).

### decimal to NUMERIC

MySQL's `DECIMAL(p,s)` maps directly to PostgreSQL's `NUMERIC(p,s)`. In PostgreSQL, `DECIMAL` is actually an alias for `NUMERIC`, so either name works. `NUMERIC` is the conventional PostgreSQL term.

---

## 4. Data Migration Steps

### Option A: Manual Export/Import (recommended for small tables)

#### Step 1: Export data from MySQL

```bash
mysqldump -u root -p \
    --no-create-info \
    --complete-insert \
    --skip-extended-insert \
    dujiaoka orders > orders_data.sql
```

- `--no-create-info`: Skip CREATE TABLE (we have our own PostgreSQL DDL).
- `--complete-insert`: Include column names in INSERT statements.
- `--skip-extended-insert`: One row per INSERT (easier to edit if needed).

#### Step 2: Create the PostgreSQL schema

```bash
goose postgres "DATABASE_URL" up
```

Or manually:

```bash
psql -d dujiao_order -f migrations/001_create_orders.sql
```

#### Step 3: Convert the SQL data

The mysqldump output needs several transformations before it can run in PostgreSQL:

1. **Change column names** in INSERT statements to match the new schema (e.g., `order_sn` to `order_no`, `search_pwd` to `order_password`).
2. **Convert status values**: Replace `1` with `'pending_payment'`, `3` with `'paid'`, `4` with `'expired'`.
3. **Convert zero dates**: Replace `'0000-00-00 00:00:00'` with `NULL`.
4. **Remove backtick quoting**: Replace `` `column` `` with `column`.
5. **Handle NULL values**: Ensure `DEFAULT NULL` columns use `NULL` not `''`.

A sample `sed` script for status conversion:

```bash
# This is illustrative; actual conversion depends on your data format
sed -i "s/, 1,/, 'pending_payment',/g" orders_data.sql
sed -i "s/, 3,/, 'paid',/g" orders_data.sql
sed -i "s/, 4,/, 'expired',/g" orders_data.sql
```

#### Step 4: Import data into PostgreSQL

```bash
psql -d dujiao_order -f orders_data.sql
```

#### Step 5: Reset the identity sequence

```sql
SELECT setval('orders_id_seq', (SELECT MAX(id) FROM orders));
```

### Option B: pgLoader (for direct transfer)

pgLoader can connect to both databases and transfer data directly with automatic type conversion.

```bash
pgloader mysql://root:password@localhost/dujiaoka \
         postgresql://postgres@localhost/dujiao_order
```

For a more controlled migration, use a configuration file:

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

**Important**: pgLoader will create its own schema. If you want to use the custom schema with renamed columns and VARCHAR status, you should:

1. Create the PostgreSQL schema first (using goose or manually).
2. Use pgLoader with `with include drop, create tables` disabled.
3. Or use the manual approach for full control.

### Option C: CSV Export/Import

```bash
# Export from MySQL
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

# Import into PostgreSQL
psql -d dujiao_order -c "\COPY orders FROM '/tmp/orders.csv' WITH (FORMAT csv, HEADER false);"
```

---

## 5. Verification Checklist

After migration, verify the following:

- [ ] Row count matches: `SELECT COUNT(*) FROM orders;` (compare with MySQL source)
- [ ] Spot check specific orders: `SELECT * FROM orders WHERE order_no = '...';`
- [ ] Status values are strings: `SELECT DISTINCT status FROM orders;` should return `'pending_payment'`, `'paid'`, `'expired'`
- [ ] No zero dates: `SELECT COUNT(*) FROM orders WHERE created_at IS NULL;` (should be 0 or expected count)
- [ ] Indexes exist: `\d orders` in psql should show `idx_orders_email` and unique constraint on `order_no`
- [ ] Trigger works: `UPDATE orders SET gd_name = gd_name WHERE id = 1;` then check `updated_at` changed
- [ ] Sequence is reset: `INSERT INTO orders (order_no, ...) VALUES ('test', ...) RETURNING id;` should produce the next ID
- [ ] Email queries work: `SELECT * FROM orders WHERE email = 'test@example.com';`
- [ ] Order_no uniqueness: `SELECT order_no, COUNT(*) FROM orders GROUP BY order_no HAVING COUNT(*) > 1;` should return 0 rows

---

## 6. Common Pitfalls

### Zero Dates

MySQL allows `'0000-00-00 00:00:00'` as a valid timestamp. PostgreSQL rejects this. If your MySQL data contains zero dates, they must be converted to `NULL` before import. pgLoader handles this automatically with the `zero-dates-to-null` option.

### Case Sensitivity

MySQL string comparisons are case-insensitive by default (depending on collation). PostgreSQL is case-sensitive. This matters for the `email` column: if your application expects case-insensitive email matching, use `LOWER(email)` in queries or create a case-insensitive index:

```sql
CREATE INDEX idx_orders_email_lower ON orders (LOWER(email));
```

### Unsigned Integer Overflow

If a MySQL `INT UNSIGNED` column has values above 2,147,483,647, mapping to PostgreSQL `INT` will cause overflow. Use `BIGINT` instead or add a `CHECK (column >= 0)` constraint.

### Status Value Gaps

dujiaoka uses status values 1, 3, 4 (skipping 2). When converting to string values, make sure the mapping covers all values that actually exist in your data. Run `SELECT DISTINCT status FROM orders;` on the MySQL source to verify.

### Sequence Start Value

After importing data with explicit `id` values, the PostgreSQL identity sequence does not automatically advance. You must reset it:

```sql
SELECT setval('orders_id_seq', (SELECT COALESCE(MAX(id), 0) FROM orders));
```

Otherwise, new inserts will fail with duplicate key errors.

### Trigger Ownership

If you drop the `orders` table and recreate it, you must also recreate the trigger. The goose `Down` migration handles this by dropping the trigger first, then the table.

### Encoding

MySQL `utf8mb4` is the correct modern encoding. However, MySQL's `utf8` (without `mb4`) is actually a 3-byte subset that cannot store all Unicode characters. Verify your MySQL source uses `utf8mb4`, not `utf8`. PostgreSQL's default UTF-8 encoding is equivalent to MySQL's `utf8mb4`.

### Long Column Names

PostgreSQL limits identifiers to 63 bytes. MySQL allows 64 characters. For the orders table this is not an issue, but it can be for other tables with long constraint names.
