import type { StackProps } from "aws-cdk-lib";
import { CfnOutput, Duration, Fn, Stack } from "aws-cdk-lib";
import { Alarm, ComparisonOperator, Metric, TreatMissingData } from "aws-cdk-lib/aws-cloudwatch";
import { SnsAction } from "aws-cdk-lib/aws-cloudwatch-actions";
import { ManagedPolicy, PolicyStatement, ServicePrincipal, User } from "aws-cdk-lib/aws-iam";
import { BlockPublicAccess, Bucket, BucketEncryption } from "aws-cdk-lib/aws-s3";
import type { IReceiptRuleAction, ReceiptRuleActionConfig } from "aws-cdk-lib/aws-ses";
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
import type { ITopic } from "aws-cdk-lib/aws-sns";
import { BackoffFunction, Subscription, SubscriptionProtocol, Topic } from "aws-cdk-lib/aws-sns";
import { Queue, QueueEncryption } from "aws-cdk-lib/aws-sqs";
import type { Construct } from "constructs";
import type { EnvironmentConfig } from "../config/environment.ts";
import { ActivateRuleSet } from "../constructs/activate-rule-set.ts";

export type MailStackProps = StackProps & {
  config: EnvironmentConfig;
  /** Where alarms publish. */
  alarms: ITopic;
};

/**
 * One DNS record the environment needs. All of them are mail only: none is
 * a website and none may sit behind a proxy. The DNS script writes them.
 */
export type MailRecord = { type: "MX" | "CNAME" | "TXT"; name: string; value: string; priority?: number };

const RAW_PREFIX = "raw/";
const SYSTEM_PREFIX = "system/";
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
 * Mail in and out for one environment, on two domains. The inbound domain
 * receives customer mail, which lands under `raw/` with a notification to
 * the application, and sends from customer addresses. The system domain sends
 * sign-up and notification mail; replies to it are kept under `system/`.
 *
 * Webhooks point at the environment's host. The application runs outside
 * AWS, so its permissions are a managed policy attached to one IAM user whose
 * access key a person creates and stores in the cluster; no key ever passes
 * through CloudFormation. The stack writes no DNS: every record it needs is
 * the `DnsRecords` output, which the DNS script reads.
 */
export class MailStack extends Stack {
  readonly inboundBucket: Bucket;
  readonly inboundTopic: Topic;
  readonly applicationPolicy: ManagedPolicy;
  readonly applicationUser: User;
  readonly systemConfigurationSetName: string;
  readonly customerConfigurationSetName: string;

  constructor(scope: Construct, id: string, props: MailStackProps) {
    super(scope, id, props);

    const { config } = props;
    const name = (suffix: string) => `dimmi-${config.name}-${suffix}`;
    this.systemConfigurationSetName = `dimmi-system-${config.name}`;
    this.customerConfigurationSetName = `dimmi-customer-${config.name}`;
    const setNames = [this.systemConfigurationSetName, this.customerConfigurationSetName];

    const ruleSetName = name("inbound");
    this.inboundBucket = this.createInboundBucket(ruleSetName);
    this.inboundTopic = this.topicFor(name("inbound"), ruleSetName);
    const eventsTopic = new Topic(this, "EventsTopic", { topicName: name("events") });

    this.receiving(ruleSetName, config.mail, this.inboundBucket, this.inboundTopic);
    this.webhooks(name("webhook-dlq"), `https://${config.host}`, { inbound: this.inboundTopic, events: eventsTopic });

    const identities = [
      this.sendingIdentity("InboundIdentity", config.mail.inbound),
      this.sendingIdentity("SystemIdentity", config.mail.system),
    ];
    this.configurationSets(setNames, eventsTopic);

    this.applicationPolicy = this.policy(name("mail-application"), setNames, this.inboundBucket);
    this.applicationUser = new User(this, "ApplicationUser", {
      userName: name("application"),
      managedPolicies: [this.applicationPolicy],
    });

    this.alarms(name, props.alarms, { bucket: this.inboundBucket, topics: [this.inboundTopic, eventsTopic] });

    const records = [
      ...this.records(config.mail.inbound, identities[0]),
      ...this.records(config.mail.system, identities[1]),
    ];

    new CfnOutput(this, "DnsRecords", { value: Fn.toJsonString(records) });
    new CfnOutput(this, "InboundBucketName", { value: this.inboundBucket.bucketName });
    new CfnOutput(this, "InboundTopicArn", { value: this.inboundTopic.topicArn });
    new CfnOutput(this, "EventsTopicArn", { value: eventsTopic.topicArn });
    new CfnOutput(this, "ReceiptRuleSetName", { value: ruleSetName });
    new CfnOutput(this, "SystemConfigurationSetName", { value: setNames[0] });
    new CfnOutput(this, "CustomerConfigurationSetName", { value: setNames[1] });
    new CfnOutput(this, "MailApplicationPolicyArn", { value: this.applicationPolicy.managedPolicyArn });
    new CfnOutput(this, "ApplicationUserName", { value: this.applicationUser.userName });
  }

