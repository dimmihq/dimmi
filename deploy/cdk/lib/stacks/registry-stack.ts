import type { StackProps } from "aws-cdk-lib";
import { CfnOutput, RemovalPolicy, Stack } from "aws-cdk-lib";
import { Repository, TagMutability, TagStatus } from "aws-cdk-lib/aws-ecr";
import type { Construct } from "constructs";
import type { EnvironmentConfig } from "../config/environment.ts";

export type RegistryStackProps = StackProps & { config: EnvironmentConfig };

/**
 * The tag every task definition runs. A release points it at a new image and
 * forces a new deployment; ECS pins the digest per deployment, so a rollback
 * returns to exactly the image that ran before.
 */
export const RELEASE_TAG = "release";

/** How many images stay for rolling back to. */
const KEPT_IMAGES = 30;

/**
 * The image repository, alone in its stack because it must hold an image
 * before the application stack can start a single task.
 */
export class RegistryStack extends Stack {
  readonly repository: Repository;

  constructor(scope: Construct, id: string, props: RegistryStackProps) {
    super(scope, id, props);

    this.repository = new Repository(this, "Repository", {
      repositoryName: `dimmi-${props.config.name}`,
      imageTagMutability: TagMutability.MUTABLE,
      imageScanOnPush: true,
      removalPolicy: props.config.protectData ? RemovalPolicy.RETAIN : RemovalPolicy.DESTROY,
      emptyOnDelete: !props.config.protectData,
      lifecycleRules: [{ tagStatus: TagStatus.ANY, maxImageCount: KEPT_IMAGES }],
    });

    new CfnOutput(this, "RepositoryUri", { value: this.repository.repositoryUri });
  }
}
