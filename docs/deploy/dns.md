# DNS at Vercel

Every record for `trydimmi.com`, staging's and production's, lives in Vercel DNS. There is no Route 53 zone and no NS
delegation. [`deploy/dns`](../../deploy/dns) computes an environment's records from its stack outputs and writes them.

## The token

**HUMAN, once, 5 minutes.** In Vercel, Account Settings, Tokens, create a token:

- Scope: the team that owns `trydimmi.com` (or your personal account if it owns the domain). Vercel tokens are scoped to
  an account or team, not to single domains, so treat this one as able to change the whole team.
- Expiration: 90 days, and put a reminder to rotate it.
- Keep it in the password manager. It never goes in the repository or in GitHub: only people run `apply`.

If the domain belongs to a team, also note the team id (Team Settings, General) as `VERCEL_TEAM_ID`.

## Stack outputs

The script reads the file `cdk deploy --outputs-file ../dns/outputs/<env>.json` writes. `outputs/` is ignored by git. To
rebuild it without deploying:

```sh
cd deploy/dns
for stack in app mail; do
  aws cloudformation describe-stacks --region us-east-2 --stack-name "dimmi-staging-$stack" \
    --query 'Stacks[0].{n:StackName,o:Outputs}' --output json
done | node -e '
  let s = ""; process.stdin.on("data", (c) => (s += c)).on("end", () => {
    const out = {};
    for (const part of s.trim().split(/\n(?=\{)/)) {
      const { n, o } = JSON.parse(part);
      out[n] = Object.fromEntries(o.map((x) => [x.OutputKey, x.OutputValue]));
    }
    process.stdout.write(JSON.stringify(out, null, 2));
  });' > outputs/staging.json
```

## Plan, apply, verify

From `deploy/dns`, with `VERCEL_TOKEN` (and `VERCEL_TEAM_ID`) exported:

| Step | Who | Command | Expect | Time |
|---|---|---|---|---|
| Plan | HUMAN | `pnpm dns plan --env staging --outputs outputs/staging.json` | One line per change, then a summary | Seconds |
| Apply | HUMAN | `pnpm dns apply --env staging --outputs outputs/staging.json` | The same changes, then `Applied` | Seconds |
| Verify | AUTOMATED | `pnpm dns verify --env staging --outputs outputs/staging.json` | `ok` per record, exit 0 | Minutes, up to the 5-minute TTL |

`apply` is idempotent: a second run reports `0 to create, 0 to update, 0 to delete`. Run `plan` after any change to the
app or mail stacks; a new load balancer means new addresses.

### Lines in a plan

- `+ create`, `~ update`, `- delete`: what `apply` will do. It deletes only records carrying this environment's comment.
- `= exists`: an identical record someone added by hand. Left alone; delete it at Vercel if the script should own it.
- `! blocked`: a record the script does not own clashes, such as a CNAME at `staging` or a second SPF record. `apply`
  refuses to change anything until it is removed or renamed by a person at Vercel.

## Ownership

Every record the script writes carries the comment `managed by dimmi deploy/dns for <env>; edits are overwritten`. Do not
edit those records by hand; change the stacks or the config instead. Records without that comment, such as the marketing
site's or other teams', are never touched.

## Why A records

The host gets one A record per load balancer Elastic IP. Production's host is the zone apex, where a CNAME is not allowed.
Vercel offers ALIAS records that would follow the load balancer's name, but the Elastic IPs never change while the load
balancer exists, so plain A records work at the apex and everywhere else with nothing to resolve on Vercel's side.

## Certificates after DNS

Once `verify` passes, the binary orders certificates on the first HTTPS request, over the TLS-ALPN or HTTP challenge
passed through by the load balancer. Check with `curl -sv https://staging.trydimmi.com/ready`. Let's Encrypt allows five
certificates per exact name per week and 50 per registered domain, which staging and production share; certificates are
stored in PostgreSQL so restarts and deploys never order again.

## Mail records

Each mail domain (`inbound.*` and `mail.*`) gets an MX to SES receiving, `v=spf1 include:amazonses.com -all`, a
`p=reject` DMARC policy, a `bounce.` subdomain with its own MX and SPF for MAIL FROM, and three Easy DKIM CNAMEs. SES marks
the identities verified within about an hour of `verify` passing. None of these names is a website.
