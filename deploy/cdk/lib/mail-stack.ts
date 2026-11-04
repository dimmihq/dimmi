import { CfnOutput, Duration, Fn, Stack, Tags } from "aws-cdk-lib";
import type { StackProps } from "aws-cdk-lib";
import { Alarm, ComparisonOperator, Metric, TreatMissingData } from "aws-cdk-lib/aws-cloudwatch";
import { SnsAction } from "aws-cdk-lib/aws-cloudwatch-actions";
import { ManagedPolicy, PolicyStatement, ServicePrincipal } from "aws-cdk-lib/aws-iam";
import { CnameRecord, HostedZone, MxRecord, TxtRecord } from "aws-cdk-lib/aws-route53";
import type { IHostedZone } from "aws-cdk-lib/aws-route53";
import { BlockPublicAccess, Bucket, BucketEncryption } from "aws-cdk-lib/aws-s3";
import {
  ConfigurationSet,
  DkimIdentity,
  EmailIdentity,
  EmailSendingEvent,
  EventDestination,
  Identity,
  MailFromBehaviorOnMxFailure,
  ReceiptRuleSet,
  TlsPolicy,
} from "aws-cdk-lib/aws-ses";
import type { IReceiptRuleAction, ReceiptRuleActionConfig } from "aws-cdk-lib/aws-ses";
import { BackoffFunction, Subscription, SubscriptionProtocol, Topic } from "aws-cdk-lib/aws-sns";
import { EmailSubscription } from "aws-cdk-lib/aws-sns-subscriptions";
import { Queue, QueueEncryption } from "aws-cdk-lib/aws-sqs";
import type { Construct } from "constructs";
import { ActivateRuleSet } from "./mail/activate-rule-set.ts";

export type MailStackProps = StackProps & {
  /** Names every resource and tag, so stages in one account stay apart. */
  stage: string;
  /** The domain SES receives on and sends default mail from. */
  inboundDomain: string;
  /** Origin of the web host; the webhook paths are appended to it. */
  webhookBaseUrl: string;
  /** Who is mailed when an alarm fires. */
  opsEmail: string;
  /** Route 53 zone to write the records in. Without it they become outputs. */
  hostedZone?: { id: string; name: string };
};

type MailRecord =
  | { kind: "MX"; name: string; value: string }
  | { kind: "CNAME"; name: string; value: string }
  | { kind: "TXT"; name: string; value: string };

const RAW_PREFIX = "raw/";
const RAW_RETENTION_DAYS = 31;

/**
 * The events the application acts on. Opens and clicks are absent because
 * mail is never tracked.
 */
const SENDING_EVENTS = [
  EmailSendingEvent.SEND,
  EmailSendingEvent.REJECT,
  EmailSendingEvent.BOUNCE,
  EmailSendingEvent.COMPLAINT,
  EmailSendingEvent.DELIVERY,
  EmailSendingEvent.RENDERING_FAILURE,
  EmailSendingEvent.DELIVERY_DELAY,
];

/**
 * Mail in and out for one stage. It deploys alone and knows nothing about what
 * runs the application: webhooks point at a configured origin, and the
 * application's permissions are a managed policy for whoever runs it to attach.
 * The stack holds no secret.
 */
export class MailStack extends Stack {
  constructor(scope: Construct, id: string, props: MailStackProps) {
    super(scope, id, props);

    Tags.of(this).add("Environment", props.stage);

    const name = (suffix: string) => `dimmi-${props.stage}-${suffix}`;
    const ops = this.opsTopic(name("ops"), props.opsEmail);

    const ruleSetName = name("inbound");
    const bucket = this.inboundBucket(ruleSetName);
    const inboundTopic = this.inboundTopic(name("inbound"), ruleSetName);
    const setNames = [`dimmi-system-${props.stage}`, `dimmi-customer-${props.stage}`];
    const eventsTopic = new Topic(this, "EventsTopic", { topicName: name("events") });

    this.receiving(ruleSetName, props.inboundDomain, bucket, inboundTopic);
    this.webhooks(name("webhook-dlq"), props.webhookBaseUrl, { inbound: inboundTopic, events: eventsTopic });

    const identity = this.sendingIdentity(props.inboundDomain);
    this.configurationSets(setNames, eventsTopic);

    const policy = this.applicationPolicy(name("mail-application"), setNames, bucket);

    this.alarms(name, ops, { bucket, topics: [inboundTopic, eventsTopic] });
    this.publishRecords(this.records(props.inboundDomain, identity), props.hostedZone);

    new CfnOutput(this, "InboundBucketName", { value: bucket.bucketName });
    new CfnOutput(this, "InboundTopicArn", { value: inboundTopic.topicArn });
    new CfnOutput(this, "EventsTopicArn", { value: eventsTopic.topicArn });
    new CfnOutput(this, "OpsTopicArn", { value: ops.topicArn });
    new CfnOutput(this, "ReceiptRuleSetName", { value: ruleSetName });
    new CfnOutput(this, "SystemConfigurationSetName", { value: setNames[0] });
    new CfnOutput(this, "CustomerConfigurationSetName", { value: setNames[1] });
    new CfnOutput(this, "MailApplicationPolicyArn", { value: policy.managedPolicyArn });
  }

