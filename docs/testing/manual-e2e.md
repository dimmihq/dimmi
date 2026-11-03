# Manual end-to-end check

The integration tests in `tests/feature` run the real application in process. This check runs the release binary
the way a self-hoster does: built with `go build`, configured only by environment variables, migrated with
`dimmi migrate`, served with `dimmi serve`, and driven with `curl` over a real socket. Run it after every
milestone that adds or changes endpoints, and add the new flows to the list below in the same change.

It catches what in-process tests cannot: configuration read from the environment, CLI commands, cookies over
real HTTP, mail leaving through a real SMTP server, process start and graceful stop, and state surviving a restart.

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
DIMMI_AUTH__OAUTH__GITHUB__CLIENT_ID=e2e-github-client
DIMMI_AUTH__OAUTH__GITHUB__CLIENT_SECRET=e2e-github-secret
DIMMI_MAIL__MAILERS__SYSTEM__DRIVER=smtp
DIMMI_MAIL__MAILERS__SYSTEM__HOST=localhost
DIMMI_MAIL__MAILERS__SYSTEM__PORT=11025
DIMMI_MAIL__MAILERS__SYSTEM__ALLOW_INSECURE=true
EOF
```

`env -i` below starts the binary with nothing but this file, so a setting the binary needs and the file lacks shows
up as a failure instead of leaking in from your shell. The bare `DATABASE_URL`, `REDIS_URL` and `PORT` names
platforms inject are not read yet; use the `DIMMI_*` names. The mail lines send every mail through Mailpit's SMTP
port (`smtp://localhost:11025`); section 5 runs without them to check the log fallback. The GitHub client is not a
real OAuth app: it turns the provider on, so the redirect and callback checks below run, but no one can finish a
sign-in at github.com with it. The integration tests cover finished sign-ins against a local identity provider.

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
track of. `serve` runs the queue worker too, which is what sends mail.

## 3. Drive the API

A helper that prints the status, the cookie and `Retry-After` headers (cookie values cut short), and the start of
the body:

```bash
B=http://127.0.0.1:18080/api/v1; O='Origin: http://localhost:18080'; J='Content-Type: application/json'
r(){ echo "== $1"; shift; curl -s -o body -D hdr -w '%{http_code}\n' "$@"
     grep -iE '^(set-cookie|retry-after):' hdr | sed -E 's/=([^;]{6})[^;]*/=\1…/'; head -c 400 body; echo; }
```

Each browser is a cookie jar (`-b a.jar -c a.jar`). Send the `Origin` header on every cookie request, as a browser
does. Mailpit keeps every mail from earlier runs, so this run uses addresses of its own:

```bash
RUN=$(date +%s); ADA=ada.$RUN@example.com; BOB=bob.$RUN@example.com; CAROL=carol.$RUN@example.com
DAVE=dave.$RUN@example.com
```

| Flow | Command | Expect |
|---|---|---|
| Server facts | `r config $B/auth/config` | 200, `signup_mode`, `password_min_length`, factors, `mail_configured: true` |
| Common password | `r weak -X POST -H "$O" -H "$J" $B/users -d "{\"name\":\"Ada\",\"email\":\"$ADA\",\"password\":\"password123\"}"` | 422, message "The password is too common. Choose another one." |
| Sign up | `r signup -c a.jar -b a.jar -X POST -H "$O" -H "$J" $B/users -d "{\"name\":\"Ada Lovelace\",\"email\":\"$ADA\",\"password\":\"analytical-engine-1843\",\"workspace_name\":\"Acme\"}"` | 201, `dimmi_session` and `dimmi_device` cookies, HttpOnly, SameSite=Lax, `email_verified: false` |
| Email taken, any case | same body with the email in capitals | 422 `email_taken` |
| Who am I | `r me -b a.jar -H "$O" $B/me` | 200, user, `auth.auth_methods` `["password"]` |
| Name a workspace | add `-H 'Dimmi-Workspace: ws_…'` (id from sign-up) | 200, `effective_abilities` filled |
| Cross-origin refused | `r xo -b a.jar -X DELETE -H 'Origin: https://evil.example' $B/sessions/current` | 403 `cross_origin_request` |
| Sign out | `r out -b a.jar -c a.jar -X DELETE -H "$O" $B/sessions/current` then `me` | 204 with the cookie cleared, then 401 |
| Lockout | five wrong passwords on `POST $B/sessions` (`{"grant":"password",...}`), then the right one | 422 ×5, then 429 `too_many_attempts`, `Retry-After: 30` |
| Lock ends | wait 31 s, right password with `-c a.jar -b a.jar` | 201 |
| Same answer for strangers | wrong password for an unknown email | 422 `invalid_credentials`, timing close to a real account's (`-w '%{time_total}'`) |
| Two browsers | sign in with jar `c.jar`, then `GET $B/sessions` from `a.jar` | two sessions, one `current: true` |
| Revoke one | `DELETE $B/sessions/ses_…` (the other one) from `a.jar`, then `me` with `c.jar` | 204, then 401 |
| Sign out others | sign in with `d.jar`, `DELETE $B/sessions` from `a.jar`, then `me` with `d.jar` and `a.jar` | 204, then 401 for d, 200 for a |
| Bad JSON | `POST $B/sessions -d '{"grant':` | 400 `bad_request` |
| OpenAPI | `curl -s -o /dev/null -w '%{http_code}' $B/openapi.json` | 200 |

