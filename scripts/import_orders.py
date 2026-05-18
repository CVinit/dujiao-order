#!/usr/bin/env python3
r"""
Convert MySQL orders.csv to PostgreSQL-compatible CSV and import.

Usage:
    python3 scripts/import_orders.py orders.csv > orders_pg.csv
    docker cp orders_pg.csv dujiao-order-db-1:/tmp/orders_pg.csv
    docker exec -i dujiao-order-db-1 psql -U dujiao -d dujiao_order \
      -c "\COPY orders(...) FROM '/tmp/orders_pg.csv' WITH (FORMAT csv, HEADER true, NULL '');"

Status mapping (MySQL -> PostgreSQL):
    1  -> pending_payment (待支付)
    2  -> paid (待处理 - 已付款等待人工处理)
    3  -> paid (处理中)
    4  -> paid (已完成)
    5  -> expired (处理失败)
    6  -> expired (异常)
    -1 -> expired (已过期)
"""

import csv
import sys

STATUS_MAP = {
    "1": "pending_payment",
    "2": "paid",
    "3": "paid",
    "4": "paid",
    "5": "expired",
    "6": "expired",
    "-1": "expired",
}

PG_COLUMNS = [
    "id", "order_no", "trade_no", "pay_id", "order_password",
    "total_price", "actual_price", "goods_price", "buy_amount",
    "client_ip", "email", "info", "status", "gd_name", "gp_name",
    "coupon_id", "coupon_discount", "wholesale_discount",
    "buy_limit_num", "buy_prompt", "created_at", "updated_at",
]


def convert_row(row):
    status = STATUS_MAP.get(row["status"], "expired")
    trade_no = row["trade_no"] if row["trade_no"] else ""
    coupon_id = row["coupon_id"] if row["coupon_id"] and row["coupon_id"] != "0" else ""
    created_at = row["created_at"] if row["created_at"] and row["created_at"] != "0000-00-00 00:00:00" else ""
    updated_at = row["updated_at"] if row["updated_at"] and row["updated_at"] != "0000-00-00 00:00:00" else ""

    return {
        "id": row["id"],
        "order_no": row["order_sn"],
        "trade_no": trade_no,
        "pay_id": row["pay_id"] or "0",
        "order_password": row["search_pwd"],
        "total_price": row["total_price"],
        "actual_price": row["actual_price"],
        "goods_price": row["goods_price"],
        "buy_amount": row["buy_amount"],
        "client_ip": row["buy_ip"],
        "email": row["email"],
        "info": row["info"] if row["info"] else "",
        "status": status,
        "gd_name": row["title"],
        "gp_name": "",
        "coupon_id": coupon_id,
        "coupon_discount": row["coupon_discount_price"],
        "wholesale_discount": row["wholesale_discount_price"],
        "buy_limit_num": "",
        "buy_prompt": "",
        "created_at": created_at,
        "updated_at": updated_at,
    }


def main():
    if len(sys.argv) < 2:
        print(f"Usage: {sys.argv[0]} <orders.csv>", file=sys.stderr)
        sys.exit(1)

    input_file = sys.argv[1]
    writer = csv.DictWriter(sys.stdout, fieldnames=PG_COLUMNS, quoting=csv.QUOTE_MINIMAL)
    writer.writeheader()

    with open(input_file, "r", encoding="utf-8") as f:
        reader = csv.DictReader(f)
        count = 0
        for row in reader:
            pg_row = convert_row(row)
            writer.writerow(pg_row)
            count += 1

    print(f"Converted {count} rows", file=sys.stderr)


if __name__ == "__main__":
    main()
