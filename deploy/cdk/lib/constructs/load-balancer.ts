import { Duration } from "aws-cdk-lib";
import type { IVpc } from "aws-cdk-lib/aws-ec2";
import { CfnEIP, SubnetType } from "aws-cdk-lib/aws-ec2";
import type { FargateService } from "aws-cdk-lib/aws-ecs";
import { NetworkLoadBalancer, NetworkTargetGroup, Protocol, TargetType } from "aws-cdk-lib/aws-elasticloadbalancingv2";
import { Construct } from "constructs";
import { APP_PORTS } from "../stacks/network-stack.ts";

export type LoadBalancerProps = {
  environmentName: string;
  vpc: IVpc;
};

/**
 * Seconds a leaving task keeps its open connections. Realtime clients are
 * told to reconnect on shutdown, so this only needs to outlast requests.
 */
const DEREGISTRATION_SECONDS = 120;

/**
 * A Network Load Balancer passing TCP through on 443 and 80, so the binary
 * terminates TLS itself with certificates it keeps in PostgreSQL.
 *
 * Each zone gets an Elastic IP. DNS then holds plain A records, which work at
 * a zone apex, where a CNAME is not allowed, and never change while the load
 * balancer lives.
 *
 * Fargate tasks register by IP, where TCP target groups hide the client's
 * address unless told otherwise. Client IP preservation is turned on, so the
 * binary reads the real address from the connection: no proxy protocol and
 * no trusted proxies. The cost is that a task cannot reach itself through
 * the load balancer.
 */
export class LoadBalancer extends Construct {
  readonly nlb: NetworkLoadBalancer;
  readonly addresses: CfnEIP[];
  private readonly targetGroups: NetworkTargetGroup[];

  constructor(scope: Construct, id: string, props: LoadBalancerProps) {
    super(scope, id);

    const subnets = props.vpc.selectSubnets({ subnetType: SubnetType.PUBLIC }).subnets;
    this.addresses = subnets.map(
      (_, index) =>
        new CfnEIP(this, `Address${index}`, {
          domain: "vpc",
          tags: [{ key: "Name", value: `dimmi-${props.environmentName}-${index}` }],
        }),
    );

    this.nlb = new NetworkLoadBalancer(this, "Nlb", {
      loadBalancerName: `dimmi-${props.environmentName}`,
      vpc: props.vpc,
      internetFacing: true,
      crossZoneEnabled: true,
      subnetMappings: subnets.map((subnet, index) => ({
        subnet,
        allocationId: this.addresses[index].attrAllocationId,
      })),
    });

    this.targetGroups = [
      this.forward("Https", 443, APP_PORTS.https, props),
      this.forward("Http", 80, APP_PORTS.http, props),
    ];
  }

  /** Registers the web service's tasks on both ports. */
  attach(service: FargateService): void {
    for (const [index, group] of this.targetGroups.entries()) {
      const containerPort = index === 0 ? APP_PORTS.https : APP_PORTS.http;
      group.addTarget(service.loadBalancerTarget({ containerName: "app", containerPort }));
    }
  }

  /**
   * Both ports share one health check: `/ready` over plain HTTP, which the
   * binary answers without redirecting to HTTPS.
   */
  private forward(id: string, listenerPort: number, targetPort: number, props: LoadBalancerProps) {
    const group = new NetworkTargetGroup(this, `${id}Targets`, {
      targetGroupName: `dimmi-${props.environmentName}-${listenerPort}`,
      vpc: props.vpc,
      port: targetPort,
      protocol: Protocol.TCP,
      targetType: TargetType.IP,
      preserveClientIp: true,
      deregistrationDelay: Duration.seconds(DEREGISTRATION_SECONDS),
      healthCheck: {
        protocol: Protocol.HTTP,
        port: String(APP_PORTS.http),
        path: "/ready",
        interval: Duration.seconds(10),
        healthyThresholdCount: 2,
        unhealthyThresholdCount: 3,
      },
    });

    this.nlb.addListener(`${id}Listener`, {
      port: listenerPort,
      protocol: Protocol.TCP,
      defaultTargetGroups: [group],
    });

    return group;
  }
}
