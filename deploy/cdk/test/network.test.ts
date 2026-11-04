import { Match } from "aws-cdk-lib/assertions";
import { describe, expect, it } from "vitest";
import { synthEnvironment } from "./support.ts";

describe("network stack", () => {
  const staging = synthEnvironment("staging").network;

  it("there is no nat gateway and data subnets have no route out", () => {
    expect(() => staging.resourceCountIs("AWS::EC2::NatGateway", 0)).not.toThrow();
    expect(() => staging.resourceCountIs("AWS::EC2::VPCEndpoint", 1)).not.toThrow();
  });

  it("the database and cache admit only application tasks", () => {
    for (const port of [5432, 6379]) {
      expect(() =>
        staging.hasResourceProperties("AWS::EC2::SecurityGroupIngress", {
          FromPort: port,
          ToPort: port,
          SourceSecurityGroupId: Match.anyValue(),
        }),
      ).not.toThrow();
    }
  });

  it("application tasks take http and https from anywhere, as the load balancer does", () => {
    expect(() =>
      staging.hasResourceProperties("AWS::EC2::SecurityGroup", {
        SecurityGroupIngress: Match.arrayWith([
          Match.objectLike({ CidrIp: "0.0.0.0/0", FromPort: 8443 }),
          Match.objectLike({ CidrIp: "0.0.0.0/0", FromPort: 8080 }),
        ]),
      }),
    ).not.toThrow();
  });
});
