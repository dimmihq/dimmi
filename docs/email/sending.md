# Sending replies from your own address

Replies leave dimmi from the forwarding address (`acme@inbound.trydimmi.com`) until you prove you own the domain
of your public address. Once the domain is verified, replies come from `support@acme.com` itself, signed for your
domain, and pass SPF, DKIM and DMARC in the customer's mail client.

## Verify your domain

1. Settings, Email, Sending domains: add `acme.com`.
2. dimmi lists the DNS records to add: one TXT record proving ownership, the DKIM records, and a return path for
   bounces. Each record has a copy button, and the host guide shows where your DNS provider keeps them.
3. Add them at your DNS provider and press "Check now". Records turn green as they are found; most providers take
   a few minutes.
4. When every record is green, the domain shows "Verified" and each address on it shows "Sends as
   support@acme.com".

Keep an SPF record you already have: dimmi proposes one merged record rather than a second one, since two SPF
records make both fail. If a check finds a mistake, such as the domain written twice in a record name, it says
which record and how to fix it.

dimmi keeps checking a verified domain. If a record disappears, replies fall back to the forwarding address and
the domain page says why, so customers keep getting answers while you fix it.

From the command line, `dimmi email:doctor acme.com` explains in plain words what is missing.

## The default address

New conversations started in dimmi leave from the default address, marked "Default" in Settings, Email. Replies
leave from the address the conversation came in on. To change the default, press "Make default" on another
address; there is always exactly one.

## Signatures

A reply closes with the first signature that fits:

1. the teammate's own signature for that address,
2. the teammate's own signature for every address,
3. the address's team signature.

Replies the agent writes, and those of a teammate who has left, carry only the address's team signature. Each
teammate writes theirs under Profile, "Email signature"; admins write the team signature of each address, and can
fix or turn off a teammate's, under Settings, Email.

Signatures use bold, italic, links and line breaks, and these placeholders:

| Placeholder | Filled with |
|---|---|
| `{{member_name}}` | the teammate's name |
| `{{member_title}}` | the teammate's title; a line left empty is dropped |
| `{{address_name}}` | the address's display name |
| `{{company_name}}` | the workspace's name |

A reply sent through the API with `{"email": {"signature": false}}` goes out unsigned.

## Reply style

Each address chooses how its replies look, under Settings, Email:

- **Plain** (default): reads as a person wrote it, and lands best in the customer's main inbox.
- **Branded**: your brand color as a line above the reply and a short footer below it.

Both styles carry the same text, signature and quoted message, and a plain text version for clients that show no
HTML. Neither loads images or anything else from a server, so no customer is tracked when they open a reply. The
preview beside the setting shows a sample reply in the style before you save it.
