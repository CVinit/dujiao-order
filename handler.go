package main

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
)

// pageData is the data structure passed to the HTML template.
type pageData struct {
	TurnstileSiteKey string
	QueryType        string
	Email            string
	OrderNo          string
	Orders           []orderDisplay
	SingleOrder      *orderDisplay
	ErrorMessage     string
	HasResult        bool
}

// orderDisplay is a safe view of an Order with no sensitive fields.
// OrderPassword is intentionally omitted.
type orderDisplay struct {
	OrderNo           string
	TradeNo           string
	TotalPrice        string
	ActualPrice       string
	GoodsPrice        string
	BuyAmount         int
	Email             string
	Info              string
	HasCardSecret     bool
	Status            string
	DisplayStatus     string
	GDName            string
	GPName            string
	CouponDiscount    string
	WholesaleDiscount string
	CreatedAt         string
	UpdatedAt         string
}

// orderToDisplay converts an Order to a display-safe struct.
// OrderPassword is never included in the output.
func orderToDisplay(o *Order) orderDisplay {
	d := orderDisplay{
		OrderNo:           o.OrderNo,
		TotalPrice:        fmtPrice(o.TotalPrice),
		ActualPrice:       fmtPrice(o.ActualPrice),
		GoodsPrice:        fmtPrice(o.GoodsPrice),
		BuyAmount:         o.BuyAmount,
		Email:             o.Email,
		Status:            o.Status,
		DisplayStatus:     o.DisplayStatus(),
		GDName:            o.GDName,
		CouponDiscount:    fmtPrice(o.CouponDiscount),
		WholesaleDiscount: fmtPrice(o.WholesaleDiscount),
		CreatedAt:         fmtTime(o.CreatedAt),
		UpdatedAt:         fmtTime(o.UpdatedAt),
		HasCardSecret:     o.HasCardSecret(),
	}
	if o.TradeNo.Valid {
		d.TradeNo = o.TradeNo.String
	}
	if o.GPName.Valid {
		d.GPName = o.GPName.String
	}
	if o.HasCardSecret() {
		d.Info = o.Info.String
	}
	return d
}

// QueryFormHandler renders the order query form page (GET /).
func QueryFormHandler(cfg Config) http.HandlerFunc {
	tmpl := mustParseTemplate()

	return func(w http.ResponseWriter, r *http.Request) {
		data := pageData{
			TurnstileSiteKey: cfg.TurnstileSiteKey,
		}
		if err := tmpl.Execute(w, data); err != nil {
			log.Printf("template error: %v", err)
			http.Error(w, "内部错误", http.StatusInternalServerError)
		}
	}
}

// QueryHandler handles query submissions (POST /query).
func QueryHandler(cfg Config, db *sql.DB, tv *TurnstileVerifier, bt *BanTracker) http.HandlerFunc {
	tmpl := mustParseTemplate()

	return func(w http.ResponseWriter, r *http.Request) {
		ip := safeIP(r)

		// Parse form values.
		if err := r.ParseForm(); err != nil {
			http.Error(w, "表单解析错误", http.StatusBadRequest)
			return
		}

		queryType := r.FormValue("query_type")
		email := r.FormValue("email")
		orderNo := r.FormValue("order_no")
		password := r.FormValue("order_password")
		turnstileToken := r.FormValue("cf-turnstile-response")

		data := pageData{
			TurnstileSiteKey: cfg.TurnstileSiteKey,
			QueryType:        queryType,
			Email:            email,
			OrderNo:          orderNo,
		}

		// Verify Turnstile token if configured.
		if cfg.TurnstileSecretKey != "" {
			if turnstileToken == "" {
				data.ErrorMessage = "请完成人机验证"
				renderTemplate(w, tmpl, data)
				return
			}
			ok, err := tv.Verify(r.Context(), turnstileToken, ip)
			if err != nil {
				log.Printf("turnstile verify error for %s: %v", ip, err)
				data.ErrorMessage = "人机验证失败，请重试"
				renderTemplate(w, tmpl, data)
				return
			}
			if !ok {
				data.ErrorMessage = "人机验证失败，请重试"
				renderTemplate(w, tmpl, data)
				return
			}
		}

		// Validate required fields.
		if password == "" {
			data.ErrorMessage = "请输入查询密码"
			renderTemplate(w, tmpl, data)
			return
		}

		var found bool

		switch queryType {
		case "email":
			if email == "" {
				data.ErrorMessage = "请输入邮箱地址"
				renderTemplate(w, tmpl, data)
				return
			}
			orders, err := QueryOrdersByEmail(r.Context(), db, email, password)
			if err != nil {
				log.Printf("query by email error for %s: %v", ip, err)
				data.ErrorMessage = "邮箱或查询密码错误"
				renderTemplate(w, tmpl, data)
				return
			}
			if len(orders) == 0 {
				// Unified error: do not reveal whether the email exists.
				data.ErrorMessage = "邮箱或查询密码错误"
				bt.RecordFailure(ip)
				renderTemplate(w, tmpl, data)
				return
			}
			data.HasResult = true
			for i := range orders {
				data.Orders = append(data.Orders, orderToDisplay(&orders[i]))
			}
			found = true

		case "order_no":
			if orderNo == "" {
				data.ErrorMessage = "请输入订单号"
				renderTemplate(w, tmpl, data)
				return
			}
			order, err := QueryOrderByOrderNo(r.Context(), db, orderNo, password)
			if err != nil {
				log.Printf("query by order_no error for %s: %v", ip, err)
				data.ErrorMessage = "邮箱或查询密码错误"
				renderTemplate(w, tmpl, data)
				return
			}
			if order == nil {
				// Unified error: do not reveal whether the order_no exists.
				data.ErrorMessage = "邮箱或查询密码错误"
				bt.RecordFailure(ip)
				renderTemplate(w, tmpl, data)
				return
			}
			data.HasResult = true
			d := orderToDisplay(order)
			data.SingleOrder = &d
			found = true

		default:
			data.ErrorMessage = "请选择查询方式"
			renderTemplate(w, tmpl, data)
			return
		}

		if found {
			bt.RecordSuccess(ip)
		}

		renderTemplate(w, tmpl, data)
	}
}

func renderTemplate(w http.ResponseWriter, tmpl *template.Template, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("template render error: %v", err)
		http.Error(w, "内部错误", http.StatusInternalServerError)
	}
}

func mustParseTemplate() *template.Template {
	tmpl, err := template.ParseFiles("templates/query.html")
	if err != nil {
		log.Fatalf("failed to parse template: %v", err)
	}
	return tmpl
}
