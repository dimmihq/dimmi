import assert from "node:assert/strict";
import { describe, test } from "node:test";
import { App } from "aws-cdk-lib";
import { Match, Template } from "aws-cdk-lib/assertions";
import { MailStack } from "../lib/mail-stack.ts";
import type { MailStackProps } from "../lib/mail-stack.ts";

const STAGING: MailStackProps = {
  env: { region: "us-east-1" },
  stage: "staging",
  inboundDomain: "inbound.staging.trydimmi.com",
  webhookBaseUrl: "https://staging.trydimmi.com",
  opsEmail: "ops@example.com",
};

const ZONE = { id: "ZFIXTUREZONE", name: "staging.trydimmi.com" };

function synth(props: Partial<MailStackProps> = {}): Template {
  return Template.fromStack(new MailStack(new App(), "Mail", { ...STAGING, hostedZone: ZONE, ...props }));
}

type Resource = { Properties: Record<string, unknown> };

function only(template: Template, type: string): Resource {
  const found = Object.values(template.findResources(type)) as Resource[];
  assert.equal(found.length, 1, `expected one ${type}`);

  return found[0];
}

function dnsOutputs(template: Template): number {
  return Object.keys(template.findOutputs("*")).filter((id) => id.startsWith("DnsRecord")).length;
}

