import { SecretValue } from "aws-cdk-lib";
import type { ISecurityGroup, IVpc } from "aws-cdk-lib/aws-ec2";
import { SubnetType } from "aws-cdk-lib/aws-ec2";
import { CfnReplicationGroup, CfnSubnetGroup } from "aws-cdk-lib/aws-elasticache";
import { Secret } from "aws-cdk-lib/aws-secretsmanager";
import { Construct } from "constructs";
import type { EnvironmentConfig } from "../config/environment.ts";

export type ValkeyProps = {
  config: EnvironmentConfig;
  vpc: IVpc;
  securityGroup: ISecurityGroup;
};

/** Matches the major version the application is developed and tested on. */
const ENGINE_VERSION = "8.1";

/**
 * Node-based ElastiCache for Valkey with TLS and an AUTH token. Serverless is
 * not an option: it lacks `CLIENT TRACKING`, which the cache's client-side
 * caching relies on. CDK has no L2 for replication groups, so this wraps the
 * CloudFormation resource.
 */
export class Valkey extends Construct {
  readonly replicationGroup: CfnReplicationGroup;
  readonly authSecret: Secret;

  constructor(scope: Construct, id: string, props: ValkeyProps) {
    super(scope, id);

    const { config } = props;
    const replicated = config.cache.replicas > 0;

    this.authSecret = new Secret(this, "AuthToken", {
      secretName: `dimmi-${config.name}/cache/auth`,
      description: "Valkey AUTH token",
      generateSecretString: { excludePunctuation: true, passwordLength: 64 },
    });

    const subnets = new CfnSubnetGroup(this, "Subnets", {
      cacheSubnetGroupName: `dimmi-${config.name}`,
      description: `dimmi ${config.name} cache`,
      subnetIds: props.vpc.selectSubnets({ subnetType: SubnetType.PRIVATE_ISOLATED }).subnetIds,
    });

    this.replicationGroup = new CfnReplicationGroup(this, "ReplicationGroup", {
      replicationGroupId: `dimmi-${config.name}`,
      replicationGroupDescription: `dimmi ${config.name}`,
      engine: "valkey",
      engineVersion: ENGINE_VERSION,
      cacheNodeType: config.cache.nodeType,
      numCacheClusters: 1 + config.cache.replicas,
      automaticFailoverEnabled: replicated,
      multiAzEnabled: replicated,
      cacheSubnetGroupName: subnets.ref,
      securityGroupIds: [props.securityGroup.securityGroupId],
      transitEncryptionEnabled: true,
      atRestEncryptionEnabled: true,
      authToken: this.authSecret.secretValue.unsafeUnwrap(),
      autoMinorVersionUpgrade: true,
      snapshotRetentionLimit: config.protectData ? 7 : 0,
    });
  }

  /** A `rediss://` URL with the token, resolved by CloudFormation like the database URLs. */
  url(): SecretValue {
    const token = this.authSecret.secretValue.unsafeUnwrap();
    const host = this.replicationGroup.attrPrimaryEndPointAddress;
    const port = this.replicationGroup.attrPrimaryEndPointPort;

    return SecretValue.unsafePlainText(`rediss://:${token}@${host}:${port}`);
  }
}
