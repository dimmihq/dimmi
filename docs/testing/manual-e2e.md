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
| Server facts | `r config $B/auth/config` | 200, `signup_mode`, `password_min_length`, factors, `mail_configured: true`, `locales` `[{"code":"en","name":"English"}]` |
| Common password | `r weak -X POST -H "$O" -H "$J" $B/users -d "{\"name\":\"Ada\",\"email\":\"$ADA\",\"password\":\"password123\"}"` | 422, message "The password is too common. Choose another one." |
| Sign up | `r signup -c a.jar -b a.jar -X POST -H "$O" -H "$J" $B/users -d "{\"name\":\"Ada Lovelace\",\"email\":\"$ADA\",\"password\":\"analytical-engine-1843\",\"workspace_name\":\"Acme\"}"` | 201, `dimmi_session` and `dimmi_device` cookies, HttpOnly, SameSite=Lax, `email_verified: false` |
| Another spelling | sign up again with `$ADA` in fullwidth letters or its domain in Unicode or punycode | 422 `email_taken`; an address with a zero-width space 422 on `email` |
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
| Verify, another device | `L=$(link $ADA Verify); r verify -X PATCH -H "$O" -H "$J" $B/email-verifications/$(frag $L id) -d "{\"token\":\"$(frag $L token)\"}"` (no cookie jar) | 409 `verification_needs_choice`; the link stays good, nothing changes |
| Verify, signed-in browser | same, with `-b a.jar` | 200, `object: email_verification`, `status: verified`, `user_id: usr_…`; password and session stay |
| Take back an address | sign up `$MAL` with jar `m.jar`, then the mailed link with no jar and `{"token":"…","claim":true}` | 200; `GET /me` with `m.jar` 401, `m.jar`'s password 422 `invalid_credentials`, providers, factors and tokens gone, its workspace in `GET /user/memberships?filter[trashed]=only` after signing in by magic link |
| Second click | the signed-in command again, or with no cookie jar | 200, same body (idempotent) |
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

Ada (`a.jar`) owns Acme from the sign-up; `WS` is its `ws_…` id, from `me`. Bob (`b.jar`) signed up above and
joins Acme by invitation; Eve signs up through one:

```bash
WS=$(curl -s -b a.jar $B/me | jq -r '.data.workspaces[0].id'); H="Dimmi-Workspace: $WS"; EVE=eve.$RUN@example.com
inv(){ r inv -b a.jar -X POST -H "$O" -H "$J" -H "$H" $B/invitations -d "{\"invitations\":[$1]}"; }
```

| Flow | Command | Expect |
|---|---|---|
| Invite a batch | `inv "{\"email\":\"$BOB\",\"role\":\"member\"},{\"email\":\"$ADA\",\"role\":\"member\"},{\"email\":\"nope\",\"role\":\"member\"}"` | 200, per address `created` (with `accept_url: null`, `invited_by.name: Ada Lovelace`), `already_member`, `invalid` with `error.code: invalid_email`; mail "Ada Lovelace invited you to join Acme on dimmi" to Bob, link `/invitation#token=…` |
| Invite again | the first address again | 200 `resent`, `send_count: 2`; one pending row; the first mail's link now 404 on preview |
| Preview | `T=$(frag $(link $BOB invited) token); r pv -H "Dimmi-Invitation-Token: $T" $B/invitations/preview` | 200 `invitation_preview`, `workspace.name: Acme`, `inviter.name`, `email_masked: b***@example.com`, `has_account: true`, `status: pending`, `Cache-Control: no-store`; a made-up token 404 |
| Someone else accepts | `r acc -b s2.jar -X POST -H "$O" -H "$J" $B/user/memberships -d "{\"invitation_token\":\"$T\"}"` | 403 `invitation_email_mismatch`; without a cookie 401 |
| Accept | the same with `b.jar` | 201 `membership`, `role: member`, `joined_via: invitation`; Bob's email verified; mail "Bob joined Acme" to Ada; again: 409 `invitation_already_accepted` |
| Send again, withdraw | `inv "{\"email\":\"$DAVE\",\"role\":\"admin\"}"; INV=$(jq -r '.data[0].invitation.id' body)`, `POST $B/invitations/$INV/deliveries` four times, then `DELETE $B/invitations/$INV` twice | 201 `invitation_delivery` three times (`send_count` 2 to 4, `expires_at` a week on), then 429 `too_many_attempts`; 204, then 422 `invalid_transition` `from: revoked`; the link's preview `status: revoked`, accept 410 `invitation_revoked` |
| List | `r li -b a.jar -H "$O" -H "$H" -g "$B/invitations?filter[status]=accepted,revoked"` | 200, Dave `revoked` first, then Bob `accepted`; `GET $B/invitations` alone lists only pending ones |
| Sign up by invitation | `inv "{\"email\":\"$EVE\",\"role\":\"member\"}"`, then `r eve -c i.jar -b i.jar -X POST -H "$O" -H "$J" $B/users -d "{\"name\":\"Eve\",\"email\":\"$EVE\",\"password\":\"analytical-engine-1843\",\"invitation_token\":\"$(frag $(link $EVE invited) token)\"}"` | 201, `email_verified: true`, `workspace.name: Acme`, `member.role: member`; no verification mail; another email with the token 403 `invitation_email_mismatch` |

```bash
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
| Members | `r mem -b a.jar -H "$O" -H "$H" "$B/members?sort=-created_at"` | 200, Eve then Bob first, each with `user.name`, `user.email`; `members/me` is Ada |
| Review members | Ada: `r rv -b a.jar -H "$O" -H "$H" "$B/members?filter[two_factor]=false&sort=-last_active_at"`; the same as Bob while he is a member | 200 with `two_factor_enabled` and `last_active_at` on each; Bob: 403 `forbidden`, and his plain list has both fields `null` |
| Export members | Ada: `curl -s -b a.jar -H "$O" -H "$H" -H "Accept: text/csv" "$B/members?filter[role]=member"` | `text/csv`, header `name,email,role,custom_role,status,teams,two_factor,last_active_at,joined_at`; a name starting with `=` comes out with a leading `'`; the audit log gains `members.exported` |
| Import members | Ada: `curl -s -b a.jar -H "$O" -H "$H" -F file=@members.csv -F dry_run=true $B/member-imports` with a file `email,role` holding Bob as admin, a new address and `not-an-address`; then without `dry_run` | 200 `member_import`, `dry_run: true`, `updated: 1`, `created: 1`, `failed: 1` with `invalid_email`, nothing changed; the real run makes Bob admin and invites the new address; a sixth import within the hour 429 `too_many_imports`; Bob: 403 |
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
| Delete is for owners | Eve: `r del -b i.jar -X DELETE -H "$O" -H "$J" -H "$H" $B/workspace -d '{"confirm_slug":"acme"}'` | 403 `forbidden` |
| Delete needs the address | Bob (owner since the transfer): the same with `b.jar` and `{"confirm_slug":"wrong"}` | 422 `confirm_slug_mismatch` on `confirm_slug` (423 `password_confirmation_required` first when Bob signed in over 15 min ago: confirm with `POST $B/user/confirmation`) |
| Delete | Bob: `{"confirm_slug":"acme"}` | 200 with `deleted_at` and `purge_after` 30 days later; `GET $B/workspace` with `-H "$H"` 404 `workspace_not_found`; mail "Acme was deleted" to Bob; `user/memberships` hides Acme, `curl -g ...?filter[trashed]=only` shows it to Bob |
| Restore | Bob: `r rs -b b.jar -X PATCH -H "$O" -H "$J" $B/workspaces/$WS -d '{"trashed":false}'` | 200 `deleted_at: null`; mail "Acme was restored"; Eve the same 404 `workspace_not_found`; `{"trashed":false,"name":"x"}` 422 `prohibited` on `name` |
| In a browser | open `$W/d/settings/workspace`, rename and save, reload; open Members, use a member's menu | the name sticks; role, deactivate, reactivate, transfer ownership and leave each ask first and update the list |
| Invite in a browser | as an owner open Members, Invite, enter an address and send; open the mailed link in a private window and sign up there; back in Members, resend and revoke another pending invitation | the invitee lands in the workspace's inbox and shows in Members; the pending list updates; the revoked link says it was withdrawn |
| Hand over a link | invite with a team ticked, then "Copy a new link" in its menu and confirm; open the old mailed link, then the new one | the old link no longer works; the new one signs the invitee up straight into the team |
| Import in a browser | Members, Import, pick a CSV | the dialog says what the file would do and lists rows with problems; Apply changes updates the list and the pending invitations |
| Review in a browser | with a teammate lacking a second factor, open Workspace settings, follow "See who", then "Export CSV" | the count line names them, the list shows only them with "No two-factor", and the file downloads |
| Delete in a browser | as an owner open Workspace, Danger zone, Delete workspace, type the address | the button stays off until the address matches; the workspace disappears and the app opens another one |

### Teams

Bob (`b.jar`) owns Acme after the transfer, and Ada's membership is deactivated since she left:

```bash
ADAM=$(curl -s -b b.jar -H "$H" "$B/members?q=$ADA" | jq -r '.data[0].id')
```

| Flow | Command | Expect |
|---|---|---|
| Create | `r tc -b b.jar -X POST -H "$O" -H "$J" -H "$H" $B/teams -d '{"name":" Support "}'; TEAM=$(jq -r .data.id body)` | 201 `object: team`, `name: Support`, `member_count: 0` |
| Same name, any case | the same with `{"name":"SUPPORT"}` | 422 `team_name_taken` on `name` |
| Add members | `r add -b b.jar -X POST -H "$O" -H "$J" -H "$H" $B/teams/$TEAM/members -d "{\"member_ids\":[\"$BOBM\",\"$ADAM\",\"mem_nope\",\"$BOBM\"]}"` | 200, results in order: `added`, `inactive`, `not_found`, `already_member` |
| List | `r tl -b b.jar -H "$O" -H "$H" "$B/teams?include=members"` | 200, Support with `member_count: 1` and Bob under `members`; without `include` no `members` key |
| Member's teams | `r mt -b b.jar -H "$O" -H "$H" "$B/members/me?include=teams"` | 200, `teams` `[{"id":"team_…","object":"team","name":"Support"}]` |
| Rename | `PATCH $B/teams/$TEAM {"name":"Customer care"}` | 200 `name: Customer care` |
| Unknown team | `r ut -b b.jar -H "$O" -H "$H" $B/teams/team_01jc0000000000000000000000` | 404 `not_found` |
| Remove member | `r rm -b b.jar -X DELETE -H "$O" -H "$H" $B/teams/$TEAM/members/$BOBM`, twice | 204, then 404 |
| Count before delete | `r cc -b b.jar -H "$O" -H "$H" "$B/teams/$TEAM?include=conversation_counts"` | 200 with `conversation_counts` `{open, snoozed, closed}` |
| Delete with conversations | with a conversation assigned to the team: `DELETE $B/teams/$TEAM?reassign_to_team_id=$TEAM`, then `?reassign_to_team_id=<other team>` | 422 `reassign_to_invalid` (a team cannot take its own), then 204 and the conversation sits with the other team; with no target the conversations lose their team |
| Delete | `r del -b b.jar -X DELETE -H "$O" -H "$H" $B/teams/$TEAM`, then `GET $B/teams/$TEAM` | 204, then 404; `teams` lists nothing; the row stays with `deleted_at` set for the inbox to sweep |
| Name free again | `POST $B/teams {"name":"Customer care"}` | 201 |
| In a browser | open `$W/d/settings/teams`, create a team, add a member from the picker, rename it, remove the member, go back to all teams and delete it | the team page opens on create; the member count follows each change; delete asks first and returns to the list |

### Audit and security logs

Run right after the members flows: Bob now owns Acme and Ada has left it. `ADAM` is Ada's `mem_` id. Filters use
square brackets, so pass `-g` to stop curl from reading them as a glob.

