# Manual end-to-end check

The integration tests in `tests/feature` run the real application in process. This check runs the release binary
the way a self-hoster does: built with `go build`, configured only by environment variables, migrated with
`dimmi migrate`, served with `dimmi serve`, and driven with `curl` over a real socket. Run it after every
milestone that adds or changes endpoints, and add the new flows to the list below in the same change.

It catches what in-process tests cannot: configuration read from the environment, CLI commands, cookies over
real HTTP, process start and graceful stop, and state surviving a restart.

## 1. Start the services and build

```bash
make services
go build -o bin/dimmi ./cmd/dimmi
docker compose exec -T postgres psql -U postgres -c 'DROP DATABASE IF EXISTS dimmi_e2e' -c 'CREATE DATABASE dimmi_e2e'
```

Work in a throwaway directory outside the repository; the binary writes `storage/` (the generated app key) into
the current directory.

```bash
export E2E=$(mktemp -d) && cd "$E2E"
cat > env <<'EOF'
DIMMI_DATABASE__URL=postgres://postgres:postgres@localhost:15432/dimmi_e2e?sslmode=disable
DIMMI_REDIS__URL=redis://localhost:16379/3
DIMMI_APP__URL=http://localhost:18080
DIMMI_APP__KEY_AUTO_GENERATE=true
DIMMI_HTTP__ADDR=127.0.0.1:18080
DIMMI_HTTP__ADMIN_ADDR=127.0.0.1:19090
DIMMI_AUTH__SIGNUP__MODE=open
EOF
```

`env -i` below starts the binary with nothing but this file, so a setting the binary needs and the file lacks shows
up as a failure instead of leaking in from your shell. The bare `DATABASE_URL`, `REDIS_URL` and `PORT` names
platforms inject are not read yet; use the `DIMMI_*` names.

## 2. Migrate and serve

```bash
alias dimmi='env -i PATH=$PATH HOME=$HOME $(cat "$E2E/env" | xargs) /path/to/repo/bin/dimmi'
dimmi migrate             # expect: refuses without --force, because app.env defaults to production
dimmi migrate --force
dimmi migrate:status      # expect: every migration "ran"
dimmi serve > serve.log 2>&1 &
curl -s http://127.0.0.1:18080/ready   # expect: {"status":"pass",...} with database, maintenance, valkey
```

Run `dimmi serve` in the background of your tool (an agent's background shell), not with a trailing `&` you lose
track of.

## 3. Drive the API

A helper that prints the status, the cookie and `Retry-After` headers (cookie values cut short), and the start of
the body:

```bash
B=http://127.0.0.1:18080/api/v1; O='Origin: http://localhost:18080'; J='Content-Type: application/json'
r(){ echo "== $1"; shift; curl -s -o body -D hdr -w '%{http_code}\n' "$@"
     grep -iE '^(set-cookie|retry-after):' hdr | sed -E 's/=([^;]{6})[^;]*/=\1…/'; head -c 400 body; echo; }
```

Each browser is a cookie jar (`-b a.jar -c a.jar`). Send the `Origin` header on every cookie request, as a browser
does.

| Flow | Command | Expect |
|---|---|---|
| Server facts | `r config $B/auth/config` | 200, `signup_mode`, `password_min_length`, factors |
| Common password | `r weak -X POST -H "$O" -H "$J" $B/users -d '{"name":"Ada","email":"ada@example.com","password":"password123"}'` | 422, message "The password is too common. Choose another one." |
| Sign up | `r signup -c a.jar -b a.jar -X POST -H "$O" -H "$J" $B/users -d '{"name":"Ada Lovelace","email":"Ada@Example.com","password":"analytical-engine-1843","workspace_name":"Acme"}'` | 201, `dimmi_session` and `dimmi_device` cookies, HttpOnly, SameSite=Lax |
| Email taken, any case | same body with `ada@example.com` | 422 `email_taken` |
| Who am I | `r me -b a.jar -H "$O" $B/me` | 200, user, `auth.auth_methods` `["password"]` |
| Name a workspace | add `-H 'Dimmi-Workspace: ws_…'` (id from sign-up) | 200, `effective_abilities` filled |
| Cross-origin refused | `r xo -b a.jar -X DELETE -H 'Origin: https://evil.example' $B/sessions/current` | 403 `cross_origin_request` |
| Sign out | `r out -b a.jar -c a.jar -X DELETE -H "$O" $B/sessions/current` then `me` | 204 with the cookie cleared, then 401 |
| Lockout | five wrong passwords on `POST $B/sessions` (`{"grant":"password",...}`), then the right one | 422 ×5, then 429 `too_many_attempts`, `Retry-After: 30` |
| Lock ends | wait 31 s, right password | 201 |
| Same answer for strangers | wrong password for an unknown email | 422 `invalid_credentials`, timing close to a real account's (`-w '%{time_total}'`) |
| Two browsers | sign in with jar `c.jar`, then `GET $B/sessions` from `a.jar` | two sessions, one `current: true` |
| Revoke one | `DELETE $B/sessions/ses_…` (the other one) from `a.jar`, then `me` with `c.jar` | 204, then 401 |
| Sign out others | sign in with `d.jar`, `DELETE $B/sessions` from `a.jar`, then `me` with `d.jar` and `a.jar` | 204, then 401 for d, 200 for a |
| Bad JSON | `POST $B/sessions -d '{"grant":'` | 400 `bad_request` |
| OpenAPI | `curl -s -o /dev/null -w '%{http_code}' $B/openapi.json` | 200 |

## 4. Check what was stored

```bash
docker compose exec -T postgres psql -U postgres -d dimmi_e2e \
  -c "select auth_method, auth_methods, confirmed_method, device_id is not null, ip, user_agent from sessions" \
  -c "select type, count(*) from security_events group by type order by type"
```

Expect one row per live session with the sign-in record filled, and security events for every sign-in, failure,
lock, sign-out and revocation above.

## 5. Stop, restart, and clean up

```bash
pkill -TERM -f 'bin/dimmi serve'   # expect: "River client stopped" in serve.log, exit code 0
dimmi serve > serve2.log 2>&1 &     # expect: no "generated a new app key" line this time
r me -b a.jar -H "$O" $B/me          # expect: 200, the session survived the restart
pkill -TERM -f 'bin/dimmi serve'
docker compose exec -T postgres psql -U postgres -c 'DROP DATABASE dimmi_e2e'
```

## Reporting

Write down every response that differs from the table, with the command and the body. A difference is a bug in
the code or in this table; fix whichever is wrong and add an integration test for it when it is the code.