### Mail flows

Mail leaves through Mailpit, and the API at `http://localhost:18025/api/v1` reads it back, the newest first. Several
mails to one address can arrive in either order (a sign-in mail and a new-device mail, say), so the helpers pick
the newest mail to an address whose subject has a word in it, print its text body, and pull a link or a code out:

```bash
M=http://localhost:18025/api/v1
mail(){ sleep 1; id=$(curl -s "$M/search?query=to:$1%20subject:$2&limit=1" | jq -r '.messages[0].ID')
        curl -s "$M/message/$id" | jq -r '"Subject: " + .Subject + "\nFrom: " + .From.Address, .Text'; }
link(){ mail "$1" "$2" | grep -oE 'http://[^ >]+#[^ >]+' | head -1; }
code(){ mail "$1" "$2" | grep -oE '[0-9]{3}-[0-9]{3}' | head -1; }
frag(){ python3 -c 'import sys,urllib.parse as u; print(u.parse_qs(u.urlparse(sys.argv[1]).fragment)[sys.argv[2]][0])' "$1" "$2"; }
```

| Flow | Command | Expect |
|---|---|---|
| Verification mail | `mail $ADA Verify` | "Verify your email address" from `no-reply@localhost`, link `http://localhost:18080/verify-email#id=ott_…&token=…`; the token only after the `#`, never in the subject |
| Verify, any device | `L=$(link $ADA Verify); r verify -X PATCH -H "$O" -H "$J" $B/email-verifications/$(frag $L id) -d "{\"token\":\"$(frag $L token)\"}"` (no cookie jar) | 200, `object: email_verification`, `status: verified`, `user_id: usr_…` |
| Second click | same command again | 200, same body (idempotent) |
| Wrong token | same id, `{"token":"nope"}` | 422 `invalid_verification_token` |
| Already verified | `r resend -b a.jar -X POST -H "$O" $B/email-verifications` | 200 "Your email address is already verified." |
| Resend, once a minute | sign up `$BOB` with jar `b.jar`, then `POST $B/email-verifications` twice with `b.jar` | 202, then 429 `too_many_attempts` "Too many attempts. Try again in 60 seconds." |
| Reset asked | `r reset -X POST -H "$O" -H "$J" $B/password-resets -d "{\"email\":\"$ADA\"}"`, then the same for `nobody.$RUN@example.com` | 202 twice, identical bodies; one mail "Reset your Dimmi password" to Ada with `/reset-password#id=prs_…&token=…` |
| Reset wrong token | `L=$(link $ADA Reset); r bad -X PATCH -H "$O" -H "$J" $B/password-resets/$(frag $L id) -d '{"token":"nope","password":"difference-engine-1822"}'`, and the same with id `prs_01jc0000000000000000000000` | 422 `invalid_reset_token` both, identical bodies |
| Reset | `r redeem -c r.jar -b r.jar -X PATCH -H "$O" -H "$J" $B/password-resets/$(frag $L id) -d "{\"token\":\"$(frag $L token)\",\"password\":\"difference-engine-1822\"}"` | 200 `me`, session cookie, `auth.auth_methods` `["password_reset"]`, `email_verified: true`; `me` with `a.jar` 401; mail "Your Dimmi password changed" |
| New password | `POST $B/sessions` with `difference-engine-1822` and jar `c2.jar` | 201; the old password 422 |
| Magic link asked | `r magic -X POST -H "$O" -H "$J" $B/magic-links -d "{\"email\":\"$ADA\"}"` | 202 "If this email can sign in, a link is on its way."; mail "Your Dimmi sign-in link" with `/login/magic#token=…`, a `123-456` code, the requesting IP and "an unknown device" for curl |
| Scanner opens the link | `curl -s -o /dev/null -w '%{http_code}\n' "$(link $ADA link)"` | 404 (no SPA yet); the link still works next |
| Link signs in | `r ml -c m.jar -b m.jar -X POST -H "$O" -H "$J" $B/sessions -d "{\"grant\":\"magic_link\",\"token\":\"$(frag $(link $ADA link) token)\"}"` | 201, `auth_methods: ["magic_link"]` |
| Link works once | same command again | 410 `magic_link_expired` |
| Wrong code | ask again, then `-d "{\"grant\":\"email_code\",\"email\":\"$ADA\",\"code\":\"000-000\"}"` | 422 `invalid_code` |
| Code signs in | `-d "{\"grant\":\"email_code\",\"email\":\"$ADA\",\"code\":\"$(code $ADA link)\"}"` with jar `e.jar` | 201, `auth_methods: ["email_code"]` |
| Sign up by link | `POST $B/magic-links` with `{"email":"$CAROL","name":"Carol Shaw"}`, then its link with jar `s2.jar` | 201, `name: Carol Shaw`, `email_verified: true`, `has_password: false` |
| Taking an account back | ask a link for `$DAVE`, sign up `$DAVE` with a password in jar `x.jar`, then use the link | 201 `email_verified: true`, `has_password: false`; `me` with `x.jar` 401; the squatter's password 422; mail "Your Dimmi password changed" saying the password was removed |
| Confirmation holds | `r conf -b m.jar -H "$O" $B/user/confirmation` | 200 `method: magic_link`, `confirmed_until` 15 min after that sign-in |
| Confirm with password | `r confirm -b c2.jar -X POST -H "$O" -H "$J" $B/user/confirmation -d '{"method":"password","password":"difference-engine-1822"}'` | 201 `method: password`, then `GET` shows the same `confirmed_until`; wrong password 422 `invalid_password`; `email_code` 422 `method_not_available` |
| No code with a password | `r codes -b c2.jar -X POST -H "$O" $B/user/confirmation-codes` | 422 `method_not_available` |
| Code for a passwordless user | `r cc -b s2.jar -X POST -H "$O" $B/user/confirmation-codes`, then `{"method":"email_code","code":"$(code $CAROL confirmation)"}` | 202 and a "Your Dimmi confirmation code" mail, then 201 `method: email_code` |
| New device | sign Ada in with a fresh jar `n.jar` | mail "New sign-in to your Dimmi account" naming "an unknown device" and the IP; signing in again from a jar that signed in before sends none |