| Flow | Command | Expect |
|---|---|---|
| Whole log, oldest first | `r log -g -b b.jar -H "$O" -H "$H" "$B/audit-logs?sort=created_at&limit=100"; jq -r '.data[] \| [.event, .actor.via.kind, .subject.label] \| @tsv' body` | 200: `workspace.created`, `member.joined` (no `via`: signing up), `workspace.updated`, `member.role_changed`, `member.deactivated`, `member.reactivated`, `member.role_changed`, `ownership.transferred`, `member.role_changed` (Ada to admin), `member.left`; each with `actor.label` "Ada Lovelace <ada…>", `via.kind: session`, `ip`, `user_agent: curl/…`, `request_id`, `changes` such as `{"role":{"from":"member","to":"admin"}}`; between them `member.signed_in` for every sign-in of a member, actor the member with no `via` |
| By event | `"$B/audit-logs?filter[event]=member.role_changed,ownership.transferred"` | 4 entries, newest first |
| By actor, paged | `"$B/audit-logs?filter[actor]=$ADAM&limit=2"`, then again with `&cursor=` the `meta.next_cursor` | 2 entries and a cursor, then the next 2 older ones |
| By time | `filter[created_after]=2026-01-01T00:00:00Z&filter[created_before]=2099-01-01T00:00:00Z`; then `filter[created_after]=yesterday` | every entry; then 400 `invalid_filter`, `parameter: created_after` |
| One entry | `r show -b b.jar -H "$O" -H "$H" $B/audit-logs/aud_…`, and `aud_01jc0000000000000000000000` | 200 the entry; 404 `not_found` |
| Members cannot read | before the promotion above, Bob (then a member): `GET $B/audit-logs` with `-H "$H"` | 403 `forbidden` |
| Own security log | `r sev -b a.jar -H "$O" "$B/user/security-events?limit=2"` | 200, newest first: `login.succeeded` with `metadata.auth_methods`, `session_id`, no `device_id`; `meta.next_cursor` while more remain |
| Tokens refused | `r sevt -H "Authorization: Bearer dmi_live_x" $B/user/security-events` | 401 `invalid_token` |
| Colleague sign-ins | `r bad -X POST -H "$O" -H "$J" $B/sessions -d '{"grant":"password","email":"'$BOB'","password":"wrong"}'`, the same for `nobody@example.com`; a second later `GET $B/audit-logs?filter[event]=member.sign_in_failed` as Bob | 422 twice; one entry about Bob, actor `system` "Unknown", `changes` with `ip`, `device_label`, `method: password`, `reason: invalid_credentials`, nothing for the unknown address |
| Second factor changes | Bob turns on an authenticator, then removes it; `GET $B/audit-logs?filter[subject]=$BOBM` | `member.two_factor_enabled`, `member.auth_factor_added`, `member.auth_factor_removed`, `member.two_factor_disabled`; `changes.factor_type: totp`, never a code |
| CSV export | `r csv -g -b b.jar -H "$O" -H "$H" -H 'Accept: text/csv' "$B/audit-logs?filter[event]=member.role_changed"`; `cat body` | 200, `Content-Type: text/csv; charset=utf-8`, `Content-Disposition: attachment; filename="dimmi-audit-acme-<30 days ago>-<today>.csv"`; header `id,created_at,event,actor_type,actor_id,actor_label,via_kind,via_id,subject_type,subject_id,subject_label,changes,ip,user_agent,request_id`, one row per role change, times in UTC, `changes` compact JSON |
| Export with a range | `?format=csv&filter[created_after]=2026-01-01T00:00:00Z` and `filter[created_after]=2020-01-01T00:00:00Z` | 200 for the first; 422 `range_too_large`, `max_days: 366` for the second |
| Export leaves an entry | `GET $B/audit-logs?filter[event]=audit_logs.exported` | one entry per export: `changes.filters` as asked plus `created_after`/`created_before`, `rows`, `completed: true` |
| Export limit | eleven exports within the hour | the eleventh: 429 `too_many_exports` with `retry_after` |
| Formula cells | rename a team to `=1+1`, export `filter[event]=team.updated` | the `subject_label` cell reads `'=1+1` |
| Entries cannot change | as the application's database role (not a superuser): `update audit_logs set event = 'x.y'` | `permission denied for table audit_logs` |
| In a browser | as Bob, open `$W/d/settings/audit-log`, pick "Changed a role" under events, then Ada under actors; open `$W/d/settings/security` | the table narrows to each filter, Older and Newer step through pages; "Recent security activity" lists the latest sign-ins with device and address |
| Date range and export in a browser | as Bob, pick "Last 7 days" in the date picker, "Any sign-in event" under events, open a row, then "Export CSV" | the events list is grouped (Sign-in, Security settings, Members and teams, Tokens and apps, Data export); the row opens with its IP address, browser and changes; the file downloads as `dimmi-audit-acme-….csv` with only the rows on screen's filters |

Retention: `audit.retention_days` (`DIMMI_AUDIT__RETENTION_DAYS`) keeps audit entries for ever on self-host
unless set, a year on the cloud; `security_events.retention_days` is 365 on both. The prune jobs run nightly at
03:10 and 03:20 UTC.

### Shared inbox

Ada (`a.jar`) in Acme, `H` as above, Bob a member. Ada adds an inbox, a customer writes through the API channel,
and the team works the conversation:

```bash
IN=$(curl -s -b a.jar -H "$O" -H "$J" -H "$H" -X POST $B/inboxes -d '{"name":"Support"}' | jq -r .data.id)
new(){ curl -s -b a.jar -H "$O" -H "$J" -H "$H" -X POST $B/conversations -d "{\"inbox_id\":\"$IN\",\"author\":\"contact\",
  \"subject\":\"$1\",\"body\":\"$2\",\"contact\":{\"email\":\"$3\",\"name\":\"$4\"}}" | jq -r .data.id; }
C=$(new 'Card declined' 'I want a refund for the duplicate charge' ann.$RUN@example.com Ann)
```

