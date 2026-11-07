# Lovelace actions demo

A stand-in for a customer's own systems, used to try agent actions end to end.

Lovelace is a fictional project management SaaS:

- Plans: Free, Pro ($12 per seat per month), Business ($24 per seat per month).
- Annual plans are billed 12 months upfront.
- Refunds: annual plans only, within 30 days of payment, never more than the invoice's unrefunded amount.

One process serves two doors onto the same in-memory records:

| Door | Default address | Auth |
|---|---|---|
| REST API | `http://localhost:4100/api` | `Authorization: Bearer demo-token` |
| MCP server (Streamable HTTP) | `http://localhost:4200/mcp` | `Authorization: Bearer demo-mcp-token` |

A refund made through MCP shows up in the REST API and the other way round.
Data resets every time the process starts.

The demo is its own Go module, so it is never built, linted or tested with the
product, and nothing in the product imports it.

## Run

From the repository root:

```sh
go run -C examples/actions-demo . -token demo-token -mcp-token demo-mcp-token
```

Or from this folder: `go run . -token demo-token -mcp-token demo-mcp-token`.

| Flag | Default | Meaning |
|---|---|---|
| `-addr` | `:4100` | REST API address |
| `-mcp-addr` | `:4200` | MCP server address |
| `-token` | `$LOVELACE_API_TOKEN`, else `demo-token` | REST bearer token |
| `-mcp-token` | `$LOVELACE_MCP_TOKEN`, else `demo-mcp-token` | MCP bearer token |

Every REST request and MCP tool call prints one line, so you can watch what an agent did:

```
10:42:07  REST  POST /api/refunds        customer=user_42   idempotency_key=k1  -> 201 refunded $144.00 on invoice inv_1001 as re_0001
10:42:09  REST  POST /api/refunds        customer=user_42   idempotency_key=k1  -> 201 replayed refund re_0001 of $144.00 on invoice inv_1001; no new money moved
10:42:15  MCP   tool issue_refund        customer=user_7    idempotency_key=-   -> error: invoice inv_1002 is for a monthly plan; only annual plans can be refunded
```

Test it with `go test ./...` from this folder.

When dimmi runs in Docker, reach the demo on the host as `host.docker.internal`.

## Seeded customers

Dates are relative to when the process starts.

| Customer | Who | Subscription | Invoices | Orders | Refundable? |
|---|---|---|---|---|---|
| `user_42` | Ada Lovelace | Pro, annual, 1 seat, $144 paid 10 days ago | `inv_1001` ($144, 10 days ago), `inv_0901` ($144, a year earlier) | `ord_5001` | `inv_1001` yes |
| `user_7` | Grace Hopper | Pro, monthly, 3 seats, $36 paid 5 days ago | `inv_1002`, `inv_0977` ($36 each) | `ord_5002` | No: monthly |
| `user_99` | Katherine Johnson | Business, annual, 5 seats, $1,440 paid 45 days ago | `inv_0950` ($1,440) | `ord_5003` | No: past 30 days |

## REST API

Amounts are US dollars. Times are RFC 3339 in UTC.

Failures answer `{"error": {"code": "...", "message": "..."}}` with:

- `400` for a bad request.
- `401` for a missing or wrong token.
- `404` for an unknown customer, invoice or order.
- `409` for an idempotency key reused with a different body.
- `422` for a refund the policy refuses. The message says why.

| Method and path | Does |
|---|---|
| `GET /api/customers/{external_id}/subscription` | Plan, seats, billing interval, amount, `paid_at`, `renews_at` |
| `GET /api/customers/{external_id}/invoices` | `{customer_id, invoices: [...]}`, newest first, with `amount_refunded` and `status` |
| `GET /api/orders/{id}?customer={external_id}` | One order; `404` if it belongs to someone else |
| `POST /api/refunds` | Body `{customer, invoice_id, amount, reason}`. Honors `Idempotency-Key` (see below). `201` with the refund |
| `POST /api/customers/{external_id}/password-reset` | `{ok: true, ...}`. Sends nothing |
| `POST /api/subscriptions/{external_id}/plan` | Body `{plan}`: `free`, `pro` or `business`. Returns the subscription |

Refund idempotency:

- The first request under an `Idempotency-Key` decides the outcome, refund or refusal.
- A retry with the same key and body returns that same outcome. It never refunds twice.
- A replayed success has `"replayed": true` and the header `Idempotent-Replayed: true`.
- The same key with a different body is refused with `409`.
- Without a key, a repeat still fails once nothing is left to refund.

```sh
curl -s -H 'Authorization: Bearer demo-token' localhost:4100/api/customers/user_42/subscription

curl -s -H 'Authorization: Bearer demo-token' -H 'Idempotency-Key: k1' \
  -d '{"customer":"user_42","invoice_id":"inv_1001","amount":144,"reason":"bought by mistake"}' \
  localhost:4100/api/refunds
```

## MCP tools

Each tool answers with `structuredContent` and a one-sentence text summary.
A policy refusal is a tool error (`isError: true`) whose text is the reason.

| Tool | Arguments | Annotations | Does |
|---|---|---|---|
| `get_subscription` | `customer_id` | read-only, idempotent | Same as the subscription endpoint |
| `list_invoices` | `customer_id` | read-only, idempotent | Same as the invoices endpoint |
| `get_order` | `customer_id`, `order_id` | read-only, idempotent | Same as the order endpoint |
| `issue_refund` | `customer_id`, `invoice_id`, `amount`, `reason`, optional `idempotency_key` | `readOnlyHint: false`, `destructiveHint: true` | Same rules as `POST /api/refunds` |
| `change_plan` | `customer_id`, `plan` (enum) | destructive, idempotent | Same as the plan endpoint |
| `send_password_reset` | `customer_id` | not destructive | Same as the password reset endpoint |

`issue_refund` takes its idempotency key from `idempotency_key`.
Without it, the `Idempotency-Key` header of the HTTP request carrying the call is used.
