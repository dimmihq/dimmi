# dimmi DNS

Writes every DNS record an environment needs into Vercel DNS, where all of `trydimmi.com` lives. No Route 53, no NS
delegation. The runbook step that uses it is in [`docs/deploy/dns.md`](../../docs/deploy/dns.md).

It reads the environment's stack outputs, computes the records, and compares them with the zone at Vercel:

| Command                                          | Does                                                                               | Needs          |
| ------------------------------------------------ | ---------------------------------------------------------------------------------- | -------------- |
| `pnpm dns plan --env staging --outputs <file>`   | Prints what apply would change. Changes nothing                                    | `VERCEL_TOKEN` |
| `pnpm dns apply --env staging --outputs <file>`  | Creates and updates the environment's records, deletes the ones it no longer needs | `VERCEL_TOKEN` |
| `pnpm dns verify --env staging --outputs <file>` | Looks each record up through public resolvers and says which are visible           | nothing        |

`VERCEL_TEAM_ID` is needed too when the domain belongs to a Vercel team.

## Ownership

Every record it writes carries the comment `managed by dimmi deploy/dns for <env>; edits are overwritten`. It changes
and deletes only records with its own environment's comment. A record someone added by hand that is identical to one it
needs is left alone and reported. A record that would clash (anything beside a CNAME, a second A or MX at the host, a
second SPF) blocks `apply` before it changes anything.

## Records

- The host: one A record per load balancer Elastic IP. A records work at the zone apex, where production's
  `trydimmi.com` sits and a CNAME is not allowed. An ALIAS to the NLB name would also work at Vercel, but the addresses
  never change while the load balancer lives, so plain A records keep the moving parts down.
- Each mail domain: MX to SES receiving, SPF, DMARC, the `bounce.` MAIL FROM MX and SPF, three Easy DKIM CNAMEs.

Node 24 or newer. Tests run with `pnpm test` against a fake Vercel API and a fake resolver.