### Identity providers

The redirect and callback are browser routes outside `/api/v1`. The helper prints `Location` cut short, so read it
whole where the table needs the query:

```bash
W=http://127.0.0.1:18080
loc(){ grep -i '^location' hdr | cut -d' ' -f2 | tr -d '\r'; }
state(){ python3 -c 'import sys,urllib.parse as u; print(u.parse_qs(u.urlparse(sys.argv[1]).query)["state"][0])' "$1"; }
```

| Flow | Command | Expect |
|---|---|---|
| Providers offered | `r config $B/auth/config; jq -c .data.providers body` | `[{"key":"github","name":"GitHub","protocol":"oauth2","redirect_url":"/auth/github/redirect"}]` |
| Provider not configured | `r google $W/auth/google/redirect` | 404 `not_found` |
| Redirect | `r gh -c o.jar -b o.jar "$W/auth/github/redirect?redirect=/inbox"; loc` | 302 to `https://github.com/login/oauth/authorize` with `code_challenge_method=S256`, `scope=read:user user:email`, `redirect_uri=http://localhost:18080/auth/github/callback`; `dimmi_oauth` cookie, HttpOnly, SameSite=Lax, `Max-Age=600`, its value not the state |
| Declined at GitHub | `r deny -b o.jar -c o.jar "$W/auth/github/callback?error=access_denied&state=$(state "$(loc)")"` (state from the redirect, run right after it) | 302 `/login?error=oauth_denied`, `dimmi_oauth` cleared |
| Callback used twice | the same command again | 302 `/login?error=oauth_state` |
| Callback from another browser | `r other "$W/auth/github/callback?code=x&state=y"` | 302 `/login?error=oauth_state` |
| Link needs a session | `r lt -H "Authorization: Bearer dmi_live_x" "$W/auth/github/redirect?intent=link"` | 401 `unauthenticated` |
| Link from a fresh session | `r link -b a.jar "$W/auth/github/redirect?intent=link"` | 302 to GitHub with a new `dimmi_oauth` cookie |
| No identities yet | `r ids -b a.jar -H "$O" $B/user/identities` | 200 `data: []` |
| Unlink unknown | `r un -b a.jar -X DELETE -H "$O" $B/user/identities/idn_01jc0000000000000000000000` | 404 `not_found` |