| Flow | Command | Expect |
|---|---|---|
| Inboxes | `r ib -b a.jar -H "$O" -H "$H" $B/inboxes` | 200, Support; Bob `POST $B/inboxes` 403 `forbidden` |
| Inbox access | `POST $B/inboxes {"name":"IT"}` as `IT`, a conversation in it; `PUT $B/inboxes/$IT/access {"mode":"restricted","member_ids":[]}`; as Bob `GET $B/inboxes`, `GET $B/conversations/{its id}`, `PATCH $B/conversations/$C {"inbox_id":"$IT"}`; then `PUT … {"mode":"restricted","member_ids":["$BOBM"]}` | 200 with `access.mode: restricted`; Bob lists only Support, the IT conversation 404, the move 422 `inbox_not_found`; once listed Bob sees both; Bob `PUT …/access` 403; the audit log has `inbox.access_changed` |
| Channels of an inbox | `POST $B/widget-settings {"name":"IT helpdesk","inbox_id":"$IT"}`; `PATCH $B/email-addresses/{address} {"inbox_id":"$IT"}`; open `$W/d/settings/inboxes/$IT` | both list under Channels with the API channel; the messenger's availability shows only the faces of those on IT's list |
| Customer starts | `r show -b a.jar -H "$O" -H "$H" $B/conversations/1` | 200 `number: 1`, `status: open`, `assignee: null`, `channel.type: api`, `unread: true`, `last_message.author.name: Ann` |
| Views | `GET $B/conversations?filter[view]=unassigned`, `…=mine`, `…=all&sort=created_at` | Ann's conversation in unassigned and all, none in mine |
| Counts | `r cnt -b a.jar -H "$O" -H "$H" $B/conversation-counts` | `views.unassigned.open: 1`, `views.unassigned.unread: 1`, `views.mine.open: 0` |
| Reply | `r rp -b a.jar -X POST -H "$O" -H "$J" -H "$H" $B/conversations/$C/replies -d '{"body":"Refunded.","client_message_id":"r1"}'` | 201, `delivery_status: not_applicable`, `meta.conversation.assignee` Ada; again 200 with `Idempotent-Replayed: true` |
| Stale reply | Bob replies with `"expected_last_message_id"` of Ann's first message after a `POST $B/conversations/$C/messages {"body":"Still waiting"}` | 409 `new_messages_since`, `data[0].excerpt` "Still waiting" |
| Note | `POST $B/conversations/$C/notes {"body":"Fraud?"}` | 201 `type: note`, `substantive: false` |
| Assign and close | `PATCH $B/conversations/$C {"assignee_id":"$BOBM","status":"closed","note":"Over to Bob"}` | 200, `meta.changed: ["assignee_id","status"]`; again `[]` |
| Snooze from closed | `PATCH $B/conversations/$C {"status":"snoozed","snooze_preset":"tomorrow"}` | 422 `invalid_transition`, `from: closed`, `to: snoozed` |
| Customer reopens | `POST $B/conversations/$C/messages {"body":"Hello?"}` | 201, `meta.conversation.status: open` |
| Timeline | `GET "$B/conversations/$C/messages?filter[type]=event"` | assigned (by the reply, then to Bob), closed, reopened (`actor.name` Ann) in order |
| Priority | `PATCH $B/conversations/$C {"priority":"urgent"}`, then `{"priority":true}`; `GET "$B/conversations?filter[view]=all&filter[priority]=high,urgent&sort=-priority"` | 200 `priority: urgent`, `meta.changed: ["priority"]`; 422; the conversation listed first |
| Team and holder views | `POST $B/teams {"name":"Billing"}`; `PATCH $B/conversations/$C {"team_id":"$TEAM"}`; `GET "$B/conversations?filter[view]=team:$TEAM"`, `…=inbox:$IN`, `…=member:$BOBM` | the conversation in the team's and inbox's views; `GET $B/conversation-counts` shows `views.teams.$TEAM.open: 1` and `views.members` per holder |
| Inbox preferences | `PATCH $B/inbox-preferences {"views":{"all":{"sort":"-created_at","status":"any"}}}`, then `GET`; `{"views":{"nowhere":{}}}` | 200 with the view kept, `updated_at` set; 422 `invalid_preferences`; a body over 64 KB 422 `preferences_too_large` |
| Viewers | Bob opens `$W/d/inbox/all/{conversation}` in a second browser; Ada `GET "$B/conversations?filter[view]=all&include=viewers"` | Bob under `viewers`; Bob's own list leaves him out; `PATCH $B/workspace {"show_viewers_on_list":false}` drops the key |
| Saved views | `POST $B/conversation-views {"name":"Refunds","scope":"personal","definition":{"op":"and","groups":[{"op":"and","conditions":[{"field":"subject","op":"contains","value":"refund"}]}]}}`; `GET "$B/conversations?filter[view]=$CVW"` | 201 `cvw_…`; the list holds only matching conversations; Bob `GET $B/conversation-views/$CVW` 404; Bob `POST` with `"scope":"shared"` 403 |
| View rules | a definition with an unknown field, 6 groups or 21 conditions; a cursor reused after `PATCH $B/conversation-views/$CVW` | 422 `invalid_definition` with `path`; 400 `cursor_mismatch` |
| Pinned view counts | `PATCH $B/inbox-preferences {"sidebar":{"pinned_views":["$CVW"]}}`; `GET $B/conversation-counts` | `views.custom.$CVW.open` set |
| Drafts | `POST "$B/drafts?upsert=true" {"conversation_id":"$C","mode":"reply","body":"Refunding","client_updated_at":"2030-01-01T00:00:00Z"}`; again with an earlier `client_updated_at`; `GET $B/drafts`; Bob `GET $B/drafts` | 201 `drf_…`; 200 `meta.stale: true` with the stored body; Ada's draft listed; Bob's list empty and his `PATCH $B/drafts/{draft}` 404 |
| Draft ends on send | `POST $B/conversations/$C/replies {"body":"Refunded."}`, then `GET "$B/drafts?filter[conversation]=$C"`; in a browser type a reply, reload, open the same conversation in a second browser | the reply draft gone, a note draft kept; the typed reply back after the reload and in the second browser |
| Formatting | in a browser open a conversation, press R, type, select a word and press Cmd+B, add a list with `- `, a link with Cmd+Shift+U; paste a web page with headings and colours | the toolbar shows each format; the sent reply's `body` is Markdown (`**…**`, `- …`, `[…](…)`) and the thread shows it formatted; pasted headings and colours arrive as plain text |
| Delete and edit | `PATCH $B/conversations/$C/messages/{note} {"body":"Fixed"}`; same on a reply; `DELETE $B/conversations/$C/messages/{note}`; Bob (member) `DELETE` on Ada's reply; `DELETE` on an event | 200 with `edited_at`; 422 `message_not_editable`; 200 `undone: false` and the timeline shows `deleted_by.name` Ada with `body: null`; 403; 422 `message_not_deletable` |
| Thread extras | in a browser add a note with a fenced ```` ```js ```` block, hover a time, press Cmd+E, open a conversation whose contact has a time zone and press the globe | the code coloured with Copy and full screen buttons; the full date in a tooltip; events folded into "N events" and still folded after a reload; times in the contact's clock |
| Seen | a widget visitor reads Ada's reply; `GET "$B/conversations/$C/messages?filter[type]=message"` | the reply's `read_at` set and the bubble says "Seen 14:02"; an email reply's `read_at` stays `null` |
| GIFs | start the server with `DIMMI_INBOX__GIF__GIPHY_API_KEY` set; `GET $B/gif-search`; `PATCH $B/workspace {"gif_picker":false}` and again; reply with `![x](https://media1.giphy.com/media/abc/giphy.gif) ![y](https://example.com/y.png)` | `enabled: true` with the key, the dashboard's policy names `https://api.giphy.com` and `https://*.giphy.com`; `enabled: false`; the sent HTML keeps the GIPHY picture, drops the other, and the message has `metadata.gif.id` `abc` |
| Emoji | in a browser type `:thu` in the composer, press Enter; add menu, Emoji, pick one | 👍 in the text; the picked emoji in the text |
| Read state | `PATCH $B/conversations/$C/read-state {}`, then `{"unread":true}` | `unread: false`, then `unread: true` |
| Search | `GET "$B/search?q=refund"`, `"$B/search?q=%231"`, `"$B/conversations?filter[view]=all&q=refunds"` | the conversation with the newest match marked in `highlight` (`<mark>Refunded</mark>`); `jump.number: 1`; one row |
| Other workspace | Carol: `GET $B/conversations/$C` with her own workspace | 404 `not_found` |
| In a browser | open `$W/d/inbox/unassigned`, open the conversation, reply, add a note, assign to me, mark as done, search | each change shows in the thread as a message or an event line; the list and counts follow |

### Customer records

Same session as the shared inbox above, `C` the conversation Ann started:

| Flow | Command | Expect |
|---|---|---|
| Tag a conversation | `r tg -b a.jar -X POST -H "$O" -H "$J" -H "$H" $B/conversations/$C/tags -d '{"tags":["Billing","billing"]}'` | 200, one tag `Billing` in `data.tags`; the timeline gains a `conversation.tagged` event |
| Tag vocabulary | `GET "$B/tags?q=bil&include=counts"` | 200, `Billing` with `counts.conversations: 1`; Bob `PATCH $B/tags/{id}` 403, `POST $B/tags {"name":"VIP"}` 201 |
| Duplicate name | `POST $B/tags {"name":"BILLING"}` | 409 `tag_exists`; `{"name":"x","color":"red"}` 422 on `color` |
| Filter by tag | `GET "$B/conversations?filter[view]=all&filter[tag]={id}"`, `…&filter[tag_any]={id},{other}` | Ann's conversation; `filter[tag]=bogus` 400 `invalid_filter` |
| Delete a tag | `DELETE $B/tags/{id}` | 204; the conversation's `tags` is empty; again 404 |
| Contact tags and notes | `PATCH $B/contacts/{ann} {"add_tags":["vip"],"notes":"Prefers email"}` | 200 with the tag and notes; `GET "$B/contacts?filter[tag]={vip}"` lists Ann only |
| Upsert a user | `POST "$B/contacts?upsert=true" {"external_id":"user_42","email":"Bob@Acme.dev"}`, then again with `"name":"Robert"` | 201 `meta.upserted: created`, `kind: user`, email lower-cased; then 200 `updated`, same id |
| Duplicates | `GET $B/contacts/{ann}/duplicates` after adding a contact with Ann's phone | the other contact with `reasons: ["same_phone"]` |
| Companies | `POST $B/companies {"name":"Acme","domain":"acme.dev"}`, link Ann with `PATCH $B/contacts/{ann} {"company_id":…}` | 201; `GET $B/companies/{id}/contacts` and `/conversations` list Ann and her conversation |
| Customer context | `r ctx -b a.jar -H "$O" -H "$H" $B/conversations/$C/context` | 200 `object: conversation_context`, Ann with notes and tags, `identities[0].type: email`, `past_conversations.total_estimate`, `duplicates_count` |
| Delete a company | Bob `DELETE $B/companies/{id}` 403; Ada again | 204; Ann's `company` is `null` |
| Customer pages | open `$W/d/customers`, search, open Ann, edit her phone; open a conversation and edit her name in the details panel | the list narrows; the profile saves; the panel shows the new name and her other conversations |
| People filters | on `$W/d/customers` pick a company, "Seen this week" and Attribute `plan` is `pro`; on Ann's page set attribute `plan` to `pro` | the list narrows to Ann; her Attributes card shows `plan` with a remove button |
| People columns | on `$W/d/customers` open Columns, tick Phone, untick Email, reload | the table keeps Phone and drops Email; `GET $B/inbox-preferences` has `views.people.columns` |
| Pinned attributes | in a conversation's details panel, star an attribute | it moves above Email for every customer; `pinned_attributes` lists it |
| Redaction defaults | `GET $B/redaction-rules` | 200 `object: redaction`, `built_ins.card_numbers: true`, `built_ins.us_ssn: false`, `rules: []`; Bob 403 |
| Redaction rule | `POST $B/redaction-rules {"name":"Accounts","pattern":"ACCT-\\d+"}`, then `{"name":"x","pattern":"("}` | 201 `object: redaction_rule`, `replacement: [redacted]`; then 422 `invalid_pattern`; an eleventh rule 422 `redaction_rule_limit_reached` |
| Redaction on write | `POST $B/conversations` as a contact with body `Card 4242 4242 4242 4242 ACCT-7` | the message body reads `Card •••• •••• •••• 4242 [redacted]`, `redactions` lists `card_numbers` and the rule's id; the database holds no original |
| Redaction switches | `PATCH $B/redaction-rules {"us_ssn":true,"order":[…]}`, then a note `SSN 123-45-6789` | 200 with the new order; the note reads `SSN •••-••-••••`; the audit log has `redaction_settings.updated` |
| Try redaction | `POST $B/redaction-tests {"text":"4242424242424242"}` | 200 `object: redaction_test`, `text: •••• •••• •••• 4242`, one `redactions` entry; nothing stored |
| Link safety settings | `PATCH $B/workspace {"link_safety":{"block_domains":["Evil.example"],"strip_inbound_email_links":true}}`, then `{"link_safety":{"block_domains":["co.uk"]}}` | 200, `link_safety.block_domains: ["evil.example"]`; then 422 `invalid_link_domains`; Bob 403 |
| Links in a thread | a customer sends `[Sign in](https://login.evil.example)` and `https://bit.ly/x`; open the conversation | the first link is struck through and cannot be opened; the second asks "Opens bit.ly. Continue?" |
| Erase a customer | `DELETE "$B/contacts/{ann}?erase=true"` | 202 `object: contact_erasure`, `status: pending`, `scheduled_for` 7 days out; Ann is gone from `GET $B/contacts` and her conversations from the lists; a reply to her is 422 `contact_unreachable_on_channel` |
| Cancel an erasure | `PATCH $B/contact-erasures/{ces} {"status":"canceled"}` | 200 `status: canceled`; Ann and her address are back; again 409 `erasure_closed` |
| Erase at once | `DELETE "$B/contacts/{ann}?erase=true&immediate=true"` more than 15 minutes after signing in, then after `POST $B/user/confirmation` | 423 `password_confirmation_required`, then 202; soon `GET $B/contact-erasures/{ces}` is `completed`, her conversation shows "Deleted contact" with empty messages |
| Export a customer | `POST $B/contacts/{ann}/exports {"include_notes":false}`, then `GET $B/contacts/{ann}/exports/{exp}` until ready, then its `url` | 202 `status: pending`, a second POST 409 `export_in_progress`; then `status: ready` with `url`; the zip holds `contact.json`, `conversations/1.json`, `conversations/1.txt` and her files, without the team's notes |
| Export from the profile | on Ann's page press Export data, then Download export | the button waits while it builds, then saves `exp_….zip` |
| Erasures page | open `$W/d/customers/erasures` as Ada, then as Bob | Ada sees the requests with Cancel on pending ones; Bob has no Erasures tab |
| Retention settings | `PATCH $B/workspace {"retention":{"closed_conversation_days":365,"export_days":14}}`, then `{"retention":{"spam_days":6}}` | 200 with `retention` showing both; then 422 `invalid_retention`; `{"closed_conversation_days":null}` keeps closed conversations again |
| Retention preview | `POST $B/retention-previews {"closed_conversation_days":30}` | 200 `object: retention_preview` with how many closed conversations have sat untouched past 30 days |
| Retention round | set a period, wait a day (the round skips a period changed in the last 24 hours), then run `apply_conversation_retention` | closed conversations untouched past the period are in the Deleted view; one with a newer note stays |
| Export conversations | `POST $B/data-exports {"kind":"conversations","format":"csv","selection":{"filter":{"view":"all","status":"any"}}}`, then `GET $B/data-exports/{dex}` until ready, then its `url` | 202 `status: pending`; then `status: ready`, `row_count`, `file_name: conversations.csv`; the file has one row per conversation Bob may see, a cell starting with `=` prefixed by `'` |
| Export transcripts and data | the same with `"format":"txt"`, then `"format":"json","include_notes":true` | several conversations download as `export.zip` with `conversation-<n>.txt` each, one as the text file itself; the JSON holds every message, notes only with `include_notes`, card numbers masked even in messages older than the rule |
| Export customers | `POST $B/data-exports {"kind":"contacts","format":"csv","selection":{"q":"ann"}}` | 202; once ready, `contacts.csv` lists only Ann |
| Export limits and rights | `"format":"txt"` over more than 1,000 conversations; a member without `conversations:export`; `"include_notes":true` as a member who may not moderate; `{"kind":"contacts","format":"txt"}` | 422 `export_too_large` with `matched` and `limit`; 403; 403; 422 `invalid_export_format` |
| Exports page | press the download icon in a view's header, pick Spreadsheet, Export, then Go to Exports; Download, then Delete | the export shows as Preparing, then Ready with its rows; the file saves as `conversations.csv`; deleting removes it from the list |
| Export expiry | set `retention.export_days` to 1, wait a day, run `expire_data_exports` | the export shows `status: expired`, no `url`, and its download answers 409 |
| Conversation safety page | open `$W/d/settings/conversation-safety`, add a rule, try a text, then open a conversation whose customer sent a card | the rule lists with its pattern; the try box shows "2 items redacted"; the message shows "1 item redacted" |

### Two-factor authentication

Ada turns on an authenticator, signs in with its code and with a recovery code, and turns it off again; then an
operator turns it off from the command line. Run these within 15 minutes of Ada's last sign-in in `a.jar`, since
adding and removing factors need a recent confirmation (423 `password_confirmation_required` otherwise; confirm with
`POST $B/user/confirmation` first). `totp` prints the code an authenticator shows for a secret, one step ahead
with a second argument of `1`, since a step already used is never accepted again:

```bash
totp(){ python3 -c 'import sys,hmac,hashlib,struct,time,base64 as b
k=b.b32decode(sys.argv[1]+"="*(-len(sys.argv[1])%8)); s=int(time.time())//30+int(sys.argv[2])
h=hmac.new(k,struct.pack(">Q",s),hashlib.sha1).digest(); o=h[-1]&15
print("%06d"%((struct.unpack(">I",h[o:o+4])[0]&0x7fffffff)%1000000))' "$1" "${2:-0}"; }
F=$B/user/auth-factors; P="{\"grant\":\"password\",\"email\":\"$ADA\",\"password\":\"difference-engine-1822\"}"
```

| Flow | Command | Expect |
|---|---|---|
| Add an authenticator | `r add -b a.jar -c a.jar -X POST -H "$O" -H "$J" $F -d '{"type":"totp"}'; FAC=$(jq -r .data.id body); SECRET=$(jq -r .data.setup.secret body)` | 201, `Cache-Control: no-store`, `status: pending`, `setup.secret` (32 base32 characters), `setup.otpauth_url` `otpauth://totp/dimmi:…&issuer=dimmi&period=30`, `setup.qr_svg` `<svg …>` |
| Wrong first code | `r wrong -b a.jar -X PATCH -H "$O" -H "$J" $F/$FAC -d '{"code":"000000"}'` | 422 `invalid_code` on `code` |
| Activate | `r on -b a.jar -c a.jar -X PATCH -H "$O" -H "$J" $F/$FAC -d "{\"code\":\"$(totp $SECRET)\"}"; RC=$(jq -r '.data.recovery_codes.codes[0]' body)` | 200 `status: active`, `preferred: true`, `recovery_codes` with 10 `codes`, `no-store`, a new `dimmi_session` cookie; mail "Two-factor authentication is on" |
| Second authenticator | `r again -b a.jar -X POST -H "$O" -H "$J" $F -d '{"type":"totp"}'` | 422 `factor_limit_reached` |
| List | `r list -b a.jar -H "$O" $F` | 200, the authenticator first, then `recovery_code` with `remaining: 10`, `total: 10`; no secrets, no codes |
| Challenge | `r pwd -c t.jar -b t.jar -X POST -H "$O" -H "$J" $B/sessions -d "$P"` | 200 `object: two_factor_challenge`, `methods` `totp` (preferred) then `recovery_code`; `dimmi_2fa` cookie, HttpOnly, `Max-Age=299`; no session cookie |
| No method | `r nm -b t.jar -X POST -H "$O" -H "$J" $B/sessions -d '{"grant":"two_factor","code":"000000"}'` | 422 `method` required |
| Wrong code | same with `"method":"totp"` | 422 `invalid_code` |
| Authenticator code | `r totp -b t.jar -c t.jar -X POST -H "$O" -H "$J" $B/sessions -d "{\"grant\":\"two_factor\",\"method\":\"totp\",\"code\":\"$(totp $SECRET 1)\"}"` | 201 `me`, `auth.auth_methods` `["password","totp"]`, `dimmi_2fa` cleared, session cookie set |
| Challenge works once | same command again | 401 `two_factor_challenge_expired` |
| Recovery code | new challenge in `rc.jar`, then `{"grant":"two_factor","method":"recovery_code","code":"$RC"}` | 201 `["password","recovery_code"]`; mail "A recovery code was used to sign in" saying 9 are left |
| New recovery codes | `r regen -b a.jar -X POST -H "$O" -H "$J" $F -d '{"type":"recovery_code"}'` | 201 `remaining: 10` and 10 `codes`, `no-store`; an old code at the next challenge 422 `invalid_code` |
| Recovery codes stay | `DELETE $F/fac_…` with the recovery set's id from the list | 409 `recovery_codes_required` |
| Turn off | `r off -b a.jar -X DELETE -H "$O" $F/$FAC` | 204; mail "Two-factor authentication is off"; `POST $B/sessions` with `$P` 201 again |
| Turn off all at once | turn on again, then `r alloff -b a.jar -X DELETE -H "$O" $F` | 204, the list empty |
| Operator turns it off | turn on again, then `dimmi user:disable-2fa $ADA`, again, then `dimmi user:disable-2fa nobody@example.com` | "Two-factor authentication is off for …", exit 0; then "was already off", exit 0; then "no account has this email", exit 1 |
| Require it without 2FA | Bob, owner of Acme since the transfer: `r req -b b.jar -X PATCH -H "$O" -H "$J" -H "$H" $B/workspace -d '{"require_two_factor":true}'` | 409 `two_factor_not_enabled` |
| Require it | Bob adds and activates an authenticator as Ada did, with `b.jar`, then the same | 200 `require_two_factor: true`; mail "Acme Support now requires two-factor authentication" to each active member without 2FA; a token of Bob's sending the field 403 `first_party_only` |
| Kept out without 2FA | any member without 2FA, by session or token: `GET $B/workspace` with `-H "$H"` | 403 `two_factor_required` with `requirement: two_factor`, `action_url: /d/settings/security`; `GET $B/me`, `GET $F` and `GET $B/sessions` still 200 |
| In right after setup | that member adds and activates an authenticator, then `GET $B/workspace` again | 200 at once |
| Cannot turn it off | Bob: `r boff -b b.jar -X DELETE -H "$O" $F` | 409 `two_factor_required_by_workspace`, `workspaces: [{"id":"ws_…","name":"Acme Support"}]`; nothing removed; `dimmi user:disable-2fa $BOB` still works |
| Lift it | Bob (with 2FA again): `{"require_two_factor":false}` | 200 `require_two_factor: false`, no mail |
| Purge | start an enrollment and leave it; with the server running, wait past the hour | the `pending` factor gone, a `purge_expired_auth_records` job `completed` in `river_job` |

### Passkeys

A passkey needs a device to sign the server's challenge, so curl checks what the server hands out and what it
refuses, and a browser runs the whole flow. In Chrome, DevTools > More tools > WebAuthn > "Enable virtual
authenticator environment", then add an authenticator: protocol `ctap2`, transport `internal`, resident keys and user
verification on. Within 15 minutes of Ada's last sign-in in `a.jar`, as above.

| Flow | Command | Expect |
|---|---|---|
| Offered | `curl -s $B/auth/config \| jq -c '[.data.factors[].type]'` | `["totp","passkey","recovery_code"]` |
| Start a passkey | `r pk -b a.jar -X POST -H "$O" -H "$J" $F -d '{"type":"passkey","name":"Work laptop"}'; PK=$(jq -r .data.id body)` | 201, `no-store`, `type: passkey`, `status: pending`, `name: Work laptop`; `setup.options` with `rp.id` the host of `app.url` (`localhost`), `rp.name` `dimmi`, a `challenge`, `user.name` Ada's email, `authenticatorSelection` `userVerification` and `residentKey` `preferred`, `attestation: none`, and `excludeCredentials` naming every passkey Ada already holds; no `secret` |
| Bad answer | `r pkbad -b a.jar -X PATCH -H "$O" -H "$J" $F/$PK -d '{"credential":{"id":"x"}}'`, then `-d '{"credential":"x"}'` | 422 `invalid_credential` on `credential`; then 422 on `credential` |
| No passkeys on an IP address | restart with `DIMMI_APP__URL=http://127.0.0.1:18080`, then the start above | 422 `factor_type_not_available`; `/auth/config` lists `totp` and `recovery_code` only |
| Add in a browser | `/d/settings/security`, "Add passkey", name it, "Continue" | on an account without 2FA the ten recovery codes show once; the passkey is listed by name with "Added …, never used", and "Turn off" appears |
| Challenge | `POST $B/sessions` with `$P` into a fresh jar | 200 `two_factor_challenge`; the passkey's method has `options` with `rpId`, a `challenge` and `allowCredentials` naming each passkey, the same options on every passkey; codes have `options: null`; recovery codes listed last |
| Sign in in a browser | sign out, sign in by password | `/login/two-factor` says "Use your passkey"; "Continue with passkey" lands on `/d`; the session's `auth.auth_methods` is `["password","passkey"]`; the list then says "last used …" and the row's `last_counter` moved |
| Answer for another challenge | in the browser start two sign-ins in two tabs; answer the older tab's challenge, then the newer | the newer challenge's cookie holds, so the older answer is 422 `invalid_credential` and counts as a wrong attempt |
| Two passkeys | add a second virtual authenticator and "Add passkey" again | both listed; either one finishes a sign-in; no new recovery codes |
| Beside an authenticator | an account with an authenticator app adds a passkey | no new recovery codes; the challenge offers `totp`, `passkey`, `recovery_code` in that order, and either signs in |
| Rename and remove | "Rename" a passkey, then "Remove" one of two | the new name sticks after a reload; 2FA stays on with the other |
| Remove the last | "Remove" the only passkey | the dialog says it is the last second factor; 2FA turns off and the recovery codes go; with Acme requiring 2FA instead 409 `two_factor_required_by_workspace` and nothing removed |

### Operator commands

Run against a fresh database with the server stopped or running.

| Flow | Steps | Expect |
|---|---|---|
| First user | `dimmi user:create --email ada@acme.com --name Ada --generate-password --workspace Acme` on an empty install | "owner of Acme", a printed password that signs in; a second run for another email with no `--workspace-slug` creates a lone account |
| Join a workspace | `dimmi user:create --email bob@acme.com --name Bob --generate-password --workspace-slug acme --role admin`, again | "is now admin of /w/acme", then exit 1 "already" |
| List users | `dimmi user:list`, `dimmi user:list --json` | a table, then JSON with `two_factor` and `workspaces` per user |
| Reset without mail | `dimmi user:reset-password bob@acme.com`, then with `--password "a long new secret"` | a `/reset-password#id=prs_…` link; then Bob's browser session answers 401 and the new password signs in |
| Verify and sign out | `dimmi user:verify-email bob@acme.com`, `dimmi user:logout bob@acme.com` | verified; Bob's sessions end, his tokens keep working |
| Suspend | `dimmi workspace:suspend acme --reason "unpaid"`, a `PATCH /workspace`, then `dimmi workspace:unsuspend acme` | the change answers 403 `workspace_suspended` while reads work; audit `workspace.suspended` then `workspace.unsuspended` |
| Restore and purge | `DELETE /workspace`, `dimmi workspace:restore acme`; delete again, `dimmi workspace:purge acme`, then with `--force` | restored and owners mailed; purge refuses without `--force`, then removes the workspace |
| Token for scripts | `dimmi token:create --email ada@acme.com --workspace-slug acme --name ci --abilities conversations:read --expires 90d` | a `dmi_live_…` secret printed once that answers `GET /api/v1/me` |

### Personal access tokens

Bob (`b.jar`) owns Acme after the transfer above. Minting needs a recent confirmation, like adding a factor: confirm
with `POST $B/user/confirmation` first when Bob's sign-in is older than 15 minutes. `T` sends a token instead of a
cookie; `-g` keeps curl from reading the brackets of `filter[member]` as a pattern:

```bash
T(){ echo "Authorization: Bearer $1"; }; K=$B/tokens
```

| Flow | Command | Expect |
|---|---|---|
| Abilities offered | `r ab -b b.jar -H "$O" -H "$H" $K/abilities` | 200, every ability group with `object: token_ability`, its `permissions` and `allowed: true` for an owner |
| Unknown ability | `r bad -b b.jar -X POST -H "$O" -H "$J" -H "$H" $K -d '{"name":"CI","abilities":["made:up"]}'` | 422 `ability_not_allowed`, `abilities: ["made:up"]` |
| Expiry in the past | same with `"abilities":["*"],"expires_at":"2020-01-01T00:00:00Z"` | 422 `invalid_expiry` on `expires_at` |
| Mint | `r mint -b b.jar -X POST -H "$O" -H "$J" -H "$H" $K -d '{"name":"CI deploy","abilities":["members:read"],"expires_at":"2027-02-01T00:00:00Z"}'; TOK=$(jq -r .data.plain_text_token body); TID=$(jq -r .data.id body)` | 201, `Cache-Control: no-store`, `plain_text_token` `dmi_live_…` of 47 characters, `prefix` its first 13; mail "A new API token was made for your dimmi account" with the name, the abilities and the IP |
| Use it | `r use -H "$(T $TOK)" $B/members`, then `r me -H "$(T $TOK)" $B/me` | 200; `auth.type: personal_access_token`, `auth.token` with `name`, `prefix`, `expires_at`, `workspaces: null`, `member.permissions` `["members:read"]` |
| Ability enforced | `r ab2 -X PATCH -H "$(T $TOK)" -H "$J" $B/workspace -d '{"name":"x"}'` | 403 `missing_ability`, `required_ability: settings:write` |
| Tokens never mint tokens | `r fp -H "$(T $TOK)" $K` | 403 `first_party_only` |
| Typo | the token with its last character changed | 401 `invalid_token` |
| List | `r list -b b.jar -H "$O" -H "$H" $K` | 200, the token with `last_used_at` and `last_used_ip` filled, no `plain_text_token` |
| Rename | `r ren -b b.jar -X PATCH -H "$O" -H "$J" -H "$H" $K/$TID -d '{"name":"Deploy bot"}'` | 200 `name: Deploy bot`, abilities unchanged |
| Revoke | `r rev -b b.jar -X DELETE -H "$O" -H "$H" $K/$TID`, then `me` with the token | 204, then 401 `token_revoked`; the list keeps it with `revoked_at` |
| Unknown id | `r nf -b b.jar -H "$O" -H "$H" $K/tok_01jc0000000000000000000000` | 404 `not_found` |
| Role ceiling | a `member` of Acme asks for `["billing:read","members:read"]` | 422 `ability_not_allowed`, `abilities: ["billing:read"]` |
| Others' tokens | that member: `DELETE $K/<Bob's tok_>` and `curl -g … "$K?filter[member]=all"`; Bob: `curl -g … "$K?filter[member]=all"` | 404 and 403 for the member; Bob sees every member's tokens |
| Deactivation ends tokens | Bob deactivates the member, then the member's token on `me` | 401 `token_revoked` |
| In a browser | open `$W/d/settings/tokens`, make a token with one ability, copy it, call `$B/members` with it, then revoke it | the secret shows once with a copy button; the list says what it may do, when it was last used and when it expires; after revoking it shows "Revoked" and the token gets 401 |

### Realtime

The realtime endpoint speaks centrifuge's JSON protocol over WebSocket. `websocat` (`brew install websocat`) is
enough to drive it by hand: each line you type is one command, each line printed is a reply or a push. A browser
sends its cookie and its page's `Origin` with the upgrade; a token goes in the connect command instead, never in
the URL. Read the session cookie out of `a.jar` (`SES=$(awk '/dimmi_session/ {print $7}' a.jar)`) and take the
workspace id from sign-up (`WS=ws_…`) and a member id from `GET $B/members` (`MEM=mem_…`).

```bash
WSS=ws://127.0.0.1:18080/connection/websocket
listen(){ websocat -H "Cookie: dimmi_session=$SES" -H "$O" "$@" $WSS; }
```

| Flow | Command | Expect |
|---|---|---|
| Connect with the cookie | `listen`, then type `{"id":1,"connect":{}}` | `{"id":1,"connect":{"client":"…","ping":25,…}}` |
| No credentials | `websocat $WSS`, then `{"id":1,"connect":{}}` | `{"id":1,"error":{"code":101,"message":"unauthorized"}}` |
| Cookie from another site | `listen -H 'Sec-Fetch-Site: cross-site'`, then connect | error 101 |
| Bad token | `websocat $WSS`, then `{"id":1,"connect":{"token":"dmi_live_nope"}}` | error 101 |
| Follow the workspace | after connecting, `{"id":2,"subscribe":{"channel":"workspace:'$WS'"}}` | `subscribe` reply with `recoverable: true`, `epoch`, `offset` |
| Other workspace refused | sign up `$BOB` with his own workspace, connect with his cookie, subscribe `workspace:$WS` | error 103 `permission denied` |
| Unknown channel | `{"id":3,"subscribe":{"channel":"lobby"}}` | error 102 |
| Live rename | keep the subscription open; in another shell `r ren -b a.jar -X PATCH -H "$O" -H "$J" $B/workspace -d '{"name":"Acme Live"}'` | a push `{"push":{"channel":"workspace:ws_…","pub":{"data":{"id":"evt_…","event":"workspace.updated",…,"data":{"id":"ws_…","object":"workspace","changes":["name"]}},"offset":…}}}` |
| Recover after a drop | note the `epoch` and last `offset`, quit, rename again, reconnect and `{"id":2,"subscribe":{"channel":"workspace:'$WS'","recover":true,"epoch":"…","offset":…}}` | `recovered: true` with the missed `workspace.updated` in `publications` |
| Client publish refused | `{"id":4,"publish":{"channel":"workspace:'$WS'","data":{}}}` | an error reply; nothing is broadcast |
| Presence | `{"id":5,"presence":{"channel":"workspace:'$WS'"}}` | one entry per connection, `user: usr_…`, `conn_info` `{"name":"Ada Lovelace","avatar_url":null}`; a channel not subscribed: error 103 |
| Deactivated member | add `$CAROL` as a member, connect with her cookie, subscribe the workspace; from `a.jar` `PATCH $B/members/mem_…` `{"status":"deactivated"}` | her socket closes with code 4009 `member_removed` within a second; Ada's subscription gets `member.deactivated` |
| Typing relay | `r typ -b a.jar -X POST -H "$O" -H "$J" -H "Dimmi-Workspace: $WS" $B/conversations/conv_…/typing-indicators -d '{}'` (a conversation started by a customer) | 202 `{"data":{"object":"typing_indicator","state":"start","expires_at":"…"}}`; an unknown `conv_` 404; `{"state":"dancing"}` 422 |
| Node SDK | `pnpm exec playwright test tests/e2e/realtime.spec.ts` | centrifuge-js connects with the cookie, follows the workspace and a conversation, lists presence, recovers after a disconnect |

### Workspace access policy

Ada (`a.jar`) owns Acme. Changing the rules needs a recent confirmation, like minting a token. The server must run
with `DIMMI_HTTP__TRUSTED_PROXIES=127.0.0.1/32` so `X-Forwarded-For` stands in for other addresses. `P` is the
policy path:

```bash
P=$B/workspace/access-policy; WH="Dimmi-Workspace: $WS"
```

| Flow | Command | Expect |
|---|---|---|
| Read | `r ap -b a.jar -H "$O" -H "$WH" $P` | 200 `object: access_policy`, empty lists, null limits, `your_ip: 127.0.0.1`, `available_sign_in_methods` starting `password`, `tokens_outside: 0`; no `sso_connection_ids` |
| Session limit | `r ml -b a.jar -X PATCH -H "$O" -H "$J" -H "$WH" $P -d '{"session_max_age_minutes":60}'` | 200 with `session_max_age_minutes: 60`; mail "The sign-in rules of Acme changed" to the other owners only; audit `workspace.access_policy_changed` |
| Limit out of range | same with `59`, then `43201` | 422 on `session_max_age_minutes` each time |
| Clear a limit | same with `{"session_max_age_minutes":null}` | 200, the limit null, other keys kept |
| Old session refused | `UPDATE sessions SET authenticated_at = now() - interval '2 hours'` for Ada, then `GET $B/workspace` | 403 `reauthentication_required`, `requirement: reauthentication_required`, `action_url: /login?reauth=1`, `reason: max_age`; `GET $B/me` still 200 |
| Sign in again | `POST $B/sessions` with `$P` and `"reauth":true` into `a.jar`, then `GET $B/workspace` | 201, then 200 |
| Own method kept | `-d '{"allowed_sign_in_methods":["magic_link"]}'` from a password session | 422 `sign_in_method_excludes_caller` |
| Method refused | set `["password"]`, sign in by mailed code in `c.jar`, `GET $B/workspace` | 403 `sign_in_method_required`, `allowed_methods: ["password"]` |
| Dashboard list keeps you | `-H 'X-Forwarded-For: 198.51.100.7' -d '{"dashboard_ip_allowlist":[{"cidr":"203.0.113.0/24","label":"Office"}]}'` | 422 `ip_allowlist_excludes_caller`, `ip: 198.51.100.7` |
| Address refused | save `127.0.0.1` and `203.0.113.0/24`, then `GET $B/workspace` with `X-Forwarded-For: 198.51.100.7` | 403 `ip_not_allowed`, `action_url: null`, `ip: 198.51.100.7`; `GET $B/me` and `GET $B/sessions` still 200 |
| API list | save `api_ip_allowlist` `203.0.113.0/24`, then a token on `$B/members` with `X-Forwarded-For: 198.51.100.7` | PATCH answers `tokens_outside` counting live tokens last used elsewhere; the token gets 403 `ip_not_allowed` naming the address and "allowlist" |
| Tokens never change rules | `PATCH $P` with a token holding `settings:write` | 403 `first_party_only` |
| Sockets close on change | keep a `listen` socket on `workspace:$WS`, then any PATCH that changes a value | the socket closes; reconnecting checks the new rules, and subscribing from outside the list is error 103 |
| Operator clears the list | `bin/dimmi workspace:clear-access-policy acme --only ip_allowlist` | prints the cleared keys; the address works again; audit actor `system`; a second run prints "Nothing to clear" |
| In a browser | `$W/d/settings/workspace-security`: untick a method, pick "Sign in again after: 1 hour", add an address | each change saves at once and survives a reload; a refused change shows its reason under the section; a refused workspace page shows "Sign in again to continue" with a button back to the same page |

### Custom roles

Ada (`a.jar`) owns Acme; Carol is a member. Shaping roles needs a recent confirmation. `R` is the roles path:

```bash
R=$B/roles
```

| Flow | Command | Expect |
|---|---|---|
| Catalog | `r perms -b a.jar -H "$O" -H "$WH" $B/permissions` | 200, one entry per permission with `label`, `description`, `group`, `assignable` (false for owner-only and `system:*`) and `base_roles` |
| Built-in roles | `r roles -b a.jar -H "$O" -H "$WH" $R` | 200, Owner, Admin and Member first with `kind: built_in` and `id: null`, then custom roles by name |
| Create | `r mk -b a.jar -X POST -H "$O" -H "$J" -H "$WH" $R -d '{"name":"Read only","base_role":"member","permissions":["members:read","conversations:read"]}'; ROL=$(jq -r .data.id body)` | 201 `rol_…`, `kind: custom`, `member_count: 0`; audit `role.created` |
| Above its base | same with `"permissions":["members:read","settings:write"]` | 422 `permission_above_base_role` naming `settings:write` |
| Owner-only | same with `"base_role":"admin","permissions":["members:read","workspace:delete"]` | 422 `permission_not_assignable` |
| Without members read | same with `"permissions":["conversations:read"]` | 422 `permission_required` |
| Assign | `r as -b a.jar -X PATCH -H "$O" -H "$J" -H "$WH" $B/members/<Carol's mem_> -d '{"custom_role_id":"'$ROL'"}'` | 200, `role: member`, `custom_role` with the id and name; Carol's `/me` lists only `members:read` and `conversations:read` |
| Narrowed | Carol replies to a conversation | 403 `forbidden`; her tokens narrow the same way |
| Edit applies next request | `PATCH $R/$ROL` with `conversations:write` added | 200; Carol's next reply works; audit `role.updated` with `permissions.added` |
| In use | `DELETE $R/$ROL` | 409 `role_in_use`, `member_count: 1` |
| Move everyone | `PATCH $B/members -d '{"ids":["<Carol's mem_>"],"changes":{"custom_role_id":null}}'` | 200 `meta.updated: 1`; then the delete answers 204 |
| Owners and self | assign a role to an owner or to yourself | 422 `invalid_transition` |
| In a browser | `$W/d/settings/roles`, "New role" > "Read only", "Create role"; then `/d/settings/members`, the member's menu > "Make Read only" | the role is listed with "Based on member · 1 member"; the member's chip shows "Read only"; their reply shows "This action is unauthorized." |

### Single sign-on connections and domains

Ada (`a.jar`) owns Acme. Changes need a recent confirmation. `C` and `D` are the connection and domain paths:

```bash
C=$B/sso-connections; D=$B/organization-domains
```

| Flow | Command | Expect |
|---|---|---|
| Create SAML | `r sc -b a.jar -X POST -H "$O" -H "$J" -H "$WH" $C -d '{"protocol":"saml","name":"Okta"}'; SSO=$(jq -r .data.id body)` | 201 `ssoc_…`, `status: draft`, `sp.acs_url` ends in `/auth/saml/$SSO/acs`, `config.sp_certificate` set, no secret in the body; audit `sso_connection.created` |
| Bad certificate | same with `"config":{"idp_certificates":["nope"]}` | 422 `invalid_saml_metadata` |
| OpenID without secret | `-d '{"protocol":"oidc","name":"Entra","config":{"issuer":"https://login.example.com","client_id":"x"}}'` | 422 `invalid_saml_metadata` |
| Activate untested | `PATCH $C/$SSO -d '{"status":"active"}'` | 422 `invalid_transition` |
| Add domain | `r da -b a.jar -X POST -H "$O" -H "$J" -H "$WH" $D -d '{"domain":"acme.com"}'; DOM=$(jq -r .data.id body)` | 201 `odom_…`, `status: pending`, `record_name: _dimmi-verify.acme.com`, `record_value: dimmi-domain-verification=…` |
| Free mail | same with `gmail.com` | 422 `domain_not_claimable` |
| Check without record | `POST $D/$DOM/checks` | 201, `status: pending`, `found_records: []` |
| Operator proof | `./bin/dimmi sso:verify-domain acme.com` | prints "Verified acme.com"; the domain shows `verified_via: cli`; audit `organization_domain.verified` by the system |
| Route | `PATCH $D/$DOM -d '{"sso_connection_id":"'$SSO'"}'` | 200 with the connection id |
| Same domain elsewhere | another organization adds and proves `acme.com` | 409 `domain_taken` on proof |
| Members cannot see it | a member's `GET $C` | 403 `forbidden` |
| Operator off switch | `./bin/dimmi sso:disable <slug>` | every connection of the workspace's organization is `disabled` and unenforced |
| Test sign-in | `POST $C/$SSO/tests` | 201 `sso_connection_test` with the identity provider `url`, and the `__Host-dimmi_sso` cookie; signing in there marks the connection `tested` and lands on `/d/settings/single-sign-on?tested=1` without a session |
| SP metadata | `curl $W/auth/saml/$SSO/metadata` | 200 `application/samlmetadata+xml` naming the ACS and signing certificate |
| Sign in | `$W/auth/sso/$SSO/redirect` in a browser, sign in at the identity provider | lands on `/d` signed in; a new person under the verified domain joins with the default role when `jit_enabled`, else `/login?error=sso_no_access` |
| Replay | post the same `SAMLResponse` again | `/login?error=sso_failed`; the connection's `last_error.code` says why |
| Lookup | `r sl -X POST -H "$J" $B/sso-lookups -d '{"email":"ada@acme.com"}'` | 200 with `redirect_url`, `name`, `enforced`; another domain gives `redirect_url: null` |
| Enforced | `PATCH $C/$SSO -d '{"enforced":true,"confirm_owners_can_sign_in":true}'`, then password sign-in for an address under the domain | 409 `sso_required` with `redirect_url`; magic links answer the same; reset requests send no mail |
| Owner bypass | `POST /sessions {"grant":"password","sso_bypass":true,...}` as an owner at the domain | 201; the session works for one hour, then `sso_required`; every owner gets "An owner signed in without single sign-on". A non-owner gets `invalid_credentials` |
| Metadata refresh | set `config.idp_metadata_url`, run the daily `refresh_sso_metadata` job | certificates follow the published metadata; a failing address sets `metadata_error` and keeps the old settings |
| OpenID Connect | create `{"protocol":"oidc","name":"Entra","client_secret":"…","config":{"issuer":"https://login.microsoftonline.com/<tenant>/v2.0","client_id":"…"}}`, register `sp.redirect_uri` at the issuer, then test and sign in | the secret never comes back (`has_secret: true`); the callback signs in like SAML; a callback without the attempt cookie lands on `/login?error=sso_failed` |
| In a browser | `$W/d/settings/single-sign-on`, "New connection", then "Add domain" and the operator command | the connection is listed as "Draft" with its reply URL under "Details"; after a reload the domain shows "Verified by the operator" with a connection picker |

### Browser pages

Run `make build` first so the binary embeds the browser app.

| Flow | Command | Expect |
|---|---|---|
| Sign-in pages | `for p in /login /signup /login/magic /login/two-factor /forgot-password /reset-password /verify-email /invitation; do curl -s -o /dev/null -D - $W$p \| grep -iE '^(cache-control\|referrer-policy)'; done` | `no-store` and `no-referrer` on each |
| App page | `curl -s -D - $W/d \| grep -iE '^(cache-control\|content-security-policy)'` | `no-cache`; policy with `'nonce-…'`, no `unsafe-inline`, no dev server |
| Theme | `curl -s -b dimmi_theme=dark $W/login \| grep -o '<html[^>]*>'` | `<html lang="en" class="dark">` |
| In a browser | open `$W/signup`, create an account, open the verification mail's link, sign out, sign in by password, then by mailed code | lands on `/d` each time; provider buttons first when providers are configured; light and dark switch and survive a reload |
| Live in two browsers | add Bob to Ada's workspace, sign in as Bob in a private window on `/d/inbox`, then rename the workspace in Ada's `/d/settings/workspace` | Bob's sidebar shows the new name without a reload; stop the server and after two seconds Bob sees "Reconnecting…", which goes away once it is back |
| Two-factor in a browser | in `/d/settings/security` add an authenticator (scan the QR code, or "Can't scan the code?" and copy the key), confirm with its code, copy the recovery codes; sign out, sign in by password, answer with the app's code; sign out again and use "Use a recovery code instead" | the key stays hidden until asked and copies from its read-only field; "Copy codes" puts all ten on the clipboard, one per line; each sign-in stops at `/login/two-factor`, then lands on `/d`; the settings then say "9 of 10 left" |
| Require 2FA in a browser | open `/d/settings/workspace` and switch "Require two-factor authentication" before and after adding an authenticator | refused with a note to turn it on for your own account first, then on and still on after a reload |
| Sessions in a browser | sign in from a second browser, open `/d/settings/security` | both sessions listed, this one marked "This session"; "Revoke" and "Sign out others" sign the other browser out |

### Messenger widget

Every workspace starts with one widget. Its `id` from `GET $B/widget-settings` is also the app id the snippet
sends (`APP=wgt_…`). A visitor's browser sends the page's `Origin`; an empty allowlist accepts any. The
session token from `POST /sessions` is the visitor's bearer (`V=dmi_ws_…`). On the local disk a finished file
has `url: null` until downloads go through the server; on an S3 disk it is a signed link.

```bash
M=http://127.0.0.1:18080/widget/v1; B_ROOT=http://127.0.0.1:18080; SITE='Origin: https://shop.example'
vis(){ r "$@" -H "$SITE" -H "Authorization: Bearer $V"; }
```

| Flow | Command | Expect |
|---|---|---|
| Widgets of a workspace | `r ws -b a.jar -H "$O" -H "Dimmi-Workspace: $WS" $B/widget-settings` | 200, one widget with `app_id`, `allowed_origins: []`, `allow_anonymous: true` |
| Eleventh widget refused | create widgets with `POST $B/widget-settings` `{"name":"Site N"}` until it stops | 201 up to ten, then 422 `entitlement_limit_reached` |
| Path in an allowed site | `PATCH $B/widget-settings/wgt_…` `{"allowed_origins":["https://shop.example/app"]}` | 422 on `allowed_origins.0` |
| Config | `r cfg -H "$SITE" $M/apps/$APP/config`, then again with `-H "If-None-Match: <etag>"` | 200 with `company`, `appearance`, `texts`, `realtime`; then 304 |
| Unknown app | `r no -H "$SITE" $M/apps/app_nope/config` | 404 `app_not_found` |
| Site not allowed | allow only `https://shop.example`, then config with `-H 'Origin: https://evil.example'` | 403 `origin_not_allowed` |
| Preflight | `curl -s -D - -o /dev/null -X OPTIONS -H "$SITE" -H 'Access-Control-Request-Method: PUT' $M/attachments` | 204, `Access-Control-Allow-Origin: https://shop.example`, methods include `PUT` |
| Team availability | `r av -H "$SITE" $M/apps/$APP/availability` | 200, at most three avatars, no emails |
| Start a session | `r ses -X POST -H "$SITE" -H "$J" $M/sessions -d "{\"app_id\":\"$APP\"}"` | 201, `token` `dmi_ws_…`, a lead `contact` |
| Honeypot | same with `"website":"x"` | 201, looks the same; its row in `widget_sessions` has `bot_score` 100 |
| Resume | `vis me $M/session` | 200, no `token`, `expires_at` moved on |
| Profile | `vis pr -X PATCH -H "$J" $M/contact -d '{"name":"Grace","email":"grace@shop.example"}'` | 200 with the name and email |
| Start a conversation | `vis cv -X POST -H "$J" $M/conversations -d '{"client_id":"<uuid>","body":"Export fails"}'` | 201, `conv_…`, `meta.message`; the same `client_id` again gives the same message |
| Teammate reply | reply from the inbox (`POST $B/conversations/conv_…/replies`), then `vis ms $M/conversations/conv_…/messages` | the reply with `author.type: member`, no member email, no notes |
| Read state | `vis rd -X PATCH -H "$J" $M/conversations/conv_…/read-state -d '{"last_read_message_id":"msg_…"}'` | 200 `unread_count: 0`; an older id leaves it |
| Typing | `vis ty -X POST -H "$J" $M/conversations/conv_…/typing-indicators -d '{"state":"started"}'` | 202 |
| Ping | `vis pg -X POST -H "$J" $M/pings -d '{"page":{"url":"https://shop.example/a?token=abc&page=2"}}'` | 202; the session keeps `?page=2` only |
| Upload a file | `vis up -X POST -H "$J" $M/attachments -d '{"filename":"shot.png","content_type":"image/png","size_bytes":<bytes>}'` | 201 `attachment_upload`, `upload.url` ending `/content` on the local disk, a signed S3 address on the S3 disk |
| Send the bytes | `vis put -X PUT -H 'Content-Type: image/png' --data-binary @shot.png $M/attachments/att_…/content` | 204; more bytes than announced: 413 `attachment_too_large` |
| Finish the upload | `vis done -X PATCH -H "$J" $M/attachments/att_… -d '{"status":"uploaded"}'`, twice | 200 `attachment` with `width` and `height`, the same both times |
| Program renamed | upload `/bin/ls` as `cat.png` and finish | 422 `attachment_type_blocked`, the row is gone |
| Send with the file | `POST $M/conversations/conv_…/messages` with `"attachment_ids":["att_…"]` | 201, the file under `attachments`; a pending upload: 409 `attachment_not_uploaded` |
| Stream token | `vis rt -X POST $M/realtime-tokens` | 201, `token`, `url` ending `/connection/uni_sse`, `channel` `widget_contact:ctc_…` |
| Live stream | `curl -N "http://127.0.0.1:18080/connection/uni_sse?cf_connect=$(jq -rn --arg t <token> '{token:$t}\|tojson\|@uri')"`, then reply from the inbox | a `connect` push, then `message.created` with `widget_id` |
| Stream past the cap | open eleven streams with fresh tokens | the first ends with `disconnect` code 4004 |
| Polling | `vis up $M/updates`, reply, then `vis up "$M/updates?since=<meta.cursor>"` | 200, the reply as `message.created` in `data`, `meta.has_more: false`; a forged `since` 400 |
| Sign out | keep a stream open, `vis bye -X DELETE $M/session` | 204; the stream gets `widget.session_revoked` (usually before the close) and `disconnect` 4009; `vis me $M/session` gives 401 `session_revoked` |
| Token as API bearer | `r bad -H "Authorization: Bearer <realtime token>" $M/session` | 401 |
| Messenger in a browser | serve a page with `<script src="$B_ROOT/widget/v1/loader.js" data-app-id="$APP" async></script>` from another local port (`python3 -m http.server 18102`), open it, click the launcher, "Chat with us", write; reply from the inbox | launcher shows with the widget's colors; the reply appears without a reload; after a reload the Messages tab lists the conversation; a wrong `data-app-id` shows no launcher and one console error `app_not_found` |
| Messenger preview | the same page with `data-preview` on the snippet | sample conversations and articles, no request to the server beyond the loader, the core and the preview chunk |

### Email channel

The install receives mail only with an inbound domain. Add these lines to `$E2E/env` before migrating, so every new
workspace gets a forwarding address, and the test inbound source takes raw emails over HTTP in place of a receiving
provider:

```bash
cat >> "$E2E/env" <<'EOF'
DIMMI_CHANNELS__EMAIL__INBOUND_DOMAIN=inbound.e2e.test
DIMMI_CHANNELS__EMAIL__INBOUND__DRIVERS=test
DIMMI_CHANNELS__EMAIL__INBOUND__TEST_SECRET=e2e-inbound-secret
EOF
EA=$B/email-addresses; IN='Authorization: Bearer e2e-inbound-secret'
mailraw(){ printf 'From: Bob <bob.%s@customer.test>\r\nTo: %s\r\nSubject: %s\r\nMessage-ID: <%s>\r\n%s\r\n\r\n%s\r\n' \
  "$RUN" "$1" "$2" "$3" "$4" "$5"; }
```

Sign up a new account first (a workspace created before the inbound domain was set has no address; run
`dimmi workspace:repair <slug>` to give it one).

| Flow | Command | Expect |
|---|---|---|
| Default address | `r ea -b a.jar -H "$O" $EA; FWD=$(jq -r '.data[0].forwarding_address' body); EMA=$(jq -r '.data[0].id' body)` | 200, one `email_address`, `forwarding_address` `<slug>@inbound.e2e.test`, `is_default: true`, twelve `setup_guides`, the first `recommended` (`generic` for the e2e domain, which has no MX), each with filled-in `steps`, `warnings` and a `deep_link`, `inbound_verified_at: null` |
| Source closed without the secret | `r no -X POST $W/webhooks/test/inbound --data-binary 'Subject: x'` | 401 `unauthenticated` |
| Mail arrives | `mailraw $FWD 'Refund for order 1042' first.$RUN@customer.test '' 'Charged twice.' \| r in -X POST -H "$IN" --data-binary @- "$W/webhooks/test/inbound?to=$FWD"` | 202 `workspaces: 1` |
| Activity log | `r act -b a.jar -H "$O" $EA/$EMA/inbound-emails` | 200, `status: processed`, `classification: normal`, a `conversation` and `message` id; the address now has `inbound_verified_at` |
| Redelivery | the same command with `&id=p1` twice | 202 twice; the activity log holds it once (a different `id` with the same Message-ID is stored `ignored`, `duplicate`) |
| Reply threads | `mailraw $FWD 'Re: Refund for order 1042' reply.$RUN@customer.test "In-Reply-To: <first.$RUN@customer.test>" 'Thanks!' \| r re …` | 202; `select count(*) from conversations` unchanged, the message joins the first conversation |
| Plain text kept | `mailraw $FWD 'Broken page' plain.$RUN@customer.test '' $'The <div> tag and 2*3*4.\r\n\r\nOn Mon, Nov 2, 2026 at 9:00 AM Ann <ann@example.com> wrote:\r\n> Which page?' \| r pl …`, then `r tl -b a.jar -H "$O" $B/conversations/<its conv_>/messages` | the message has `body` "The <div> tag and 2*3*4.", `body_format: plain`, `has_original: true` |
| Full message | `r og -b a.jar -H "$O" $B/conversations/<conv_>/messages/<msg_>/original` | 200 `message_original`, `text` with the quoted lines, `html: null`, `raw_available: true` for an owner |
| Raw email | `r raw -b a.jar -H "$O" $B/conversations/<conv_>/messages/<msg_>/original/raw` | 200 `application/octet-stream`, `Content-Disposition: attachment; filename="msg_….eml"`, `X-Content-Type-Options: nosniff`, `Content-Security-Policy: default-src 'none'; sandbox`; a member without `email:read_raw` gets 403 and `raw_available: false` |
| Out of office | `mailraw $FWD 'Automatic reply: away' ooo.$RUN@customer.test 'Auto-Submitted: auto-replied' 'Away.' \| r ooo …` | 202; activity `ignore_reason: auto_reply_no_thread`, no new conversation |
| Unknown address | the first mail to `ghost@inbound.e2e.test` | 202 `workspaces: 0`, nothing stored |
| Add an address | `r add -b a.jar -X POST -H "$O" -H "$J" $EA -d '{"address":"sales@acme.test"}'` | 201, `forwarding_address` `<slug>.sales@inbound.e2e.test`; the same again 409 `address_in_use`; `x@inbound.e2e.test` 422 `invalid_address` |
| Settings | `r upd -b a.jar -X PATCH -H "$O" -H "$J" $EA/$EMA -d '{"settings":{"bulk_mail_handling":"drop"}}'` | 200 with `bulk_mail_handling: drop` |
| Forwarding test | `r t -b a.jar -X POST -H "$O" $EA/$EMA/tests` | 201 `email_address_test` `status: waiting`, `id` `eat_...`, `sent_to` the public address; Mailpit holds it with an `X-Dimmi-Test` header; feeding that raw mail back through the source shows `ignore_reason: forwarding_test`; feeding it again shows `test_stale`; with the log mailer it is 422 `no_sender_available` with the subject to send by hand |
| Forwarding test status | `r t -b a.jar -H "$O" $EA/$EMA/tests/$EAT` | 200 `status: received` with `latency_ms` once the mail came back; `timed_out` with `findings` ten minutes after a test that never came back; a second POST within 30 s is 429 `email_test_throttled` |
| Disconnect | `r del -b a.jar -X DELETE -H "$O" $EA/<the sales ema_>` | 204; mail to its forwarding address is stored `ignored` with `address_deleted` |
| In a browser | open `$W/d/settings/email` | the forwarding address with a Copy button, the Gmail steps, "Receiving" once mail arrived, the last emails, "Send test email" and "Add address" |
| Full message in a browser | open the "Broken page" conversation, then "Show full message" | the thread shows the text with its `<div>` and stars as typed; the dialog shows the quoted thread and offers "Download original" |
| Reply not delivered | insert a `hard_bounce` row in `email_suppressions` for the customer, then reply to their email conversation in the browser | the reply shows "Not delivered: The recipient's address bounced or complained before." without a reload |
| Send again | delete that row, then `r rt -b a.jar -X POST -H "$O" $B/conversations/<conv_>/messages/<msg_>/retry` (or "Send again") | 200 with `delivery_status: pending`, the mail reaches Mailpit; the same call again 409 `message_not_failed` |
| Teammate answers by email | the owner replies to a customer's email from their own verified, SPF-passing mailbox | a note by the owner in the thread, no contact for the owner, the conversation not woken |
| Reply-To | send `mailraw` with a `Reply-To: home.$RUN@customer.test` header, then reply in the browser | Mailpit shows the reply to the Reply-To address only; the contact keeps the From address |
| Reply lines and links | reply "Hi,⏎Thanks,⏎Ana⏎⏎[guide](https://example.com/a)⏎⏎3. Open settings" | in Mailpit the HTML shows three lines; the text part reads `guide (https://example.com/a)` and `3. Open settings` |
| Load run | `go build -tags dev -o bin/dimmi-dev ./cmd/dimmi`, then with the same env `bin/dimmi-dev email:load --url $W --to $FWD --secret e2e-inbound-secret --rate 20 --duration 10s` | `sent 200 … 0 refused`, then `processed 200 of 200` with p50 and p95; the full bar is `--rate 50 --duration 10m` with p95 under 10 s, on a database nothing else uses |
| Metrics | restart with `DIMMI_TELEMETRY__METRICS=prometheus`, send one mail, `curl -s http://127.0.0.1:19090/metrics \| grep dimmi_email_` | `dimmi_email_inbound_total` by source, status and classification, `dimmi_email_inbound_latency_seconds` and `dimmi_email_auth_total`; provider webhooks also add `dimmi_email_inbound_bytes`; no address, subject or body in any label |

Provider webhooks. Restart with `DIMMI_CHANNELS__EMAIL__INBOUND__DRIVERS=test,postmark,cloudflare`,
`DIMMI_MAILBOX__SOURCES__POSTMARK__DRIVER=postmark`, `__USERNAME=hook`, `__PASSWORD=pm-secret`,
`DIMMI_MAILBOX__SOURCES__CLOUDFLARE__DRIVER=cloudflare` and `__SECRET=cf-secret`.

| Flow | Command | Expect |
|---|---|---|
| Missing secret | start once without `__PASSWORD` | `dimmi serve` exits: `mailbox.sources.postmark username and password: mailbox: required option missing` |
| Source off | `r off -X POST $W/webhooks/mailgun/inbound/mime -d x` | 404 `inbound_driver_disabled` |
| Postmark | `mailraw $FWD 'Postmark hello' pm.$RUN@customer.test '' 'Hi.' \| jq -Rs "{MessageID:\"pm-$RUN\",OriginalRecipient:\"$FWD\",RawEmail:.}" \| r pm -X POST -u hook:pm-secret -H "$J" --data-binary @- $W/webhooks/postmark/inbound` | 200; the activity log shows it `processed`; the same again 200 and adds nothing |
| Postmark without raw | the same with `{MessageID:"pm-x",OriginalRecipient:"'$FWD'"}` | 503; `dimmi email:inbound-doctor` names "Include raw email content in JSON payload" until the next good post |
| Postmark bad credentials | `-u hook:wrong` | 403 |
| Cloudflare | `TS=$(date +%s); mailraw $FWD 'Cloudflare hello' cf.$RUN@customer.test '' 'Hi.' > cf.eml; SIG=$(printf '%s.%s' $TS $(shasum -a 256 cf.eml \| cut -d' ' -f1) \| openssl dgst -sha256 -hmac cf-secret -r \| cut -d' ' -f1); r cf -X POST -H 'Content-Type: message/rfc822' -H "X-Inbound-Timestamp: $TS" -H "X-Inbound-Signature: $SIG" -H "X-Inbound-Envelope-From: cf.$RUN@customer.test" -H "X-Inbound-Envelope-To: $FWD" --data-binary @cf.eml $W/webhooks/cloudflare/inbound` | 200 and a conversation; a wrong `SIG` or a `TS` ten minutes old 401 |
| Worker | `dimmi email:cloudflare-worker` | prints `worker.js` and `wrangler.toml` with `$W/webhooks/cloudflare/inbound` filled in |
| Doctor | `dimmi email:inbound-doctor` | one line per check (`email.inbound_mx`, `email.inbound_source.*`, `email.inbound_url`, `email.misconfigured.*`, `email.clock`, `email.inbound_recent`); `fail` on the e2e domain's missing MX, `warn` on the http `app.url`, exit 1 |

### Sending domains

The release binary sends customer mail through the `smtp` mailer to Mailpit, so a domain is set up for a relay we sign
for: an ownership TXT record and a signing key TXT record. Checks read real DNS unless
`DIMMI_CHANNELS__EMAIL__SENDING__RESOLVERS` points at a server you run; with no records published a domain stays
`verifying`, which is what the rows below expect. `ED=$B/email-domains`.

| Flow | Command | Expect |
|---|---|---|
| Add | `r ed -b a.jar -X POST -H "$O" -H "$J" $ED -d '{"domain":"acme-'$RUN'.test"}'; EDM=$(jq -r .data.id body)` | 201 `email_domain`, `status: pending`, `driver: smtp`, records `ownership`, `dkim` (TXT `v=DKIM1; k=rsa; p=…`) and `dmarc`, `host_guide.key` `other` |
| Refused names | the same with `gmail.com`, `com`, `inbound.e2e.test`, then the first domain again | 422 `invalid_domain` three times, then 409 `domain_exists` |
| Zone file | `curl -s -b a.jar -H "$O" "$ED/$EDM?format=bind"` | `text/plain`, one `3600 IN TXT` line per record, the key split into quoted strings of 255 bytes |
| Check | `r chk -b a.jar -X POST -H "$O" $ED/$EDM/checks`, again at once | 202 `status: verifying`, then 429 `domain_check_throttled` with `Retry-After` |
| Check landed | `r show -b a.jar -H "$O" $ED/$EDM` after a few seconds | `last_checked_at` set, `next_check_at` a minute later, records `missing`, warning `dmarc_missing` with its message |
| Return path and host | `r upd -b a.jar -X PATCH -H "$O" -H "$J" $ED/$EDM -d '{"dns_host":"cloudflare"}'` | 200, `provider_hint: cloudflare`, record `host` values without the domain |
| New key | `r rot -b a.jar -X POST -H "$O" $ED/$EDM/dkim-rotations` as the owner, then as an admin | 201 with a `dkim_next` record, then 403 |
| Automatic setup | `r dc -b a.jar -X POST -H "$O" $ED/$EDM/dns-setups` | 422 `dns_setup_unavailable` (needs the `ses` driver, `domain_connect.enabled` and a host that knows the template); the domain shows `dns_setup: null` |
| Host sends back | open `$W/d/settings/email/domains/$EDM?error=access_denied`, then `?dns_setup=applied` | "Your DNS host did not add the records"; then the domain starts checking |
| Remove | `r del -b a.jar -X DELETE -H "$O" $ED/$EDM` | 204; `GET` then 404 |
| Doctor | `bin/dimmi-cloud email:doctor acme-$RUN.test` in `$E2E` with its env | `[ok]` relay line, `[info]` no mail server of its own, `[fail]` no MX for `inbound.e2e.test` (exit 1), `[warn]` DMARC missing |
| In a browser | open `$W/d/settings/email`, add a domain | the domain page with its records, Copy buttons, the host's steps, "Check now", and "Remove domain" |

### Signatures

`ES=$B/email-signatures`; `$EMA` is an address id from `GET $B/email-addresses`.

| Flow | Command | Expect |
|---|---|---|
| Own | `r sig -b a.jar -X POST -H "$O" -H "$J" $ES -d '{"body":"Thanks,\n**{{member_name}}**"}'; SIG=$(jq -r .data.id body)` | 201 `email_signature`, `kind: personal`, `member_id` the caller, `email_address_id: null` |
| Twice | the same again | 409 `signature_exists` |
| Unknown placeholder | the same with `{"body":"{{nickname}}"}` | 422 `unknown_placeholder`, `placeholders: ["{{nickname}}"]`, `errors.body` |
| Team's | `r team -b a.jar -X POST -H "$O" -H "$J" $ES -d '{"kind":"address","email_address_id":"'$EMA'","body":"The {{address_name}} team"}'`, then as a member | 201 `kind: address`, `member_id: null`; then 403 |
| List | `curl -s -b a.jar -H "$O" "$ES?filter[member]=me"`, `?filter[kind]=address`, `?filter[kind]=other` | one each, then 400 `invalid_filter` |
| Preview | `r prev -b a.jar -X POST -H "$O" -H "$J" $ES/renderings -d '{"body":"**{{member_name}}**\n{{member_title}}","email_address_id":"'$EMA'"}'` | 201 `email_signature_rendering`; `html` has the name in `<strong>`, the title line gone; `text` starts `-- ` |
| Reply | reply to a conversation on `$EMA` in the inbox, open it in Mailpit | the text ends `-- ` and the signature, above the quote; with `{"email":{"signature":false}}` none |
| Off and gone | `PATCH $ES/$SIG` `{"enabled":false}`, then `DELETE $ES/$SIG` | 200 `enabled: false`, then 204; the next reply carries the team's |
| In a browser | open `$W/d/settings/profile`, then `$W/d/settings/email` | "Email signature" with a live preview; "Team signatures" with one editor per address and "Teammates' signatures" by name, both absent for a member |

### Reply style and default address

`EA=$B/email-addresses/$EMA`; `$EMB` is a second address id.

| Flow | Command | Expect |
|---|---|---|
| Preview | `r tpl -b a.jar -X POST -H "$O" -H "$J" $EA/template-renderings -d '{"template":"branded","brand_color":"#ff5500","footer_markdown":"**Acme**"}'` | 201 `email_template_rendering`; `html` has a table, `border-top:2px solid #ff5500`, `<strong>Acme</strong>`, no `<img` |
| Bad color | the same with `{"brand_color":"red"}` | 422 |
| Member | the preview as a member | 403 |
| Branded | `PATCH $EA` `{"settings":{"template":"branded","brand_color":"#FF5500","footer_markdown":"**Acme**"}}`, reply in the inbox, open it in Mailpit | 200, `brand_color: "#ff5500"`; the mail has the colored rule and footer, its text ends with the footer |
| Plain again | `PATCH $EA` `{"settings":{"template":"plain"}}`, reply | the plain look, no table |
| Default | `PATCH $B/email-addresses/$EMB` `{"is_default":true}`, then `GET $EA` | 200 `is_default: true`; `$EMA` no longer the default and listed second |
| Default off | `PATCH $B/email-addresses/$EMB` `{"is_default":false}` | 422 `default_address_required` |
| In a browser | open `$W/d/settings/email` as the owner | each card has "Reply style" with a preview; "Make default" on all but the default, which shows a "Default" chip |

## Realtime connections end with their credential

Open a socket with a browser's cookie (`a.jar`) and one with a token, using the SDK or `websocat`, and follow a conversation on each.

| Flow | Try | Expect |
|---|---|---|
| Sign out | `DELETE $B/sessions/current` with `a.jar` | the cookie socket closes with 4009; connecting again with that cookie answers 101 `unauthorized`; the token socket stays |
| Session ended elsewhere | sign in again in `b.jar`; `DELETE $B/sessions/{id of a.jar}` with `b.jar` | only `a.jar`'s socket closes |
| Everywhere else | `DELETE $B/sessions` with `b.jar`, confirmed | every socket of the other sessions closes |
| Token revoked | `DELETE $B/tokens/{id}` | only that token's socket closes |
| Password reset | redeem a reset link | every socket of the user closes, and every token answers 401 |
| Taken back | magic link, reset or `claim: true` on an unverified account | every socket of the stranger closes |
| Runs out or lost | delete the session row by hand or let a token expire | the socket closes with 3005 at its next subscribe, or within `DIMMI_BROADCASTING__RECHECK` (one minute) on its own |

## Conversation access

Restrict a teammate with `PATCH $B/members/{mem}`, then act as them in a browser and with their token.

| Flow | Request or action | Expect |
|---|---|---|
| Restrict to teams | `PATCH $B/members/{mem}` `{"conversation_access":"teams","access_team_ids":["team_..."]}` | 200 with the fields; `member.access_changed` in the audit log; their sockets close with 4009 and reconnect |
| Own change | same on `/members/me` | 403 `forbidden` |
| Owner target | same on an owner | 422 `owner_access_unrestricted` |
| Team list unusable | `{"conversation_access":"assigned","access_team_ids":["team_..."]}` | 422 `access_teams_not_used`; an unknown team 422 `team_not_found` |
| Hidden conversation | `GET $B/conversations/{id}`, `/conversations/{number}`, `/messages`, `/neighbors`, `/read-state` as them | 404 `not_found` on each |
| Lists and counts | `GET $B/conversations?filter[view]=all`, `GET $B/conversation-counts`, `GET $B/search?q=%23{number}` | hidden conversation absent, not counted, `jump` null |
| Token | their token on the same requests | same answers as their browser |
| List channel | subscribe `workspace_conversations:{ws}` as them | refused; copies arrive on `member:{mem}` for visible conversations only |
| Reassigned away | owner moves a conversation they can see | `conversation.hidden` on `member:{mem}`, their `conversation:` subscription ends, open tab shows the no-access page |
| Sidebar | sign in as them | no Unassigned, unless `conversation_access_unassigned` |

## Availability and automatic assignment

Make a team of two teammates the default team of an inbox, then follow conversations a customer starts.

| Flow | Request or action | Expect |
|---|---|---|
| Go away | as a teammate `PATCH $B/members/me` `{"availability":"away"}` | 200 `availability: away`; `member.availability_changed` on `workspace:{ws}`; avatar dot turns away |
| Reassign while online | `{"availability":"online","away_reassign":true}` | 422 `away_reassign_requires_away` |
| Capacity by admin | owner `PATCH $B/members/{mem}` `{"conversation_capacity":2}`; same as a member on someone else | 200 with the limit and an audit entry; 403 `forbidden` |
| Inbox settings | `PATCH $B/inboxes/{inbox}` `{"default_team_id":"team_...","settings":{"assignment":{"mode":"round_robin"}}}` | 200 with `default_team_id` and the merged settings; settings left out stay |
| Bad settings | `{"settings":{"reopen_window_days":400}}`, `{"settings":{"assignment":{"overflow_team_id":"<the default team>"}}}` | 422 `invalid_inbox_setting`; 422 `overflow_team_is_default` |
| Round robin | start four conversations through the API | turns alternate between the two teammates |
| Nobody available | set both away, start one | unassigned, `assignment_wait.reason` `no_available_member`, one `conversation.assignment_waiting` timeline row |
| Drain | one teammate comes back | the waiting conversation is assigned within a second |
| Status | `GET $B/inboxes/{inbox}/assignment-status?channel_type=api` | each member with `open_count`, `eligible`, `why_not`; `waiting.count`; a member without `settings:read` 403 |
| Waiting list | `GET $B/conversations?filter[view]=all&filter[assignment_waiting]=true` | only the waiting ones |
| Delete inbox | `DELETE $B/inboxes/{inbox}` holding conversations; then `?move_to=inbox_...`; the default inbox | 422 `move_to_required`; 204 and its conversations show the target inbox; 422 `default_inbox` |
| Seen receipt | widget conversation; subscribe `widget_contact:ctc_...`; `PATCH $B/conversations/{id}/read-state` `{}`; then `PATCH $B/workspace` `{"seen_receipt_shows_name":true}` and write again | `conversation.seen_by_team` with `conversation_id`, `seen_at`, no member id or name; after the switch, a second receipt after 5 s carries `seen_by.name` |

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
lock, sign-out, revocation, `password.reset`, `magic_link.used`, `sessions.revoked_others`, `token.created` and
`token.revoked`; one-time tokens with
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

## Own account

A signed-in browser session in `a.jar`, `$O`, `$J`, `$B`, `r`, `link` and `frag` as above. A person's own account
needs no workspace header.

| Flow | Command | Expect |
|---|---|---|
| Read profile | `r user -b a.jar $B/user` | 200 `user`, `avatar_url` null, `timezone` null |
| Rename | `r rename -b a.jar -X PATCH -H "$O" -H "$J" $B/user -d '{"name":"Ada L"}'` | 200, `name` "Ada L"; `user.updated` on each workspace channel |
| Time zone | same route with `{"timezone":"Asia/Tokyo"}`, `{"timezone":"Mars/Base"}`, `{"timezone":null}` | 200 set; 422 `errors.timezone`; 200 null |
| Language | same route with `{"locale":"en"}`, `{"locale":"fr"}` (not in `app.locales`), `{"locale":null}` | 200 `locale` "en"; 422 `errors.locale`; 200 `locale` null |
| Resolved language | `r me -b a.jar -H "Dimmi-Workspace: $WS" $B/me` after `{"locale":null}` | 200, `user.locale` null, `locale` the workspace's language |
| Messages follow the person | start the server with `DIMMI_APP__LOCALES=en,en-XA` after `make pseudo-locale build`, `PATCH /user {"locale":"en-XA"}`, then `{"timezone":"Mars/Base"}` with `Accept-Language: en` | 422 with an accented `errors.timezone` message and `Content-Language: en-XA`; `/auth/config` lists `en-XA` second |
| Upload picture | `r pic -b a.jar -X PUT -H "$O" -F avatar=@logo.png $B/user/avatar`, then `curl -I $HOST$(jq -r .data.avatar_url pic.json)` | 200 with `avatar_url` `/avatars/<uuid>.png`; 200 `image/png`, `immutable`, `nosniff` |
| Refuse non-picture | same with a text file | 422 `invalid_avatar` |
| Remove picture | `r nopic -b a.jar -X DELETE -H "$O" $B/user/avatar` | 204; the old address 404 once the worker ran |
| Upload workspace logo | `r logo -b a.jar -X PUT -H "$O" -H "Dimmi-Workspace: $WS" -F logo=@logo.png $B/workspace/logo`, then `curl -I $HOST$(jq -r .data.logo_url logo.json)` | 200 workspace with `logo_url` `/avatars/<uuid>.png`; 200 `image/png` |
| Remove workspace logo | `r nologo -b a.jar -X DELETE -H "$O" -H "Dimmi-Workspace: $WS" $B/workspace/logo` | 204; `GET $B/workspace` shows `logo_url` null; a member gets 403 |
| Upload a reply file | `r file -b a.jar -X POST -H "$O" -H "Dimmi-Workspace: $WS" -H "Content-Type: image/png" --data-binary @shot.png "$B/attachments?filename=shot.png"`, then `curl -sI "$(jq -r .data.url file.json)"` | 201 `attachment` with `url` under `/files/local/…`; the link answers 200 `image/png`, and 404 once its signature is changed |
| Reply with a file | `r sent -b a.jar -X POST -H "$O" -H "Dimmi-Workspace: $WS" -d '{"body":"Here","attachment_ids":["'$(jq -r .data.id file.json)'"]}' $B/conversations/$CONV/replies` on a messenger conversation | 201; the timeline shows the file under `attachments`; the same id again 409 `attachment_not_uploaded`; on an API-channel conversation 403 `attachments_disabled` |
| Change password | `r pw -b a.jar -c a.jar -X PATCH -H "$O" -H "$J" $B/user/password -d '{"current_password":"...","password":"..."}'` | 200 `tokens_remaining`; new session cookie; another browser's `me` 401; tokens still work; mail "Your Dimmi password changed" |
| Wrong password | same with a wrong `current_password` | 422 `invalid_password` on `current_password` |
| Ask to move | `r mv -b a.jar -X POST -H "$O" -H "$J" $B/email-changes -d "{\"email\":\"moved.$RUN@example.com\"}"` | 201 `email_change` `pending`, masked; mails "Confirm your new" to the new address and "Someone asked" to the old |
| Bad link | `L=$(link moved.$RUN@example.com Confirm); r mvbad -X PATCH -H "$O" -H "$J" $B/email-changes/$(frag $L id) -d '{"token":"nope"}'` | 422 `invalid_email_change_token` |
| Confirm move | same with `$(frag $L token)`, then again | 200 `confirmed` with the full address, mail "email address changed" to the old; second 422 |
| Preferences | `r pref -b a.jar -X PATCH -H "$O" -H "$J" $B/user -d '{"preferences":{"send_with_enter":true}}'`, then `{"preferences":{"send_with_enter":null}}` | 200 with `preferences.send_with_enter` true, then false; `me.user.preferences` the same |
| Unknown preference | same with `{"preferences":{"autoplay":true}}` or `{"send_with_enter":"yes"}` | 422 on `preferences.autoplay`, `preferences.send_with_enter` |
| Name customers see | `r cfn -b a.jar -X PATCH -H "$O" -H "$J" $B/workspace -d '{"customer_facing_name":"Acme Support"}'`, then `null` | 200 `customer_facing_name` "Acme Support", then null; `workspace.updated` in the audit log once |
| Preview deletion | `r prev -b a.jar $B/user/deletion-preview` | 200 `blocking` with workspaces it alone owns with teammates, `will_be_deleted` with those it is alone in |
| Blocked deletion | `r del -b a.jar -X DELETE -H "$O" -H "$J" $B/user -d "{\"confirm_email\":\"$ADA\"}"` with teammates present | 409 `sole_owner`, `workspaces` |
| Delete account | same once nothing blocks | 204, cookie cleared; `me` 401; tokens 401; mail "account was deleted"; signing up with the address again works as a new user; old member rows show "Former teammate" |

Browser: `/d/settings/profile`, `/d/settings/security`, `/d/settings/preferences`, `/d/settings/workspace` and
`/email/confirm` from the mailed link, in light and dark. Every control on them does something; there is no
Notifications page in settings yet. The Language rows on Preferences and on the workspace page show only when
`app.locales` lists more than one language; picking one changes the page's text and `<html lang>` without a reload.

## Sign-in methods and sign-in help

Start the server with Google pointed at a local OpenID Connect provider, as `tests/e2e/server.ts` does:
`DIMMI_AUTH__OAUTH__GOOGLE__CLIENT_ID`, `..._CLIENT_SECRET`, `DIMMI_SOCIALITE__PROVIDERS__GOOGLE__ISSUER` and
`DIMMI_HTTP_CLIENT__ALLOW_HOSTS` set to the provider's host. `a.jar` is the owner, `$WS` its workspace id, `$M`
a teammate's `mem_` id whose account has two-factor on, `t.jar` the teammate's browser.

| Flow | Command | Expect |
|---|---|---|
| List sign-in methods | `r ids -b a.jar $B/user/identities`; `r cfg $B/auth/config` | 200 linked accounts; `providers` lists `google` |
| Link a provider | open `$W/auth/google/redirect?intent=link` in the signed-in browser, approve at the provider | lands on `/d/settings/security?linked=google`; a toast says Google is linked; mail "linked" |
| Unlink the last way in | `r un -b a.jar -X DELETE -H "$O" $B/user/identities/<idn_>` on an account with no password and mail sign-in off | 409 `last_login_method`; the page says "Set a password first" |
| Start a reset | `r rst -b a.jar -X POST -H "$O" -H "$J" -H "Dimmi-Workspace: $WS" $B/members/$M/two-factor-resets` | 201 `two_factor_reset` `pending`; mail "Reset two-factor" to the teammate; audit `member.two_factor_reset_requested` |
| Refused resets | same for an owner as an admin; for a teammate without two-factor; for one who also works in another organization's workspace; a fourth time in a day | 403; 409 `two_factor_not_enabled`; 409 `two_factor_reset_blocked`; 429 |
| Finish a reset | `L=$(link <teammate> "Reset two-factor"); r fin -X PATCH -H "$O" -H "$J" $B/two-factor-resets/$(frag $L id) -d "{\"token\":\"$(frag $L token)\"}"`, then with `"password"` | 422 `password_required`; then 200 `completed`; `t.jar` `me` 401; mail "was reset"; audit `member.two_factor_reset_completed` |
| Clear a sign-in lock | fail the teammate's password until 429, `r lock -b a.jar -H "$O" -H "Dimmi-Workspace: $WS" $B/members/$M`, then `-X DELETE $B/members/$M/sign-in-lock` | `sign_in_locked: true`; 204; signing in works again; audit `member.sign_in_lock_cleared` |

Browser: the security page's "Sign-in methods" section, the member menu's "Help with two-factor" and "Clear
sign-in lock", and `/two-factor-reset` from the mailed link, in light and dark.

## Signed-in messenger visitors

`a.jar` is the owner, `$WS` its workspace id, `$APP` the messenger's `wgt_` id and `$W` the install's address.
`sign SUB` stands for a host server minting an HS256 token with the secret below: header `{"alg":"HS256","kid":"$KID"}`,
claims `{"sub":"SUB","iat":now,"exp":now+600}`, key the whole `dmi_idsec_` string. Widget calls send
`-H "Origin: $W"`.

| Flow | Command | Expect |
|---|---|---|
| Create a secret | `r isk -b a.jar -X POST -H "$O" -H "$J" -H "Dimmi-Workspace: $WS" $B/identity-secrets -d '{"name":"Production","algorithm":"HS256"}'` | 201 with `secret` `dmi_idsec_` and 43 characters, `kid` `isk_`; listing it again shows no `secret` |
| No secret yet | on a fresh workspace, `POST $W/widget/v1/sessions` with `{"app_id":"$APP","user_jwt":"a.b.c"}` | 401 `identity_not_configured` with a `hint` |
| Sign in | `POST $W/widget/v1/sessions` with `{"app_id":"$APP","user_jwt":"$(sign user_42)"}` | 201 `kind` `identified`, `identity.status` `verified`; the contact is a user with external id `user_42` |
| Hand over a lead | start an anonymous session, write a message, then sign in with `previous_token` | 201; same contact, now a user; the old token answers 401 `session_revoked` |
| Same customer again | sign in with `previous_token` of the signed-in session | 200, same `id`, no `token` |
| Refresh | `PATCH $W/widget/v1/session` with the bearer and `{"user_jwt":"$(sign user_42)"}`; then with `$(sign user_7)` | 200 and `expires_at` moved on; then 409 `identity_mismatch` |
| Resume without proof | `GET $W/widget/v1/session` with a signed-in bearer | 409 `identity_proof_required` |
| Bad tokens | wrong key, no `sub`, `exp` past, `exp` in milliseconds, unknown `kid`, foreign `aud` | 401 `token_signature_invalid`, `token_sub_missing`, `token_expired`, `token_lifetime_too_long`, `token_kid_unknown`, `token_audience_mismatch`, each with a `hint`; the session sent as `previous_token` still works |
| Nonce | the same token with a `nonce` claim twice | 201, then 401 `token_replayed` |
| Rotate | `POST $B/identity-secrets/$KID/rotations` | 201 new secret; the old one `retiring` and still signs in for 7 days, then 401 `token_secret_revoked` |
| Revoke | `PATCH $B/identity-secrets/$KID` with `{"status":"revoked"}` | 200; every session it signed in answers 401 `session_revoked` |
| Messenger | page with `data-identity-url` answering `{"token":"<jwt>"}` for a signed-in user | the messenger signs in on load; signing out on the site sends `DELETE /widget/v1/session` on the next load and shows none of the previous conversations |
| Agent actions | a published action binding `{contact.external_id}` on the widget channel; ask the signed-in messenger about the account | the action runs with the token's `sub`, never a value the model chose |

## Reporting

Write down every response that differs from the table, with the command and the body. A difference is a bug in
the code or in this table; fix whichever is wrong and add an integration test for it when it is the code.

## API rate limits

Start the server with `DIMMI_API__LIMITS__TOKEN_PER_MINUTE=12`, `..._WORKSPACE_PER_MINUTE=4`,
`..._USER_PER_MINUTE=1200` and `..._BAD_CREDENTIAL_PER_MINUTE=3`. `$T` is a token with `members:read`.

| Flow | Command | Expect |
|---|---|---|
| Headers | `curl -i -H "Authorization: Bearer $T" $B/members` | 200 with `X-RateLimit-Limit: 12`, `X-RateLimit-Remaining`, `X-RateLimit-Scope: token` |
| Burst | the same three times within ten seconds | third 429 `too_many_requests`, `X-RateLimit-Scope: burst`, `Retry-After`, `X-RateLimit-Reset` |
| Workspace share | two tokens of one workspace, five calls between them in a minute | the fifth 429 with scope `workspace`; the dashboard in a browser keeps working |
| Bad tokens | `curl -H "Authorization: Bearer dmi_live_nope" $B/members` four times | three 401, then 429 |
| Limits shown | `r ent -b a.jar -H "$O" -H "Dimmi-Workspace: $WS" $B/workspace/entitlements` | 200 `api.token_per_minute` 12; the tokens page says "12 requests a minute per token, 4 for the workspace's tokens together." |
| Metric | scrape `/metrics` on the admin address | `dimmi_api_throttled_total{scope="burst"}` counts the refusals |

