import type { StackProps } from "aws-cdk-lib";
import { CfnOutput, Duration, Stack } from "aws-cdk-lib";
import type { IRepository } from "aws-cdk-lib/aws-ecr";
import type { FargateService } from "aws-cdk-lib/aws-ecs";
import { Effect, FederatedPrincipal, PolicyStatement, Role } from "aws-cdk-lib/aws-iam";
import type { Construct } from "constructs";
import type { EnvironmentConfig } from "../config/environment.ts";
import { environments } from "../config/environments.ts";
import { GITHUB_ISSUER } from "./account-stack.ts";
import type { AppStack } from "./app-stack.ts";
import type { DataStack } from "./data-stack.ts";

export type CiStackProps = StackProps & {
  config: EnvironmentConfig;
  repository: IRepository;
  app: AppStack;
  data: DataStack;
};

/**
 * The role a GitHub Actions run assumes to release into this environment.
 *
 * Only runs in the repository's matching GitHub environment may assume it,
 * which is where approvals for production are enforced. It can push images,
 * run the one-off tasks, roll the services and read what a release needs to
 * report. It cannot change infrastructure: stacks are deployed by a person.
 * Every grant names this environment's resources, so the staging role
 * cannot reach production.
 */
export class CiStack extends Stack {
  readonly role: Role;

  constructor(scope: Construct, id: string, props: CiStackProps) {
    super(scope, id, props);

    const { config } = props;

    this.role = new Role(this, "DeployRole", {
      roleName: `dimmi-${config.name}-deploy`,
      description: `GitHub Actions releases to ${config.name}`,
      maxSessionDuration: Duration.hours(1),
      assumedBy: new FederatedPrincipal(
        this.formatArn({ service: "iam", region: "", resource: "oidc-provider", resourceName: GITHUB_ISSUER }),
        {
          StringEquals: {
            [`${GITHUB_ISSUER}:aud`]: "sts.amazonaws.com",
            [`${GITHUB_ISSUER}:sub`]: `repo:${config.github.repository}:environment:${config.github.environment}`,
          },
        },
        "sts:AssumeRoleWithWebIdentity",
      ),
    });

    for (const statement of this.statements(props)) {
      this.role.addToPolicy(statement);
    }

    new CfnOutput(this, "DeployRoleArn", { value: this.role.roleArn });
  }

  private statements(props: CiStackProps): PolicyStatement[] {
    const { config, app, data } = props;
    const cluster = app.cluster.clusterArn;
    const family = (name: string) =>
      this.formatArn({ service: "ecs", resource: "task-definition", resourceName: `dimmi-${config.name}-${name}:*` });
    const stacks = this.formatArn({
      service: "cloudformation",
      resource: "stack",
      resourceName: `dimmi-${config.name}-*/*`,
    });

    return [
      new PolicyStatement({ sid: "Login", actions: ["ecr:GetAuthorizationToken"], resources: ["*"] }),
      new PolicyStatement({
        sid: "PushImages",
        actions: [
          "ecr:BatchCheckLayerAvailability",
          "ecr:BatchGetImage",
          "ecr:CompleteLayerUpload",
          "ecr:DescribeImages",
          "ecr:GetDownloadUrlForLayer",
          "ecr:InitiateLayerUpload",
          "ecr:PutImage",
          "ecr:UploadLayerPart",
        ],
        resources: [props.repository.repositoryArn],
      }),
      ...this.promotion(config),
      new PolicyStatement({
        sid: "RunOneOffTasks",
        actions: ["ecs:RunTask"],
        resources: [family("migrate"), family("provision")],
        conditions: { ArnEquals: { "ecs:cluster": cluster } },
      }),
      new PolicyStatement({
        sid: "RollServices",
        actions: ["ecs:UpdateService", "ecs:DescribeServices"],
        resources: app.services.map((service: FargateService) => service.serviceArn),
      }),
      new PolicyStatement({
        sid: "WatchTasks",
        actions: ["ecs:DescribeTasks", "ecs:ListTasks"],
        resources: ["*"],
        conditions: { ArnEquals: { "ecs:cluster": cluster } },
      }),
      new PolicyStatement({ sid: "ReadTaskDefinitions", actions: ["ecs:DescribeTaskDefinition"], resources: ["*"] }),
      new PolicyStatement({
        sid: "PassTaskRoles",
        actions: ["iam:PassRole"],
        resources: [
          this.formatArn({ service: "iam", region: "", resource: "role", resourceName: `dimmi-${config.name}-app-*` }),
        ],
        conditions: { StringEquals: { "iam:PassedToService": "ecs-tasks.amazonaws.com" } },
      }),
      new PolicyStatement({ sid: "ReadOutputs", actions: ["cloudformation:DescribeStacks"], resources: [stacks] }),
      new PolicyStatement({
        sid: "ReadTaskLogs",
        actions: ["logs:GetLogEvents", "logs:FilterLogEvents"],
        resources: [
          `${this.formatArn({ service: "logs", resource: "log-group", resourceName: `/dimmi/${config.name}` })}:*`,
        ],
      }),
      new PolicyStatement({
        sid: "PassTheGate",
        effect: Effect.ALLOW,
        actions: ["secretsmanager:GetSecretValue"],
        resources: [data.gateCiKey.secretArn],
      }),
    ];
  }

  /** Read access to the images of the environment this one promotes from. */
  private promotion(config: EnvironmentConfig): PolicyStatement[] {
    const source = config.promotesFrom ? environments[config.promotesFrom] : undefined;
    if (!source) {
      return [];
    }

    return [
      new PolicyStatement({
        sid: "PullPromotedImages",
        actions: [
          "ecr:BatchCheckLayerAvailability",
          "ecr:BatchGetImage",
          "ecr:DescribeImages",
          "ecr:GetDownloadUrlForLayer",
        ],
        resources: [
          this.formatArn({
            service: "ecr",
            region: source.region,
            resource: "repository",
            resourceName: `dimmi-${source.name}`,
          }),
        ],
      }),
    ];
  }
}