  private ruleSetArn(ruleSetName: string): string {
    return this.formatArn({ service: "ses", resource: "receipt-rule-set", resourceName: `${ruleSetName}:receipt-rule/*` });
  }

  private opsTopic(topicName: string, email: string): Topic {
    const topic = new Topic(this, "OpsTopic", { topicName });
    topic.addSubscription(new EmailSubscription(email));
    topic.addToResourcePolicy(
      new PolicyStatement({
        actions: ["sns:Publish"],
        principals: [new ServicePrincipal("cloudwatch.amazonaws.com")],
        resources: [topic.topicArn],
        conditions: { StringEquals: { "aws:SourceAccount": this.account } },
      }),
    );

    return topic;
  }

  /**
   * SES may only write under the raw prefix, and only for this account's own
   * rules. Mail is encrypted by the bucket rather than by SES, whose client-side
   * encryption the receiving driver cannot read.
   */
  private inboundBucket(ruleSetName: string): Bucket {
    const bucket = new Bucket(this, "InboundBucket", {
      blockPublicAccess: BlockPublicAccess.BLOCK_ALL,
      encryption: BucketEncryption.S3_MANAGED,
      enforceSSL: true,
      lifecycleRules: [{ prefix: RAW_PREFIX, expiration: Duration.days(RAW_RETENTION_DAYS) }],
      metrics: [{ id: "raw", prefix: RAW_PREFIX }],
    });

    bucket.addToResourcePolicy(
      new PolicyStatement({
        actions: ["s3:PutObject"],
        principals: [new ServicePrincipal("ses.amazonaws.com")],
        resources: [bucket.arnForObjects(`${RAW_PREFIX}*`)],
        conditions: {
          StringEquals: { "aws:SourceAccount": this.account },
          ArnLike: { "aws:SourceArn": this.ruleSetArn(ruleSetName) },
        },
      }),
    );

    return bucket;
  }

  private inboundTopic(topicName: string, ruleSetName: string): Topic {
    const topic = new Topic(this, "InboundTopic", { topicName });
    topic.addToResourcePolicy(
      new PolicyStatement({
        actions: ["sns:Publish"],
        principals: [new ServicePrincipal("ses.amazonaws.com")],
        resources: [topic.topicArn],
        conditions: {
          StringEquals: { "aws:SourceAccount": this.account },
          ArnLike: { "aws:SourceArn": this.ruleSetArn(ruleSetName) },
        },
      }),
    );

    return topic;
  }

  private receiving(ruleSetName: string, domain: string, bucket: Bucket, topic: Topic): void {
    const ruleSet = new ReceiptRuleSet(this, "ReceiptRuleSet", { receiptRuleSetName: ruleSetName });
    const store: IReceiptRuleAction = {
      bind: (): ReceiptRuleActionConfig => ({
        s3Action: { bucketName: bucket.bucketName, objectKeyPrefix: RAW_PREFIX, topicArn: topic.topicArn },
      }),
    };

    const rule = ruleSet.addRule("Inbound", {
      receiptRuleName: ruleSetName,
      recipients: [domain],
      scanEnabled: true,
      tlsPolicy: TlsPolicy.OPTIONAL,
      actions: [store],
    });

    if (bucket.policy) {
      rule.node.addDependency(bucket.policy);
    }

    rule.node.addDependency(topic);

    const activation = new ActivateRuleSet(this, "ActivateRuleSet", ruleSetName);
    activation.node.addDependency(rule);
  }

