# Forwarding your support address to dimmi

Every address in dimmi has a forwarding address, shown in Settings, Email, such as `acme@inbound.trydimmi.com`.
Forward your public address (`support@acme.com`) to it and keep a copy in your own mailbox. Every guide ends the
same way: press "Send test email" and wait for "Receiving".

Forward, do not alias: an alias or a group that rewrites the sender makes every customer look like the same person.

## Gmail (personal account)

1. Open Gmail settings, [Forwarding and POP/IMAP](https://mail.google.com/mail/u/0/#settings/fwdandpop), and add
   the forwarding address.
2. Gmail sends a confirmation code. dimmi receives it and shows it in Settings, Email; paste it into Gmail.
3. Choose "Forward a copy of incoming mail" and keep Gmail's copy.

Replies you send from dimmi do not appear in Gmail's Sent folder. Connect the mailbox instead of forwarding to
see them there.

## Google Workspace (recommended: a routing rule)

A routing rule needs an administrator but no confirmation code, keeps the original sender, leaves DKIM intact,
and keeps working when staff change. It works for a user, an alias or a group address.

1. Admin console, Apps, Google Workspace, Gmail, [Routing](https://admin.google.com/ac/apps/gmail/routing),
   Configure. Name it `dimmi support@acme.com`.
2. Messages to affect: Inbound and Internal - receiving.
3. Envelope filter: Only affect specific envelope recipients, Single recipient, `support@acme.com`.
4. Also deliver to: Add more recipients, Add, Basic, the forwarding address.
5. Account types to affect: Users, Groups, Unrecognized / Catch-all.
6. Save. Google says a rule can take up to 24 hours; usually it is minutes, and the test stays red until then.

Without an administrator, the Gmail steps above work unless your administrator blocked forwarding. Use "Send to
my IT admin" in Settings, Email to hand them these steps.

### When support@ is a Google Group

A group rewrites the sender to "Jane via Support", adds list headers and may hold mail for moderation. Keep the
group for people if you like, add the routing rule above on the group's address with "Groups" ticked, and remove
the forwarding address from the group's members.

## Microsoft 365

Two things block forwarding by default:

- The outbound anti-spam policy refuses automatic forwarding outside the company. The sender then gets a bounce
  with `5.7.520`. In Microsoft Defender, Anti-spam, the outbound policy, set Automatic forwarding to On, scoped to
  this mailbox.
- A shared mailbox can only be forwarded by an administrator, and an Outlook rule needs a licensed user.

Then: Exchange admin center, Recipients, [Mailboxes](https://admin.exchange.microsoft.com/#/mailboxes), the
mailbox, Mail flow settings, Email forwarding, Forward to an external SMTP address, the forwarding address, and
keep a copy. Tenants that forbid changing the forwarding policy can use a transport rule that adds the forwarding
address as Bcc instead.

## Outlook.com

Settings, Mail, Forwarding, Enable forwarding, the forwarding address, and tick "Keep a copy of forwarded
messages".

## Fastmail

Settings, Filters and Rules, add a rule for mail to `support@acme.com` with the action "Send a copy to" the
forwarding address, keeping the message in your mailbox.

## iCloud Mail

iCloud forwards a whole mailbox only, not a single address: Settings, Mail, Forwarding, "Forward my email to" the
forwarding address. Leave "Delete messages after forwarding" off.

## Zoho Mail

Settings, Mail Forwarding and POP/IMAP, Add forwarding address, then confirm with the code dimmi shows in
Settings, Email, and keep a copy in Zoho.

## cPanel hosting

Email, Forwarders, Add Forwarder: address `support`, domain `acme.com`, destination the forwarding address. Keep
the mailbox as well, so a copy stays with you.

## Cloudflare Email Routing

Email, Email Routing, Destination addresses: add the forwarding address and confirm it. Then Routing rules: custom
address `support@acme.com`, action "Send to an email", destination the forwarding address.

## Any other host

Find mail forwarding in your host's control panel, forward `support@acme.com` to the forwarding address, and keep
a copy.

## The Sent folder

Forwarding only brings mail in. Replies written in dimmi leave from dimmi and do not appear in your mail
provider's Sent folder, and replies you write in your own mail app do not appear in dimmi. Connect the mailbox
(Google, Microsoft or IMAP) when both sides should show every message.
