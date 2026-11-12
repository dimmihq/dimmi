# Email with Amazon SES

dimmi sends two kinds of mail and receives one:

- **System mail** (sign-in links, invitations, notifications), from an
  address such as `no-reply@mail.example.com`.
- **Customer mail** (replies to customers, broadcasts), from the inbound
  domain or from a workspace's own verified domain.
- **Incoming support mail**, delivered to the **inbound domain**, such as
  `inbound.example.com`. Every address there belongs to dimmi; a team
  forwards `support@theircompany.com` to its inbound address.

Use subdomains, never the root domain: the inbound domain's MX record sends
all of its mail to SES.

Everything below can be done in the AWS console or with the AWS CLI. The
commands assume the CLI is signed in to the user's account with admin rights,
and these shell variables:

```bash
REGION=us-east-1                  # a Region with SES receiving: us-east-1, us-west-2, eu-west-1, ...
ACCOUNT=$(aws sts get-caller-identity --query Account --output text)
APP=support.example.com           # the app's domain
SYSTEM=mail.example.com           # sends system mail
INBOUND=inbound.example.com       # receives support mail, sends customer mail
BUCKET=dimmi-inbound-$ACCOUNT     # raw incoming mail
```

If the user has no AWS CLI, give them the console path for each step instead.
Never ask them to paste AWS secret keys into the chat; have them run
`dimmictl set` with the keys themselves, or pipe them in yourself without
echoing them.

## 1. Sending domains

Verify both domains with Easy DKIM and a custom MAIL FROM, so mail passes
SPF, DKIM and DMARC:

```bash
for D in $SYSTEM $INBOUND; do
  aws sesv2 create-email-identity --region $REGION --email-identity $D
  aws sesv2 put-email-identity-mail-from-attributes --region $REGION \
    --email-identity $D --mail-from-domain bounce.$D --behavior-on-mx-failure REJECT_MESSAGE
  aws sesv2 get-email-identity --region $REGION --email-identity $D \
    --query 'DkimAttributes.Tokens' --output text
done
```

Console: SES > Identities > Create identity > Domain, Easy DKIM RSA 2048,
then "Custom MAIL FROM domain" `bounce.<domain>`.

Then add these DNS records for **each** domain `D` (the three DKIM tokens are
the ones printed above):

| Type  | Name                          | Value                                      |
|-------|-------------------------------|--------------------------------------------|
| CNAME | `<token>._domainkey.D` (x3)   | `<token>.dkim.amazonses.com`               |
| MX    | `bounce.D`                    | `10 feedback-smtp.REGION.amazonses.com`    |
| TXT   | `bounce.D`                    | `v=spf1 include:amazonses.com -all`        |
| TXT   | `D`                           | `v=spf1 include:amazonses.com -all`        |
| TXT   | `_dmarc.D`                    | `v=DMARC1; p=quarantine;`                  |

Wait until both identities show "Verified" (`aws sesv2 get-email-identity ...
--query VerifiedForSendingStatus`), usually minutes.

## 2. Production access

New SES accounts are in the sandbox: they only send to verified addresses,
200 a day. Request production access in SES > Account dashboard > "Request
production access" (or `aws sesv2 put-account-details`). Say it is
transactional support mail: replies to customers who wrote in, sign-in links
and invitations, with bounces and complaints handled. AWS answers within a
day. The install works in the sandbox meanwhile, for verified addresses only.

## 3. Receiving

Incoming mail is stored in S3 by SES, which then tells dimmi through SNS.

```bash
# The bucket, private, raw mail kept 30 days.
aws s3api create-bucket --region $REGION --bucket $BUCKET \
  $( [ $REGION = us-east-1 ] || echo --create-bucket-configuration LocationConstraint=$REGION )
aws s3api put-public-access-block --bucket $BUCKET --public-access-block-configuration \
  BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true
aws s3api put-bucket-lifecycle-configuration --bucket $BUCKET --lifecycle-configuration \
  '{"Rules":[{"ID":"raw","Status":"Enabled","Filter":{"Prefix":"raw/"},"Expiration":{"Days":30}}]}'
aws s3api put-bucket-policy --bucket $BUCKET --policy "{\"Version\":\"2012-10-17\",\"Statement\":[{
  \"Effect\":\"Allow\",\"Principal\":{\"Service\":\"ses.amazonaws.com\"},\"Action\":\"s3:PutObject\",
  \"Resource\":\"arn:aws:s3:::$BUCKET/raw/*\",
  \"Condition\":{\"StringEquals\":{\"aws:SourceAccount\":\"$ACCOUNT\"}}}]}"

# The topic SES publishes to.
TOPIC=$(aws sns create-topic --region $REGION --name dimmi-inbound --query TopicArn --output text)
aws sns set-topic-attributes --region $REGION --topic-arn $TOPIC --attribute-name Policy \
  --attribute-value "{\"Version\":\"2012-10-17\",\"Statement\":[{\"Effect\":\"Allow\",
  \"Principal\":{\"Service\":\"ses.amazonaws.com\"},\"Action\":\"sns:Publish\",\"Resource\":\"$TOPIC\",
  \"Condition\":{\"StringEquals\":{\"aws:SourceAccount\":\"$ACCOUNT\"}}}]}"

# The rule: mail for the inbound domain goes to the bucket, then the topic.
aws ses create-receipt-rule-set --region $REGION --rule-set-name dimmi
aws ses create-receipt-rule --region $REGION --rule-set-name dimmi --rule "{
  \"Name\":\"dimmi-inbound\",\"Enabled\":true,\"ScanEnabled\":true,\"TlsPolicy\":\"Optional\",
  \"Recipients\":[\"$INBOUND\"],
  \"Actions\":[{\"S3Action\":{\"BucketName\":\"$BUCKET\",\"ObjectKeyPrefix\":\"raw/\",\"TopicArn\":\"$TOPIC\"}}]}"
aws ses set-active-receipt-rule-set --region $REGION --rule-set-name dimmi
```