describe("mail stack", () => {
  const template = synth();

  test("the receipt rule scans mail and stores it under raw with a notification", () => {
    template.hasResourceProperties("AWS::SES::ReceiptRule", {
      Rule: {
        Enabled: true,
        ScanEnabled: true,
        TlsPolicy: "Optional",
        Recipients: ["inbound.staging.trydimmi.com"],
        Actions: [
          {
            S3Action: {
              BucketName: Match.anyValue(),
              ObjectKeyPrefix: "raw/",
              TopicArn: Match.anyValue(),
              KmsKeyArn: Match.absent(),
            },
          },
        ],
      },
    });
  });

  test("the rule set is made active by a custom resource and cleared on delete", () => {
    const resource = only(template, "Custom::AWS");

    assert.match(String(resource.Properties.Create), /setActiveReceiptRuleSet/);
    assert.match(String(resource.Properties.Create), /dimmi-staging-inbound/);
    assert.doesNotMatch(String(resource.Properties.Delete), /dimmi-staging-inbound/);
  });

  test("raw mail expires after 31 days in a private encrypted bucket", () => {
    template.hasResourceProperties("AWS::S3::Bucket", {
      LifecycleConfiguration: { Rules: [{ Prefix: "raw/", ExpirationInDays: 31, Status: "Enabled" }] },
      BucketEncryption: Match.objectLike({
        ServerSideEncryptionConfiguration: [{ ServerSideEncryptionByDefault: { SSEAlgorithm: "AES256" } }],
      }),
      PublicAccessBlockConfiguration: {
        BlockPublicAcls: true,
        BlockPublicPolicy: true,
        IgnorePublicAcls: true,
        RestrictPublicBuckets: true,
      },
    });
  });

  test("only this account's receipt rules may write to the bucket", () => {
    template.hasResourceProperties("AWS::S3::BucketPolicy", {
      PolicyDocument: {
        Statement: Match.arrayWith([
          Match.objectLike({
            Action: "s3:PutObject",
            Effect: "Allow",
            Principal: { Service: "ses.amazonaws.com" },
            Condition: {
              StringEquals: { "aws:SourceAccount": { Ref: "AWS::AccountId" } },
              ArnLike: { "aws:SourceArn": Match.anyValue() },
            },
          }),
        ]),
      },
    });
  });

  test("both webhooks retry for about an hour and then dead-letter", () => {
    const subscriptions = Object.values(
      template.findResources("AWS::SNS::Subscription", { Properties: { Protocol: "https" } }),
    ) as Resource[];

    assert.deepEqual(subscriptions.map((sub) => sub.Properties.Endpoint).sort(), [
      "https://staging.trydimmi.com/webhooks/ses/events",
      "https://staging.trydimmi.com/webhooks/ses/inbound",
    ]);

    for (const subscription of subscriptions) {
      assert.ok(subscription.Properties.RedrivePolicy, "dead-letter queue");

      const { healthyRetryPolicy: retry } = subscription.Properties.DeliveryPolicy as {
        healthyRetryPolicy: Record<string, number>;
      };
      const backoff = retry.numRetries - retry.numMinDelayRetries - retry.numMaxDelayRetries;
      const longest =
        retry.numMinDelayRetries * retry.minDelayTarget +
        retry.numMaxDelayRetries * retry.maxDelayTarget +
        backoff * retry.maxDelayTarget;
      const shortest =
        retry.numMinDelayRetries * retry.minDelayTarget +
        retry.numMaxDelayRetries * retry.maxDelayTarget +
        backoff * retry.minDelayTarget;

      assert.ok(longest <= 3600, `SNS refuses retry policies over an hour, got ${longest}`);
      assert.ok(shortest >= 3300, `retries stop too early, after ${shortest}`);
    }

    template.resourceCountIs("AWS::SQS::Queue", 1);
  });

  test("the inbound topic takes mail notifications only from this account's rules", () => {
    template.hasResourceProperties("AWS::SNS::TopicPolicy", {
      PolicyDocument: {
        Statement: Match.arrayWith([
          Match.objectLike({
            Principal: { Service: "ses.amazonaws.com" },
            Condition: {
              StringEquals: { "aws:SourceAccount": { Ref: "AWS::AccountId" } },
              ArnLike: { "aws:SourceArn": Match.anyValue() },
            },
          }),
        ]),
      },
    });
  });

  test("the identity signs with easy dkim and rejects on a missing mail from record", () => {
    template.hasResourceProperties("AWS::SES::EmailIdentity", {
      EmailIdentity: "inbound.staging.trydimmi.com",
      DkimAttributes: { SigningEnabled: true },
      DkimSigningAttributes: Match.absent(),
      MailFromAttributes: {
        MailFromDomain: "bounce.inbound.staging.trydimmi.com",
        BehaviorOnMxFailure: "REJECT_MESSAGE",
      },
    });
  });

  test("each configuration set carries the stage and sends events to the topic", () => {
    template.hasResourceProperties("AWS::SES::ConfigurationSet", { Name: "dimmi-system-staging" });
    template.hasResourceProperties("AWS::SES::ConfigurationSet", { Name: "dimmi-customer-staging" });
    template.resourceCountIs("AWS::SES::ConfigurationSetEventDestination", 2);
    template.allResourcesProperties("AWS::SES::ConfigurationSetEventDestination", {
      EventDestination: {
        Enabled: true,
        MatchingEventTypes: Match.arrayWith(["reject", "bounce", "complaint", "delivery"]),
        SnsDestination: { TopicARN: Match.anyValue() },
      },
    });
  });

  test("the configuration sets never ask for opens or clicks", () => {
    const destinations = Object.values(template.findResources("AWS::SES::ConfigurationSetEventDestination"));

    assert.doesNotMatch(JSON.stringify(destinations), /"open"|"click"/);
  });

  test("the application policy sends, manages identities and tenants, and reads raw mail", () => {
    const policy = only(template, "AWS::IAM::ManagedPolicy");
    const statements = (policy.Properties.PolicyDocument as { Statement: { Action: string | string[] }[] }).Statement;
    const actions = statements.flatMap((statement) => statement.Action);

    for (const action of [
      "ses:SendEmail",
      "ses:SendRawEmail",
      "ses:CreateEmailIdentity",
      "ses:CreateTenant",
      "s3:GetObject",
      "s3:DeleteObject",
    ]) {
      assert.ok(actions.includes(action), `missing ${action}`);
    }

    assert.ok(!actions.includes("s3:PutObject"), "only SES writes raw mail");
    assert.equal(policy.Properties.ManagedPolicyName, "dimmi-staging-mail-application");
    template.resourceCountIs("AWS::IAM::User", 0);
    template.hasOutput("MailApplicationPolicyArn", {});
  });

  test("every alarm notifies the ops topic", () => {
    const thresholds = [
      ["Reputation.BounceRate", 0.05, "GreaterThanThreshold"],
      ["Reputation.ComplaintRate", 0.001, "GreaterThanThreshold"],
      ["PutRequests", 1, "LessThanThreshold"],
    ] as const;

    for (const [MetricName, Threshold, ComparisonOperator] of thresholds) {
      template.hasResourceProperties("AWS::CloudWatch::Alarm", {
        MetricName,
        Threshold,
        ComparisonOperator,
        AlarmActions: [Match.anyValue()],
      });
    }

    template.hasResourceProperties("AWS::CloudWatch::Alarm", {
      MetricName: "PutRequests",
      Period: 3600,
      EvaluationPeriods: 6,
      TreatMissingData: "breaching",
    });

    assert.equal(
      Object.keys(template.findResources("AWS::CloudWatch::Alarm", { Properties: { MetricName: "NumberOfNotificationsFailed" } }))
        .length,
      2,
    );
    template.hasResourceProperties("AWS::SNS::Subscription", { Protocol: "email", Endpoint: "ops@example.com" });
  });

  test("a zone id writes every record to route 53 and none to outputs", () => {
    template.resourceCountIs("AWS::Route53::RecordSet", 8);
    assert.equal(dnsOutputs(template), 0);
  });

  test("without a zone id the records become outputs for a human", () => {
    const manual = synth({ hostedZone: undefined });

    manual.resourceCountIs("AWS::Route53::RecordSet", 0);
    assert.equal(dnsOutputs(manual), 8);
  });

  test("a zone that does not hold the inbound domain is refused", () => {
    assert.throws(() => synth({ hostedZone: { id: "ZFIXTUREZONE", name: "other.example.com" } }), /not inside the zone/);
  });

  test("no stage takes another stage's names", () => {
    const production = synth({ stage: "production", hostedZone: undefined });

    production.hasResourceProperties("AWS::SES::ConfigurationSet", { Name: "dimmi-customer-production" });
    assert.doesNotMatch(JSON.stringify(production.toJSON()), /dimmi-[a-z]+-staging/);
  });

  test("the template holds no account id", () => {
    assert.doesNotMatch(JSON.stringify(template.toJSON()), /(?<![0-9])[0-9]{12}(?![0-9])/);
  });
});