### Workspaces and members

Ada (`a.jar`) owns Acme from the sign-up; `WS` is its `ws_…` id, from `me`. Bob (`b.jar`) signed up above. Until
invitations exist, add Bob to Acme by hand, verified:

```bash
WS=$(curl -s -b a.jar $B/me | jq -r '.data.workspaces[0].id'); H="Dimmi-Workspace: $WS"
docker compose exec -T postgres psql -U postgres -d dimmi_e2e -c "update users set email_verified_at = now()
  where email = '$BOB'; insert into workspace_members (workspace_id, user_id, role, joined_via)
  select m.workspace_id, u.id, 'member', 'invitation' from workspace_members m join users o on o.id = m.user_id,
  users u where o.email = '$ADA' and u.email = '$BOB'"
BOBM=$(curl -s -b a.jar -H "$H" "$B/members?q=$BOB" | jq -r '.data[0].id')
```

| Flow | Command | Expect |
|---|---|---|
| Memberships | `r ms -b a.jar -H "$O" $B/user/memberships` | 200, one `membership` with `role: owner`, `joined_via: created`, `workspace.slug: acme` |
| Only workspace implied | `r ws -b a.jar -H "$O" $B/workspace` | 200 Acme, no `organization_id` |
| Several need a name | Bob's own workspace plus Acme: `r ws2 -b b.jar -H "$O" $B/workspace` | 400 `workspace_required`; with `-H "$H"` 200 Acme |
| Foreign workspace | Carol names Acme: `r fw -b s2.jar -H "$O" -H "$H" $B/workspace` | 404 `workspace_not_found` |
| Rename and move | `r upd -b a.jar -X PATCH -H "$O" -H "$J" -H "$H" $B/workspace -d '{"name":"Acme Support","slug":"acme-help","timezone":"Europe/Lisbon"}'` | 200 with the new values; `organizations.name` Acme Support |
| Old slug held | Bob in his own workspace: `PATCH $B/workspace {"slug":"acme"}` | 422 `slug_reserved`; Ada taking `acme` back: 200 |
| Bad values | `{"slug":"admin"}`, `{"slug":"Not A Slug"}`, `{"timezone":"America/Nowhere"}`, `{"test_mode":true}` | 422 `slug_reserved`, `validation_failed`, `validation_failed`, `test_mode_immutable` |
| Members | `r mem -b a.jar -H "$O" -H "$H" "$B/members?sort=-created_at"` | 200, Bob first, each with `user.name`, `user.email`; `members/me` is Ada |
| Promote | `r pr -b a.jar -X PATCH -H "$O" -H "$J" -H "$H" $B/members/$BOBM -d '{"role":"admin"}'` | 200 `role: admin`; again: 200, nothing changes |
| Self-deactivate | Bob: `PATCH $B/members/me {"status":"deactivated"}` | 422 `invalid_transition`, `from: active`, `to: deactivated` |
| Admin touches owner | Bob: `PATCH $B/members/<Ada's mem_> {"role":"admin"}` | 403 `forbidden` |
| Deactivate | Ada: `PATCH $B/members/$BOBM {"status":"deactivated"}`, then Bob `GET $B/workspace` with `-H "$H"` | 200 `status: deactivated`, then 403 `membership_deactivated` |
| Reactivate | Ada: `{"status":"active"}` | 200 `status: active`, role kept |
| Last owner | Ada: `PATCH $B/members/me {"role":"admin"}`, and `DELETE $B/user/memberships/<Ada's mem_>` | 409 `last_owner` both |
| Transfer | Ada: `PATCH $B/members/$BOBM {"role":"owner"}` | 200 `role: owner`; Ada's `members/me` now `admin` |
| Transfer needs recent sign-in | the same from a session older than 15 min (or `keep_owner` back) | 423 `password_confirmation_required` until `POST $B/user/confirmation` |
| Leave | Ada: `r lv -b a.jar -X DELETE -H "$O" $B/user/memberships/<Ada's mem_>` | 204; `user/memberships` no longer lists Acme |
| Repair | `dimmi workspace:repair acme` | `acme: defaults checked, organization in order`, exit 0; unknown slug exits 1 "Workspace not found" |
| In a browser | open `$W/d/settings/workspace`, rename and save, reload; open Members, use a member's menu | the name sticks; role, deactivate, reactivate, transfer ownership and leave each ask first and update the list |