Only one receipt rule set is active per Region. If the account already has an
active one, add the rule to it instead of activating `dimmi`.

DNS for receiving, on the inbound domain:

| Type | Name       | Value                                    |
|------|------------|------------------------------------------|
| MX   | `INBOUND`  | `10 inbound-smtp.REGION.amazonaws.com`   |

Subscribe dimmi to the topic **after** the dimmi settings in step 5 are in
place, since dimmi confirms the subscription itself when it arrives:

```bash
aws sns subscribe --region $REGION --topic-arn $TOPIC --protocol https \
  --notification-endpoint https://$APP/webhooks/ses/inbound
aws sns list-subscriptions-by-topic --region $REGION --topic-arn $TOPIC   # not PendingConfirmation
```

## 4. An IAM user for dimmi

One user with only what dimmi needs: send mail, manage the sending identities
of workspaces' own domains, and read and delete raw incoming mail.

```bash
aws iam create-user --user-name dimmi
aws iam put-user-policy --user-name dimmi --policy-name dimmi-mail --policy-document "{
 \"Version\":\"2012-10-17\",\"Statement\":[
  {\"Sid\":\"Send\",\"Effect\":\"Allow\",\"Action\":[\"ses:SendEmail\",\"ses:SendRawEmail\"],
   \"Resource\":[\"arn:aws:ses:$REGION:$ACCOUNT:identity/*\",\"arn:aws:ses:$REGION:$ACCOUNT:configuration-set/*\"]},
  {\"Sid\":\"Domains\",\"Effect\":\"Allow\",\"Action\":[\"ses:CreateEmailIdentity\",\"ses:DeleteEmailIdentity\",
   \"ses:GetEmailIdentity\",\"ses:PutEmailIdentityDkimAttributes\",\"ses:PutEmailIdentityDkimSigningAttributes\",
   \"ses:PutEmailIdentityFeedbackAttributes\",\"ses:PutEmailIdentityMailFromAttributes\"],
   \"Resource\":\"arn:aws:ses:$REGION:$ACCOUNT:identity/*\"},
  {\"Sid\":\"RawMail\",\"Effect\":\"Allow\",\"Action\":[\"s3:GetObject\",\"s3:DeleteObject\"],
   \"Resource\":\"arn:aws:s3:::$BUCKET/raw/*\"}]}"
aws iam create-access-key --user-name dimmi
```

The last command prints the access key once. Its values go straight into
step 5; do not echo them into the chat.

## 5. dimmi settings

On the server (replace the values; keys on stdin keep them out of history):

```bash
dimmictl set <<EOF
DIMMI_MAIL__FROM__NAME=Acme Support
DIMMI_MAIL__FROM__EMAIL=no-reply@$SYSTEM
DIMMI_MAIL__MAILERS__SYSTEM__DRIVER=ses
DIMMI_MAIL__MAILERS__SYSTEM__REGION=$REGION
DIMMI_MAIL__MAILERS__SYSTEM__ACCESS_KEY_ID=AKIA...
DIMMI_MAIL__MAILERS__SYSTEM__SECRET_ACCESS_KEY=...
DIMMI_MAIL__MAILERS__CUSTOMER__DRIVER=ses
DIMMI_MAIL__MAILERS__CUSTOMER__REGION=$REGION
DIMMI_MAIL__MAILERS__CUSTOMER__ACCESS_KEY_ID=AKIA...
DIMMI_MAIL__MAILERS__CUSTOMER__SECRET_ACCESS_KEY=...
DIMMI_CHANNELS__EMAIL__INBOUND_DOMAIN=$INBOUND
DIMMI_CHANNELS__EMAIL__INBOUND__DRIVERS=ses
DIMMI_MAILBOX__DEFAULT=ses
DIMMI_MAILBOX__SOURCES__SES__DRIVER=ses
DIMMI_MAILBOX__SOURCES__SES__TOPIC_ARNS=$TOPIC
DIMMI_FILESYSTEMS__DISKS__SES_INBOUND__DRIVER=s3
DIMMI_FILESYSTEMS__DISKS__SES_INBOUND__BUCKET=$BUCKET
DIMMI_FILESYSTEMS__DISKS__SES_INBOUND__REGION=$REGION
DIMMI_FILESYSTEMS__DISKS__SES_INBOUND__ACCESS_KEY_ID=AKIA...
DIMMI_FILESYSTEMS__DISKS__SES_INBOUND__SECRET_ACCESS_KEY=...
EOF
```

Notes:

- `TOPIC_ARNS` is required: dimmi refuses every notification from a topic
  not listed there.
- The customer mailer is optional; without it, customer mail goes through
  the system mailer.
- To use SES configuration sets (for reputation metrics), create them and add
  `DIMMI_MAIL__MAILERS__<NAME>__CONFIGURATION_SET=<name>`.

Then subscribe the topic (end of step 3).

## 6. Check it

- Invite a teammate from the app: the invitation should arrive, and the
  message headers should show `dkim=pass` and `spf=pass`.
- Send an email to the workspace's own address at the inbound domain, shown
  in the app's email settings (for a workspace `acme`, along the lines of
  `acme@inbound.example.com`); it should appear in the inbox within a minute.
  If not: `dimmictl logs web | grep -i ses`, and check the SNS subscription is
  confirmed and the receipt rule set is active.
- The app's own checks: `docker compose -f /opt/dimmi/compose.yaml exec web
  /dimmi email:doctor` and `... /dimmi email:inbound-doctor`.
