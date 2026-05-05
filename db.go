package main

import (
	"context"
	"database/sql"
	"fmt"
)

// Connect opens a PostgreSQL connection pool using the pgx stdlib adapter.
func Connect(cfg Config) (*sql.DB, error) {
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}

	// The pgx driver was registered in main.go via sql.Register("pgx", stdlib.GetDefaultDriver())
	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(0) // connections are reused indefinitely

	return db, nil
}

// QueryOrdersByEmail returns all orders matching the given email AND order_password.
// The password comparison is done in the SQL WHERE clause to prevent timing
// attacks that would leak whether an email exists. Both conditions are evaluated
// server-side in a single parameterized query, so the response time is the same
// whether the email does not exist or the password is wrong.
// If no rows match, it returns an empty slice (not an error).
func QueryOrdersByEmail(ctx context.Context, db *sql.DB, email, password string) ([]Order, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, order_no, trade_no, pay_id, total_price, actual_price,
		       goods_price, buy_amount, client_ip, email, info, status,
		       gd_name, gp_name, coupon_id, coupon_discount, wholesale_discount,
		       created_at, updated_at
		FROM orders
		WHERE email = $1 AND order_password = $2
		ORDER BY created_at DESC
	`, email, password)
	if err != nil {
		return nil, fmt.Errorf("query by email: %w", err)
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var o Order
		if err := scanOrder(rows, &o); err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	if orders == nil {
		orders = []Order{}
	}
	return orders, nil
}

// QueryOrderByOrderNo returns a single order matching the given order_no AND
// order_password. The password comparison is done in the SQL WHERE clause to
// prevent timing attacks that would leak whether an order_no exists. Both
// conditions are evaluated server-side in a single parameterized query.
// Returns nil if no matching row is found (not an error).
func QueryOrderByOrderNo(ctx context.Context, db *sql.DB, orderNo, password string) (*Order, error) {
	row := db.QueryRowContext(ctx, `
		SELECT id, order_no, trade_no, pay_id, total_price, actual_price,
		       goods_price, buy_amount, client_ip, email, info, status,
		       gd_name, gp_name, coupon_id, coupon_discount, wholesale_discount,
		       created_at, updated_at
		FROM orders
		WHERE order_no = $1 AND order_password = $2
	`, orderNo, password)

	var o Order
	if err := scanOrderRow(row, &o); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query by order_no: %w", err)
	}
	return &o, nil
}

func scanOrder(rows *sql.Rows, o *Order) error {
	return rows.Scan(
		&o.ID, &o.OrderNo, &o.TradeNo, &o.PayID,
		&o.TotalPrice, &o.ActualPrice, &o.GoodsPrice,
		&o.BuyAmount, &o.ClientIP, &o.Email, &o.Info, &o.Status,
		&o.GDName, &o.GPName, &o.CouponID, &o.CouponDiscount,
		&o.WholesaleDiscount, &o.CreatedAt, &o.UpdatedAt,
	)
}

func scanOrderRow(row *sql.Row, o *Order) error {
	return row.Scan(
		&o.ID, &o.OrderNo, &o.TradeNo, &o.PayID,
		&o.TotalPrice, &o.ActualPrice, &o.GoodsPrice,
		&o.BuyAmount, &o.ClientIP, &o.Email, &o.Info, &o.Status,
		&o.GDName, &o.GPName, &o.CouponID, &o.CouponDiscount,
		&o.WholesaleDiscount, &o.CreatedAt, &o.UpdatedAt,
	)
}
