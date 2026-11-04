# What dimmi's email never does

dimmi handles support mail without anyone running a mail server. These are deliberate, and they hold on the
cloud and on a self-hosted install alike.

- **No mail server of its own.** dimmi never delivers mail straight to a recipient's MX and never looks up MX
  records to send. The `smtp` mail driver only talks to the relay host you configure.
- **No IP addresses to keep warm.** No dedicated IPs, no IP warming, no IP pools. The cloud sends through Amazon
  SES shared pools only.
- **Nothing listening on port 25.** Mail comes in through a provider's webhook over HTTPS (SES, Postmark, Mailgun,
  SendGrid, Resend, Cloudflare) or by reading a connected mailbox (Gmail, Microsoft 365, IMAP). See
  [receiving email without AWS](../self-host/inbound-email.md).
- **No sending credentials per workspace.** A workspace cannot bring its own SMTP host or its own Postmark or SES
  keys. The install's sending provider is set once, in its configuration. The only credentials a workspace keeps
  are those of a mailbox it connects: a Google or Microsoft grant for that mailbox, or an IMAP password that only
  reads. All are sealed at rest, and an IMAP password never sends mail.
- **No tracking.** No open tracking pixels, no click tracking, no rewritten links.

Also left out on purpose: IMAP IDLE (a connected IMAP mailbox is read every minute), importing a mailbox's
history (a connection starts at the moment it is made), S/MIME and PGP, and an HTML email designer.