  /**
   * SNS retries a failing endpoint for just under its limit of an hour. What
   * still fails lands in one queue, kept for the longest time SQS allows, so an
   * outage of the application loses nothing.
   */
  private webhooks(queueName: string, baseUrl: string, topics: { inbound: Topic; events: Topic }): void {
    const deadLetterQueue = new Queue(this, "WebhookDeadLetterQueue", {
      queueName,
      retentionPeriod: Duration.days(14),
      encryption: QueueEncryption.SQS_MANAGED,
      enforceSSL: true,
    });

    const deliveryPolicy = {
      healthyRetryPolicy: {
        minDelayTarget: Duration.seconds(30),
        maxDelayTarget: Duration.seconds(60),
        numRetries: 61,
        numNoDelayRetries: 0,
        numMinDelayRetries: 2,
        numMaxDelayRetries: 54,
        backoffFunction: BackoffFunction.EXPONENTIAL,
      },
    };

    const origin = baseUrl.replace(/\/+$/, "");
    const subscribe = (id: string, topic: Topic, path: string) =>
      new Subscription(this, id, {
        topic,
        protocol: SubscriptionProtocol.HTTPS,
        endpoint: `${origin}${path}`,
        deadLetterQueue,
        deliveryPolicy,
      });

    subscribe("InboundWebhook", topics.inbound, "/webhooks/ses/inbound");
    subscribe("EventsWebhook", topics.events, "/webhooks/ses/events");
  }

  private sendingIdentity(domain: string): EmailIdentity {
    return new EmailIdentity(this, "InboundIdentity", {
      identity: Identity.domain(domain),
      dkimIdentity: DkimIdentity.easyDkim(),
      dkimSigning: true,
      mailFromDomain: `bounce.${domain}`,
      mailFromBehaviorOnMxFailure: MailFromBehaviorOnMxFailure.REJECT_MESSAGE,
    });
  }

  private configurationSets(names: string[], topic: Topic): void {
    for (const [index, configurationSetName] of names.entries()) {
      const id = index === 0 ? "SystemConfigurationSet" : "CustomerConfigurationSet";
      const set = new ConfigurationSet(this, id, { configurationSetName, reputationMetrics: true });
      set.addEventDestination("Events", {
        destination: EventDestination.snsTopic(topic),
        events: SENDING_EVENTS,
      });
    }
  }

  /**
   * What the running application may do. It is a customer-managed policy, not
   * a role, because whatever runs the application owns its own role and attaches
   * this. Raw mail can be read and erased, never written: only SES writes it.
   */
  private applicationPolicy(managedPolicyName: string, setNames: string[], bucket: Bucket): ManagedPolicy {
    const ses = (resource: string, resourceName: string) => this.formatArn({ service: "ses", resource, resourceName });
    const identities = ses("identity", "*");
    const tenants = ses("tenant", "*");

    return new ManagedPolicy(this, "MailApplicationPolicy", {
      managedPolicyName,
      statements: [
        new PolicyStatement({
          sid: "Send",
          actions: ["ses:SendEmail", "ses:SendRawEmail"],
          resources: [identities, tenants, ...setNames.map((name) => ses("configuration-set", name))],
        }),
        new PolicyStatement({
          sid: "ManageIdentities",
          actions: [
            "ses:CreateEmailIdentity",
            "ses:DeleteEmailIdentity",
            "ses:GetEmailIdentity",
            "ses:PutEmailIdentityDkimAttributes",
            "ses:PutEmailIdentityDkimSigningAttributes",
            "ses:PutEmailIdentityFeedbackAttributes",
            "ses:PutEmailIdentityMailFromAttributes",
          ],
          resources: [identities],
        }),
        new PolicyStatement({
          sid: "ManageTenants",
          actions: [
            "ses:CreateTenant",
            "ses:DeleteTenant",
            "ses:GetTenant",
            "ses:CreateTenantResourceAssociation",
            "ses:DeleteTenantResourceAssociation",
            "ses:ListTenantResources",
          ],
          resources: [tenants, identities, ...setNames.map((name) => ses("configuration-set", name))],
        }),
        new PolicyStatement({
          sid: "ReadAndEraseRawMail",
          actions: ["s3:GetObject", "s3:DeleteObject"],
          resources: [bucket.arnForObjects(`${RAW_PREFIX}*`)],
        }),
      ],
    });
  }

