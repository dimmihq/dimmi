# SES production access

**HUMAN, once per environment's Region, 15 minutes to send, about a day for AWS to answer.**

A new Region starts in the SES sandbox: mail goes only to verified addresses and the mailbox simulator, at most 200
messages a day and one a second. Receiving is not limited. Access is granted per Region, so staging (us-east-2) and
production (us-east-1) each need their own request.

## Before you ask

- `pnpm dns verify` passes and both identities show `Verified` in the SES console of that Region.
- The host serves the site over HTTPS. Production's reviewer may look at it; staging's is behind the gate, which is fine.
- The `dimmi-<env>-ses-bounce-rate` and `-complaint-rate` alarms exist and the ops subscription is confirmed.

## Where

SES console in the Region, Account dashboard, Request production access. Or:

```sh
aws sesv2 put-account-details --region us-east-1 \
  --production-access-enabled --mail-type TRANSACTIONAL \
  --website-url https://trydimmi.com --contact-language EN \
  --use-case-description file://use-case.txt \
  --additional-contact-email-addresses ops@trydimmi.com
```

## Draft wording

Edit before sending; describe only what is true on the day.

> dimmi is a customer support inbox for software companies. Each customer workspace connects its own support address;
> customers' end users write to it, and agents reply from the dimmi dashboard. We send two kinds of mail:
>
> 1. System mail from mail.trydimmi.com: sign-up confirmation, sign-in links, password resets, invitations and
>    notifications to people who use dimmi. Only people who signed up or were invited by a workspace member receive it.
> 2. Replies to support conversations from inbound.trydimmi.com and from customers' own verified domains. Every
>    recipient wrote to the workspace first; we never send to purchased or scraped lists and send no marketing.
>
> Bounces and complaints arrive through an SES configuration set and SNS. A hard bounce or complaint adds the address to
> our suppression list and stops all further mail to it; the workspace owner sees why. Each workspace has sending limits,
> and we pause a workspace whose bounce or complaint rate rises. CloudWatch alarms notify our operations address at a 5%
> bounce or 0.1% complaint rate. Mail carries no tracking pixels or rewritten links.
>
> We expect under 1,000 messages a day at launch [staging: under 100, sent only to our own test mailboxes].

The sentences about per-workspace limits and pausing describe features the email channel builds before launch; leave
them out if they are not live when you send this.

## After

AWS answers by mail within about a day; a refusal says what to add. Check: the account dashboard shows `Production
access` and the new quota. If AWS asks for more, answer in the same case rather than opening a new one.
