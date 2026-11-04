import type { StackProps } from "aws-cdk-lib";
import { Stack } from "aws-cdk-lib";
import {
  GatewayVpcEndpointAwsService,
  IpAddresses,
  Peer,
  Port,
  SecurityGroup,
  SubnetType,
  Vpc,
} from "aws-cdk-lib/aws-ec2";
import type { Construct } from "constructs";
import type { EnvironmentConfig } from "../config/environment.ts";

export type NetworkStackProps = StackProps & { config: EnvironmentConfig };

/** The two ports the binary listens on inside its container. */
export const APP_PORTS = { http: 8080, https: 8443 } as const;

const POSTGRES_PORT = 5432;
const VALKEY_PORT = 6379;

/**
 * The VPC and the security groups between its parts. Groups live here, not
 * beside what they guard, so data and compute stacks can both refer to them
 * without depending on each other.
 *
 * Databases sit in isolated subnets with no route out. Tasks sit in public
 * subnets with public addresses unless the environment pays for NAT: they
 * must reach SES, SNS, OAuth providers and Let's Encrypt.
 *
 * The load balancer keeps each client's own address, so the application
 * group admits the whole internet on its two ports. That is no wider than
 * the load balancer itself, which passes TCP through untouched.
 */
export class NetworkStack extends Stack {
  readonly vpc: Vpc;
  readonly appSecurityGroup: SecurityGroup;
  readonly databaseSecurityGroup: SecurityGroup;
  readonly cacheSecurityGroup: SecurityGroup;
  /** True when tasks sit in public subnets and need a public address. */
  readonly tasksArePublic: boolean;

  constructor(scope: Construct, id: string, props: NetworkStackProps) {
    super(scope, id, props);

    const { network } = props.config;
    this.tasksArePublic = network.natGateways === 0;

    this.vpc = new Vpc(this, "Vpc", {
      vpcName: `dimmi-${props.config.name}`,
      ipAddresses: IpAddresses.cidr("10.40.0.0/16"),
      maxAzs: network.maxAzs,
      natGateways: network.natGateways,
      subnetConfiguration: [
        { name: "public", subnetType: SubnetType.PUBLIC, cidrMask: 22 },
        ...(this.tasksArePublic ? [] : [{ name: "app", subnetType: SubnetType.PRIVATE_WITH_EGRESS, cidrMask: 22 }]),
        { name: "data", subnetType: SubnetType.PRIVATE_ISOLATED, cidrMask: 24 },
      ],
      gatewayEndpoints: { S3: { service: GatewayVpcEndpointAwsService.S3 } },
    });

    this.appSecurityGroup = this.group("App", "Application tasks");
    this.databaseSecurityGroup = this.group("Database", "PostgreSQL, reachable from application tasks only");
    this.cacheSecurityGroup = this.group("Cache", "Valkey, reachable from application tasks only");

    this.appSecurityGroup.addIngressRule(Peer.anyIpv4(), Port.tcp(APP_PORTS.https), "HTTPS, passed through by the NLB");
    this.appSecurityGroup.addIngressRule(Peer.anyIpv4(), Port.tcp(APP_PORTS.http), "HTTP, ACME and health checks");
    this.databaseSecurityGroup.addIngressRule(this.appSecurityGroup, Port.tcp(POSTGRES_PORT), "Application tasks");
    this.cacheSecurityGroup.addIngressRule(this.appSecurityGroup, Port.tcp(VALKEY_PORT), "Application tasks");
  }

  /**
   * Only the application group may open connections. Database and cache
   * groups answer and never call out.
   */
  private group(id: string, description: string): SecurityGroup {
    return new SecurityGroup(this, `${id}SecurityGroup`, {
      vpc: this.vpc,
      description,
      allowAllOutbound: id === "App",
    });
  }
}
