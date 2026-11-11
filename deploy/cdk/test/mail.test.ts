import { Match } from "aws-cdk-lib/assertions";
import { describe, expect, it } from "vitest";
import { only, resources, synthEnvironment } from "./support.ts";

describe("mail stack", () => {
  const template = synthEnvironment("staging").mail;

  it("the inbound rule scans mail and stores it under raw with a notification", () => {
    expect(() =>
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
      }),
    ).not.toThrow();
  });

  it("the system rule stores replies under system without a notification", () => {
    expect(() =>
      template.hasResourceProperties("AWS::SES::ReceiptRule", {
        Rule: Match.objectLike({
          ScanEnabled: true,
          Recipients: ["mail.staging.trydimmi.com"],
          Actions: [{ S3Action: { BucketName: Match.anyValue(), ObjectKeyPrefix: "system/" } }],
        }),
      }),
    ).not.toThrow();
    expect(() => template.resourceCountIs("AWS::SES::ReceiptRule", 2)).not.toThrow();
  });

  it("the rule set is made active by a custom resource and cleared on delete", () => {
    const resource = only(template, "Custom::AWS");

    expect(String(resource.Properties.Create)).toMatch(/setActiveReceiptRuleSet/);
    expect(String(resource.Properties.Create)).toMatch(/dimmi-staging-inbound/);
    expect(String(resource.Properties.Delete)).not.toMatch(/dimmi-staging-inbound/);
  });

  it("raw mail expires after 31 days in a private encrypted bucket", () => {
    expect(() =>
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
      }),
    ).not.toThrow();
  });

  it("only this account's receipt rules may write to the bucket", () => {
    expect(() =>
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
      }),
    ).not.toThrow();
  });

  it("both webhooks retry for about an hour and then dead-letter", () => {
    const subscriptions = resources(template, "AWS::SNS::Subscription");

    expect(subscriptions.map((sub) => sub.Properties.Endpoint).sort()).toEqual([
      "https://staging.trydimmi.com/webhooks/ses/events",
      "https://staging.trydimmi.com/webhooks/ses/inbound",
    ]);

    for (const subscription of subscriptions) {
      expect(subscription.Properties.RedrivePolicy, "dead-letter queue").toBeTruthy();

      const { healthyRetryPolicy: retry } = subscription.Properties.DeliveryPolicy as {
        healthyRetryPolicy: Record<string, number>;
      };
      const backoff = retry.numRetries - retry.numMinDelayRetries - retry.numMaxDelayRetries;
      const fixed = retry.numMinDelayRetries * retry.minDelayTarget + retry.numMaxDelayRetries * retry.maxDelayTarget;

      expect(fixed + backoff * retry.maxDelayTarget, "SNS refuses retry policies over an hour").toBeLessThanOrEqual(
        3600,
      );
      expect(fixed + backoff * retry.minDelayTarget, "retries stop too early").toBeGreaterThanOrEqual(3300);
    }

    expect(() => template.resourceCountIs("AWS::SQS::Queue", 1)).not.toThrow();
  });

  it("the inbound topic takes mail notifications only from this account's rules", () => {
    expect(() =>
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
      }),
    ).not.toThrow();
  });

  it("both identities sign with easy dkim and reject on a missing mail from record", () => {
    for (const domain of ["inbound.staging.trydimmi.com", "mail.staging.trydimmi.com"]) {
      expect(() =>
        template.hasResourceProperties("AWS::SES::EmailIdentity", {
          EmailIdentity: domain,
          DkimAttributes: { SigningEnabled: true },
          DkimSigningAttributes: Match.absent(),
          MailFromAttributes: { MailFromDomain: `bounce.${domain}`, BehaviorOnMxFailure: "REJECT_MESSAGE" },
        }),
      ).not.toThrow();
    }
  });

  it("each configuration set carries the environment and sends events to the topic", () => {
    expect(() =>
      template.hasResourceProperties("AWS::SES::ConfigurationSet", { Name: "dimmi-system-staging" }),
    ).not.toThrow();
    expect(() =>
      template.hasResourceProperties("AWS::SES::ConfigurationSet", { Name: "dimmi-customer-staging" }),
    ).not.toThrow();
    expect(() => template.resourceCountIs("AWS::SES::ConfigurationSetEventDestination", 2)).not.toThrow();
    expect(() =>
      template.allResourcesProperties("AWS::SES::ConfigurationSetEventDestination", {
        EventDestination: {
          Enabled: true,
          MatchingEventTypes: Match.arrayWith(["reject", "bounce", "complaint", "delivery"]),
          SnsDestination: { TopicARN: Match.anyValue() },
        },
      }),
    ).not.toThrow();
  });

  it("the configuration sets never ask for opens or clicks", () => {
    const destinations = resources(template, "AWS::SES::ConfigurationSetEventDestination");

    expect(JSON.stringify(destinations)).not.toMatch(/"open"|"click"/);
  });

  it("the application policy sends, manages identities and tenants, and reads raw mail", () => {
    const policy = only(template, "AWS::IAM::ManagedPolicy");
    const statements = (policy.Properties.PolicyDocument as { Statement: { Action: string | string[] }[] }).Statement;
    const actions = statements.flatMap((statement) => statement.Action);

    expect(actions).toEqual(
      expect.arrayContaining([
        "ses:SendEmail",
        "ses:SendRawEmail",
        "ses:CreateEmailIdentity",
        "ses:CreateTenant",
        "s3:GetObject",
        "s3:DeleteObject",
      ]),
    );
    expect(actions, "only SES writes raw mail").not.toContain("s3:PutObject");
    expect(policy.Properties.ManagedPolicyName).toBe("dimmi-staging-mail-application");
  });

  it("one application user carries the policy and no access key is ever made here", () => {
    const user = only(template, "AWS::IAM::User");

    expect(user.Properties.UserName).toBe("dimmi-staging-application");
    expect((user.Properties.ManagedPolicyArns as unknown[]).length).toBe(1);
    expect(() => template.resourceCountIs("AWS::IAM::AccessKey", 0)).not.toThrow();
  });

  it("every alarm notifies the ops topic", () => {
    const thresholds = [
      ["Reputation.BounceRate", 0.05, "GreaterThanThreshold"],
      ["Reputation.ComplaintRate", 0.001, "GreaterThanThreshold"],
      ["PutRequests", 1, "LessThanThreshold"],
    ] as const;

    for (const [MetricName, Threshold, ComparisonOperator] of thresholds) {
      expect(() =>
        template.hasResourceProperties("AWS::CloudWatch::Alarm", {
          MetricName,
          Threshold,
          ComparisonOperator,
          AlarmActions: [Match.anyValue()],
        }),
      ).not.toThrow();
    }

    expect(() =>
      template.hasResourceProperties("AWS::CloudWatch::Alarm", {
        MetricName: "PutRequests",
        Period: 3600,
        EvaluationPeriods: 6,
        TreatMissingData: "breaching",
      }),
    ).not.toThrow();

    const failures = template.findResources("AWS::CloudWatch::Alarm", {
      Properties: { MetricName: "NumberOfNotificationsFailed" },
    });
    expect(Object.keys(failures)).toHaveLength(2);
  });

  it("every dns record is one output for the dns script, and route 53 is never used", () => {
    expect(() => template.resourceCountIs("AWS::Route53::RecordSet", 0)).not.toThrow();
    expect(() => template.resourceCountIs("AWS::Route53::HostedZone", 0)).not.toThrow();

    const output = JSON.stringify(template.findOutputs("DnsRecords"));
    for (const name of [
      "inbound.staging.trydimmi.com",
      "_dmarc.mail.staging.trydimmi.com",
      "bounce.mail.staging.trydimmi.com",
    ]) {
      expect(output).toContain(name);
    }
    expect(output).toContain("inbound-smtp.us-east-2.amazonaws.com");
    expect(output).toContain("DkimDNSTokenName3");
  });

  it("production takes none of staging's names", () => {
    const production = synthEnvironment("production").mail;

    expect(() =>
      production.hasResourceProperties("AWS::SES::ConfigurationSet", { Name: "dimmi-customer-production" }),
    ).not.toThrow();
    expect(JSON.stringify(production.toJSON())).not.toMatch(/staging/);
  });
});