  private ruleSetArn(ruleSetName: string): string {
    return this.formatArn({
      service: "ses",
      resource: "receipt-rule-set",
      resourceName: `${ruleSetName}:receipt-rule/*`,
    });
  }

  /**
   * SES may only write under the raw and system prefixes, and only for this
   * account's own rules. Mail is encrypted by the bucket rather than by SES, whose client-side
   * encryption the receiving driver cannot read.
   */
  private createInboundBucket(ruleSetName: string): Bucket {
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
        resources: [bucket.arnForObjects(`${RAW_PREFIX}*`), bucket.arnForObjects(`${SYSTEM_PREFIX}*`)],
        conditions: {
          StringEquals: { "aws:SourceAccount": this.account },
          ArnLike: { "aws:SourceArn": this.ruleSetArn(ruleSetName) },
        },
      }),
    );

    return bucket;
  }

  private topicFor(topicName: string, ruleSetName: string): Topic {
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

  /**
   * Two rules in one set, each listing its recipients: a rule without any
   * would match every verified domain in the Region.
   */
  private receiving(ruleSetName: string, mail: EnvironmentConfig["mail"], bucket: Bucket, topic: Topic): void {
    const ruleSet = new ReceiptRuleSet(this, "ReceiptRuleSet", { receiptRuleSetName: ruleSetName });
    const store = (objectKeyPrefix: string, topicArn?: string): IReceiptRuleAction => ({
      bind: (): ReceiptRuleActionConfig => ({ s3Action: { bucketName: bucket.bucketName, objectKeyPrefix, topicArn } }),
    });

    const rules = [
      ruleSet.addRule("Inbound", {
        receiptRuleName: ruleSetName,
        recipients: [mail.inbound],
        scanEnabled: true,
        tlsPolicy: TlsPolicy.OPTIONAL,
        actions: [store(RAW_PREFIX, topic.topicArn)],
      }),
      ruleSet.addRule("System", {
        receiptRuleName: `${ruleSetName}-system`,
        recipients: [mail.system],
        scanEnabled: true,
        tlsPolicy: TlsPolicy.OPTIONAL,
        actions: [store(SYSTEM_PREFIX)],
      }),
    ];

    for (const rule of rules) {
      if (bucket.policy) {
        rule.node.addDependency(bucket.policy);
      }
      rule.node.addDependency(topic);
    }

    const activation = new ActivateRuleSet(this, "ActivateRuleSet", ruleSetName);
    activation.node.addDependency(...rules);
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

  private sendingIdentity(id: string, domain: string): EmailIdentity {
    return new EmailIdentity(this, id, {
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
   * What the running application may do, as a customer-managed policy the
   * application user carries. Raw mail can be read and erased, never written:
   * only SES writes it.
   */
  private policy(managedPolicyName: string, setNames: string[], bucket: Bucket): ManagedPolicy {
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
  private alarms(name: (suffix: string) => string, ops: ITopic, on: { bucket: Bucket; topics: Topic[] }): void {
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
   * The records one mail domain needs: MX to SES receiving, SPF, DMARC,
   * the MAIL FROM subdomain and the three Easy DKIM names.
   */
  private records(domain: string, identity: EmailIdentity): MailRecord[] {
    const mailFrom = `bounce.${domain}`;
    const spf = "v=spf1 include:amazonses.com -all";

    return [
      { type: "MX", name: domain, value: `inbound-smtp.${this.region}.amazonaws.com`, priority: 10 },
      { type: "TXT", name: domain, value: spf },
      { type: "TXT", name: `_dmarc.${domain}`, value: "v=DMARC1; p=reject;" },
      { type: "MX", name: mailFrom, value: `feedback-smtp.${this.region}.amazonses.com`, priority: 10 },
      { type: "TXT", name: mailFrom, value: spf },
      ...identity.dkimRecords.map((record): MailRecord => ({ type: "CNAME", name: record.name, value: record.value })),
    ];
  }
}