  /**
   * Reputation metrics are silent when nothing is sent, which is healthy.
   * Received objects are the opposite: silence means mail stopped arriving, so
   * missing data breaches. The object count S3 publishes is daily, too coarse for
   * six hours, so that alarm reads request metrics on the raw prefix.
   */
  private alarms(name: (suffix: string) => string, ops: Topic, on: { bucket: Bucket; topics: Topic[] }): void {
    const action = new SnsAction(ops);
    const hour = Duration.hours(1);
    const ses = (metricName: string) =>
      new Metric({ namespace: "AWS/SES", metricName, statistic: "Maximum", period: hour });

    const alarms = [
      new Alarm(this, "BounceRateAlarm", {
        alarmName: name("ses-bounce-rate"),
        metric: ses("Reputation.BounceRate"),
        threshold: 0.05,
        evaluationPeriods: 1,
        comparisonOperator: ComparisonOperator.GREATER_THAN_THRESHOLD,
        treatMissingData: TreatMissingData.NOT_BREACHING,
      }),
      new Alarm(this, "ComplaintRateAlarm", {
        alarmName: name("ses-complaint-rate"),
        metric: ses("Reputation.ComplaintRate"),
        threshold: 0.001,
        evaluationPeriods: 1,
        comparisonOperator: ComparisonOperator.GREATER_THAN_THRESHOLD,
        treatMissingData: TreatMissingData.NOT_BREACHING,
      }),
      new Alarm(this, "InboundSilenceAlarm", {
        alarmName: name("inbound-silence"),
        metric: new Metric({
          namespace: "AWS/S3",
          metricName: "PutRequests",
          dimensionsMap: { BucketName: on.bucket.bucketName, FilterId: "raw" },
          statistic: "Sum",
          period: hour,
        }),
        threshold: 1,
        evaluationPeriods: 6,
        comparisonOperator: ComparisonOperator.LESS_THAN_THRESHOLD,
        treatMissingData: TreatMissingData.BREACHING,
      }),
      ...on.topics.map(
        (topic, index) =>
          new Alarm(this, `WebhookFailureAlarm${index}`, {
            alarmName: name(`webhook-failures-${index === 0 ? "inbound" : "events"}`),
            metric: topic.metricNumberOfNotificationsFailed({ statistic: "Sum", period: Duration.minutes(5) }),
            threshold: 0,
            evaluationPeriods: 1,
            comparisonOperator: ComparisonOperator.GREATER_THAN_THRESHOLD,
            treatMissingData: TreatMissingData.NOT_BREACHING,
          }),
      ),
    ];

    for (const alarm of alarms) {
      alarm.addAlarmAction(action);
    }
  }

  /**
   * Every DNS name the stage needs. All of them are mail only: none is a website
   * and none may sit behind a proxy.
   */
  private records(domain: string, identity: EmailIdentity): MailRecord[] {
    const mailFrom = `bounce.${domain}`;

    return [
      { kind: "MX", name: domain, value: `inbound-smtp.${this.region}.amazonaws.com` },
      { kind: "TXT", name: domain, value: "v=spf1 include:amazonses.com -all" },
      { kind: "TXT", name: `_dmarc.${domain}`, value: "v=DMARC1; p=reject;" },
      { kind: "MX", name: mailFrom, value: `feedback-smtp.${this.region}.amazonses.com` },
      { kind: "TXT", name: mailFrom, value: "v=spf1 include:amazonses.com -all" },
      ...identity.dkimRecords.map((record): MailRecord => ({ kind: "CNAME", name: record.name, value: record.value })),
    ];
  }

  private publishRecords(records: MailRecord[], hostedZone: MailStackProps["hostedZone"]): void {
    if (!hostedZone) {
      records.forEach((record, index) => {
        const priority = record.kind === "MX" ? "10 " : "";
        new CfnOutput(this, `DnsRecord${index}`, {
          value: Fn.join(" ", [record.name, record.kind, `${priority}${record.value}`]),
        });
      });

      return;
    }

    if (!records[0].name.endsWith(`.${hostedZone.name}`)) {
      throw new Error(`${records[0].name} is not inside the zone ${hostedZone.name}`);
    }

    const zone = HostedZone.fromHostedZoneAttributes(this, "Zone", {
      hostedZoneId: hostedZone.id,
      zoneName: hostedZone.name,
    });

    records.forEach((record, index) => this.writeRecord(zone, `DnsRecord${index}`, record));
  }

  private writeRecord(zone: IHostedZone, id: string, record: MailRecord): void {
    const recordName = `${record.name}.`;

    switch (record.kind) {
      case "MX":
        new MxRecord(this, id, { zone, recordName, values: [{ priority: 10, hostName: record.value }] });
        break;
      case "CNAME":
        new CnameRecord(this, id, { zone, recordName, domainName: record.value });
        break;
      case "TXT":
        new TxtRecord(this, id, { zone, recordName, values: [record.value] });
        break;
    }
  }
}
