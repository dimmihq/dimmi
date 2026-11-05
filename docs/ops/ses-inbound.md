# Receiving email with Amazon SES

SES accepts mail for the inbound domain, stores each message in S3 and notifies an SNS topic, which posts to
dimmi. dimmi reads the raw message from the bucket. Nothing in dimmi listens for SMTP.

## AWS resources

| Resource | Setting |
|---|---|
| SES identity | the inbound domain, verified, in a region that receives mail |
| MX record | `inbound-smtp.<region>.amazonaws.com`, priority 10, on the inbound domain |
| Receipt rule set | one rule set, active. AWS has no API call in CloudFormation to activate it, so activate it once by hand or with a custom resource |
| Receipt rule | recipient: the inbound domain; actions: S3 to the inbound bucket under `raw/`, with the SNS topic on the S3 action; spam and virus scanning on; TLS optional (senders without TLS still deliver) |
| SES message encryption | off: dimmi cannot read client-side encrypted objects. Use bucket encryption instead |
| S3 bucket | public access blocked, default encryption on, lifecycle rule deleting `raw/` after 31 days |
| Bucket policy | allow `ses.amazonaws.com` `s3:PutObject` on `raw/*`, conditioned on `aws:SourceAccount` and `aws:SourceArn` of the receipt rule |
| SNS topic | HTTPS subscription to `https://<app.url>/webhooks/ses/inbound`; dimmi confirms it on its own |
| IAM for dimmi | `s3:GetObject` and `s3:DeleteObject` on `raw/*` (delete serves erasure requests) |

## dimmi settings

```bash
DIMMI_CHANNELS__EMAIL__INBOUND_DOMAIN=inbound.example.com
DIMMI_CHANNELS__EMAIL__INBOUND__DRIVERS=ses
DIMMI_MAILBOX__SOURCES__SES__DRIVER=ses
DIMMI_MAILBOX__SOURCES__SES__TOPIC_ARNS=arn:aws:sns:eu-west-1:<account>:<topic>
DIMMI_FILESYSTEMS__DISKS__SES_INBOUND__DRIVER=s3
DIMMI_FILESYSTEMS__DISKS__SES_INBOUND__BUCKET=<inbound bucket>
DIMMI_FILESYSTEMS__DISKS__SES_INBOUND__REGION=eu-west-1
```

dimmi accepts notifications only from the listed topics, verifies every SNS signature against a certificate
fetched from an SNS host, and confirms the subscription itself. An unsubscribe notification is logged and shows up in
`dimmi email:inbound-doctor`.

## Sending through the same account

Outbound mail uses two configuration sets so a paused workspace never blocks system mail: `dimmi-system` for mail
to dimmi users (sign-in codes, resets) and `dimmi-customer` for mail to customers. Use SES shared IP pools only;
dimmi never asks for dedicated IPs. On the cloud each workspace sends under its own SES tenant.

## Checks after a change

1. `dig MX inbound.example.com` shows the SES host.
2. `dimmi email:inbound-doctor` passes `email.inbound_mx` and `email.inbound_source.ses`.
3. Send a mail to a forwarding address; the address's activity log shows it within seconds.
4. In S3 the object appears under `raw/`; after 31 days the lifecycle rule removes it.

When a retention hold is introduced, replace the 31-day rule with expiry on the tag `dimmi-hold=false`, so held
objects survive.
