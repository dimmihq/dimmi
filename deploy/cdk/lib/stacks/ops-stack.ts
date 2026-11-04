import type { StackProps } from "aws-cdk-lib";
import { CfnOutput, Stack } from "aws-cdk-lib";
import { CfnBudget } from "aws-cdk-lib/aws-budgets";
import { PolicyStatement, ServicePrincipal } from "aws-cdk-lib/aws-iam";
import { Topic } from "aws-cdk-lib/aws-sns";
import { EmailSubscription } from "aws-cdk-lib/aws-sns-subscriptions";
import type { Construct } from "constructs";
import type { EnvironmentConfig } from "../config/environment.ts";

export type OpsStackProps = StackProps & { config: EnvironmentConfig };

/**
 * Where an environment calls for help: one topic every alarm publishes to,
 * mailed to the alarm address, and a monthly budget on the environment's
 * tag. The address must confirm the subscription mail before anything
 * reaches it.
 */
export class OpsStack extends Stack {
  readonly topic: Topic;

  constructor(scope: Construct, id: string, props: OpsStackProps) {
    super(scope, id, props);

    const { config } = props;

    this.topic = new Topic(this, "OpsTopic", { topicName: `dimmi-${config.name}-ops` });
    this.topic.addSubscription(new EmailSubscription(config.alarmEmail));
    this.topic.addToResourcePolicy(
      new PolicyStatement({
        actions: ["sns:Publish"],
        principals: [new ServicePrincipal("cloudwatch.amazonaws.com")],
        resources: [this.topic.topicArn],
        conditions: { StringEquals: { "aws:SourceAccount": this.account } },
      }),
    );

    this.budget(config);

    new CfnOutput(this, "OpsTopicArn", { value: this.topic.topicArn });
  }

  /**
   * Filters on the `Environment` tag, which only counts once a person has
   * activated it as a cost allocation tag in the billing console.
   */
  private budget(config: EnvironmentConfig): void {
    new CfnBudget(this, "Budget", {
      budget: {
        budgetName: `dimmi-${config.name}`,
        budgetType: "COST",
        timeUnit: "MONTHLY",
        budgetLimit: { amount: config.budgetUsd, unit: "USD" },
        costFilters: { TagKeyValue: [`user:Environment$${config.name}`] },
      },
      notificationsWithSubscribers: [80, 100].map((threshold) => ({
        notification: {
          notificationType: threshold === 100 ? "FORECASTED" : "ACTUAL",
          comparisonOperator: "GREATER_THAN",
          threshold,
          thresholdType: "PERCENTAGE",
        },
        subscribers: [{ subscriptionType: "EMAIL", address: config.alarmEmail }],
      })),
    });
  }
}