### Browser pages

Run `make build` first so the binary embeds the browser app.

| Flow | Command | Expect |
|---|---|---|
| Sign-in pages | `for p in /login /signup /login/magic /forgot-password /reset-password /verify-email; do curl -s -o /dev/null -D - $W$p \| grep -iE '^(cache-control\|referrer-policy)'; done` | `no-store` and `no-referrer` on each |
| App page | `curl -s -D - $W/d \| grep -iE '^(cache-control\|content-security-policy)'` | `no-cache`; policy with `'nonce-…'`, no `unsafe-inline`, no dev server |
| Theme | `curl -s -b dimmi_theme=dark $W/login \| grep -o '<html[^>]*>'` | `<html lang="en" class="dark">` |
| In a browser | open `$W/signup`, create an account, open the verification mail's link, sign out, sign in by password, then by mailed code | lands on `/d` each time; provider buttons first when providers are configured; light and dark switch and survive a reload |

## 4. Check what was stored

```bash
docker compose exec -T postgres psql -U postgres -d dimmi_e2e \
  -c "select auth_method, auth_methods, confirmed_method, device_id is not null, ip, user_agent from sessions" \
  -c "select type, count(*) from security_events group by type order by type" \
  -c "select purpose, count(*), count(consumed_at) from one_time_tokens group by purpose" \
  -c "select email, created_via, email_verified_at is not null as verified, password_hash is not null as has_password from users" \
  -c "select kind, state, count(*) from river_job group by kind, state"
```

Expect one row per live session with the sign-in record filled; security events for every sign-in, failure,
lock, sign-out, revocation, `password.reset`, `magic_link.used` and `sessions.revoked_others`; one-time tokens with
a `code_hash` only for magic links and confirmation codes, used ones `consumed`; Carol `created_via magic_link`,
verified, without a password; Dave verified without a password; every `send_mail` job `completed`.

## 5. Stop, restart without a mail server, and clean up

```bash
pkill -TERM -f 'bin/dimmi serve'   # expect: "River client stopped" in serve.log, exit code 0
grep -v MAIL "$E2E/env" > "$E2E/env.nomail"
alias dimmi='env -i PATH=$PATH HOME=$HOME $(cat "$E2E/env.nomail" | xargs) /path/to/repo/bin/dimmi'
dimmi serve > serve2.log 2>&1 &     # expect: no "generated a new app key" line this time
r me -b c2.jar -H "$O" $B/me         # expect: 200, the session survived the restart
r config $B/auth/config              # expect: mail_configured: false
r reset -X POST -H "$O" -H "$J" $B/password-resets -d "{\"email\":\"$ADA\"}"
grep mail_fallback serve2.log        # expect: one WARN "mail logged instead of sent" line with subject, recipients
                                     # and the reset link in links, and no message body
pkill -TERM -f 'bin/dimmi serve'
docker compose exec -T postgres psql -U postgres -c 'DROP DATABASE dimmi_e2e'
```

## Reporting

Write down every response that differs from the table, with the command and the body. A difference is a bug in
the code or in this table; fix whichever is wrong and add an integration test for it when it is the code.
