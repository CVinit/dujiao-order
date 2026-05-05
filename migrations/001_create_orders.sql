-- +goose Up

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
    status               VARCHAR(32) NOT NULL DEFAULT 'pending_payment' CHECK (status IN ('pending_payment', 'paid', 'expired')),
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

CREATE TRIGGER update_orders_updated_at BEFORE UPDATE ON orders FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- +goose Down

DROP TRIGGER IF EXISTS update_orders_updated_at ON orders;
DROP FUNCTION IF EXISTS update_updated_at_column();
DROP TABLE IF EXISTS orders;
