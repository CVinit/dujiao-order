package main

import (
	"database/sql"
	"time"
)

// Order represents a single order record from the database.
type Order struct {
	ID                int64
	OrderNo           string
	TradeNo           sql.NullString
	PayID             int64
	OrderPassword     string // NOT exposed to frontend
	TotalPrice        float64
	ActualPrice       float64
	GoodsPrice        float64
	BuyAmount         int
	ClientIP          string
	Email             string
	Info              sql.NullString // card secret content
	Status            string
	GDName            string
	GPName            sql.NullString
	CouponID          sql.NullInt64
	CouponDiscount    float64
	WholesaleDiscount float64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Status constants matching the PostgreSQL CHECK constraint.
const (
	StatusPendingPayment = "pending_payment"
	StatusPaid           = "paid"
	StatusExpired         = "expired"
)

// DisplayStatus returns Chinese status text for display in templates.
func (o *Order) DisplayStatus() string {
	switch o.Status {
	case StatusPendingPayment:
		return "待支付"
	case StatusPaid:
		return "已支付"
	case StatusExpired:
		return "已过期"
	default:
		return o.Status
	}
}

// HasCardSecret returns true if the order is paid and has card secret content.
func (o *Order) HasCardSecret() bool {
	return o.Status == StatusPaid && o.Info.Valid && o.Info.String != ""
}
