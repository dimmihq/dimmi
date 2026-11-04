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
| Delete | `r del -b b.jar -X DELETE -H "$O" -H "$H" $B/teams/$TEAM`, then `GET $B/teams/$TEAM` | 204, then 404; `teams` lists nothing; the row stays with `deleted_at` set for the inbox to sweep |
| Name free again | `POST $B/teams {"name":"Customer care"}` | 201 |
| In a browser | open `$W/d/settings/teams`, create a team, add a member from the picker, rename it, remove the member, go back to all teams and delete it | the team page opens on create; the member count follows each change; delete asks first and returns to the list |

### Audit and security logs

Run right after the members flows: Bob now owns Acme and Ada has left it. `ADAM` is Ada's `mem_` id. Filters use
square brackets, so pass `-g` to stop curl from reading them as a glob.

| Flow | Command | Expect |
|---|---|---|
| Whole log, oldest first | `r log -g -b b.jar -H "$O" -H "$H" "$B/audit-logs?sort=created_at&limit=100"; jq -r '.data[] \| [.event, .actor.via.kind, .subject.label] \| @tsv' body` | 200: `workspace.created`, `member.joined` (no `via`: signing up), `workspace.updated`, `member.role_changed`, `member.deactivated`, `member.reactivated`, `member.role_changed`, `ownership.transferred`, `member.role_changed` (Ada to admin), `member.left`; each with `actor.label` "Ada Lovelace <ada…>", `via.kind: session`, `ip`, `user_agent: curl/…`, `request_id`, `changes` such as `{"role":{"from":"member","to":"admin"}}` |
| By event | `"$B/audit-logs?filter[event]=member.role_changed,ownership.transferred"` | 4 entries, newest first |
| By actor, paged | `"$B/audit-logs?filter[actor]=$ADAM&limit=2"`, then again with `&cursor=` the `meta.next_cursor` | 2 entries and a cursor, then the next 2 older ones |
| By time | `filter[created_after]=2026-01-01T00:00:00Z&filter[created_before]=2099-01-01T00:00:00Z`; then `filter[created_after]=yesterday` | every entry; then 400 `invalid_filter`, `parameter: created_after` |
| One entry | `r show -b b.jar -H "$O" -H "$H" $B/audit-logs/aud_…`, and `aud_01jc0000000000000000000000` | 200 the entry; 404 `not_found` |
| Members cannot read | before the promotion above, Bob (then a member): `GET $B/audit-logs` with `-H "$H"` | 403 `forbidden` |
| Own security log | `r sev -b a.jar -H "$O" "$B/user/security-events?limit=2"` | 200, newest first: `login.succeeded` with `metadata.auth_methods`, `session_id`, no `device_id`; `meta.next_cursor` while more remain |
| Tokens refused | `r sevt -H "Authorization: Bearer dmi_live_x" $B/user/security-events` | 401 `invalid_token` |
| Entries cannot change | as the application's database role (not a superuser): `update audit_logs set event = 'x.y'` | `permission denied for table audit_logs` |
| In a browser | as Bob, open `$W/d/settings/audit-log`, pick "Changed a role" under events, then Ada under actors; open `$W/d/settings/security` | the table narrows to each filter, Older and Newer step through pages; "Recent security activity" lists the latest sign-ins with device and address |

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
| Redaction defaults | `GET $B/redaction-rules` | 200 `object: redaction`, `built_ins.card_numbers: true`, `built_ins.us_ssn: false`, `rules: []`; Bob 403 |
| Redaction rule | `POST $B/redaction-rules {"name":"Accounts","pattern":"ACCT-\\d+"}`, then `{"name":"x","pattern":"("}` | 201 `object: redaction_rule`, `replacement: [redacted]`; then 422 `invalid_pattern`; an eleventh rule 422 `redaction_rule_limit_reached` |
| Redaction on write | `POST $B/conversations` as a contact with body `Card 4242 4242 4242 4242 ACCT-7` | the message body reads `Card •••• •••• •••• 4242 [redacted]`, `redactions` lists `card_numbers` and the rule's id; the database holds no original |
| Redaction switches | `PATCH $B/redaction-rules {"us_ssn":true,"order":[…]}`, then a note `SSN 123-45-6789` | 200 with the new order; the note reads `SSN •••-••-••••`; the audit log has `redaction_settings.updated` |
| Try redaction | `POST $B/redaction-tests {"text":"4242424242424242"}` | 200 `object: redaction_test`, `text: •••• •••• •••• 4242`, one `redactions` entry; nothing stored |
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
| Default address | `r ea -b a.jar -H "$O" $EA; FWD=$(jq -r '.data[0].forwarding_address' body); EMA=$(jq -r '.data[0].id' body)` | 200, one `email_address`, `forwarding_address` `<slug>@inbound.e2e.test`, `is_default: true`, `setup_guides` from `gmail`, `inbound_verified_at: null` |
| Source closed without the secret | `r no -X POST $W/webhooks/test/inbound --data-binary 'Subject: x'` | 401 `unauthenticated` |
| Mail arrives | `mailraw $FWD 'Refund for order 1042' first.$RUN@customer.test '' 'Charged twice.' \| r in -X POST -H "$IN" --data-binary @- "$W/webhooks/test/inbound?to=$FWD"` | 202 `workspaces: 1` |
| Activity log | `r act -b a.jar -H "$O" $EA/$EMA/inbound-emails` | 200, `status: processed`, `classification: normal`, a `conversation` and `message` id; the address now has `inbound_verified_at` |
| Redelivery | the same command with `&id=p1` twice | 202 twice; the activity log holds it once (a different `id` with the same Message-ID is stored `ignored`, `duplicate`) |
| Reply threads | `mailraw $FWD 'Re: Refund for order 1042' reply.$RUN@customer.test "In-Reply-To: <first.$RUN@customer.test>" 'Thanks!' \| r re …` | 202; `select count(*) from conversations` unchanged, the message joins the first conversation |
| Out of office | `mailraw $FWD 'Automatic reply: away' ooo.$RUN@customer.test 'Auto-Submitted: auto-replied' 'Away.' \| r ooo …` | 202; activity `ignore_reason: auto_reply_no_thread`, no new conversation |
| Unknown address | the first mail to `ghost@inbound.e2e.test` | 202 `workspaces: 0`, nothing stored |
| Add an address | `r add -b a.jar -X POST -H "$O" -H "$J" $EA -d '{"address":"sales@acme.test"}'` | 201, `forwarding_address` `<slug>.sales@inbound.e2e.test`; the same again 409 `address_in_use`; `x@inbound.e2e.test` 422 `invalid_address` |
| Settings | `r upd -b a.jar -X PATCH -H "$O" -H "$J" $EA/$EMA -d '{"settings":{"bulk_mail_handling":"drop"}}'` | 200 with `bulk_mail_handling: drop` |
| Forwarding test | `r t -b a.jar -X POST -H "$O" $EA/$EMA/tests` | 201 `email_address_test` `sent_to` the public address; Mailpit holds it with an `X-Dimmi-Test` header; feeding that raw mail back through the source shows `ignore_reason: forwarding_test` |
| Disconnect | `r del -b a.jar -X DELETE -H "$O" $EA/<the sales ema_>` | 204; mail to its forwarding address is stored `ignored` with `address_deleted` |
| In a browser | open `$W/d/settings/email` | the forwarding address with a Copy button, the Gmail steps, "Receiving" once mail arrived, the last emails, "Send test email" and "Add address" |

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

## Reporting

Write down every response that differs from the table, with the command and the body. A difference is a bug in
the code or in this table; fix whichever is wrong and add an integration test for it when it is the code.
