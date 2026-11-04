import { Stack } from "aws-cdk-lib";
import { AwsCustomResource, AwsCustomResourcePolicy, PhysicalResourceId } from "aws-cdk-lib/custom-resources";
import { Effect, PolicyStatement } from "aws-cdk-lib/aws-iam";
import { Construct } from "constructs";

/**
 * SES keeps one active receipt rule set per account and Region, and
 * CloudFormation has no resource for choosing it. Without this step a deployed
 * rule set receives nothing. Deleting the stack clears the active set, so two
 * stages must never share a Region: the second would silently take the first
 * one's mail.
 */
export class ActivateRuleSet extends Construct {
  constructor(scope: Construct, id: string, ruleSetName: string) {
    super(scope, id);

    const activate = {
      service: "SES",
      action: "setActiveReceiptRuleSet",
      parameters: { RuleSetName: ruleSetName },
      physicalResourceId: PhysicalResourceId.of(`active-receipt-rule-set-${ruleSetName}`),
    };

    new AwsCustomResource(this, "Resource", {
      onCreate: activate,
      onUpdate: activate,
      onDelete: { service: "SES", action: "setActiveReceiptRuleSet", parameters: {} },
      installLatestAwsSdk: false,
      policy: AwsCustomResourcePolicy.fromStatements([
        new PolicyStatement({
          effect: Effect.ALLOW,
          actions: ["ses:SetActiveReceiptRuleSet"],
          resources: ["*"],
          conditions: { StringEquals: { "aws:RequestedRegion": Stack.of(this).region } },
        }),
      ]),
    });
  }
}
