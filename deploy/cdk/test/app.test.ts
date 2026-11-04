import { Match } from "aws-cdk-lib/assertions";
import { describe, expect, it } from "vitest";
import { resources, synthEnvironment } from "./support.ts";

type Container = {
  Name: string;
  Command?: string[];
  Environment?: { Name: string; Value: unknown }[];
  Secrets?: { Name: string; ValueFrom: unknown }[];
  Image: unknown;
};

function containers(family: string, template: ReturnType<typeof synthEnvironment>["app"]): Container[] {
  const task = resources(template, "AWS::ECS::TaskDefinition").find((found) => found.Properties.Family === family);
  if (!task) {
    throw new Error(`no task definition ${family}`);
  }

  return task.Properties.ContainerDefinitions as Container[];
}

function variable(container: Container, name: string): unknown {
  return container.Environment?.find((entry) => entry.Name === name)?.Value;
}

describe("app stack", () => {
  const staging = synthEnvironment("staging").app;
  const production = synthEnvironment("production").app;

  it("the load balancer passes tcp through on 443 and 80 from fixed addresses", () => {
    expect(() =>
      staging.hasResourceProperties("AWS::ElasticLoadBalancingV2::LoadBalancer", {
        Type: "network",
        Scheme: "internet-facing",
        SubnetMappings: Match.arrayWith([Match.objectLike({ AllocationId: Match.anyValue() })]),
      }),
    ).not.toThrow();
    expect(() => staging.resourceCountIs("AWS::EC2::EIP", 2)).not.toThrow();
    for (const port of [443, 80]) {
      expect(() =>
        staging.hasResourceProperties("AWS::ElasticLoadBalancingV2::Listener", { Port: port, Protocol: "TCP" }),
      ).not.toThrow();
    }
    expect(() => staging.resourceCountIs("AWS::CertificateManager::Certificate", 0)).not.toThrow();
  });

  it("targets keep the client's address and are checked on ready over plain http", () => {
    for (const port of [8443, 8080]) {
      expect(() =>
        staging.hasResourceProperties("AWS::ElasticLoadBalancingV2::TargetGroup", {
          Port: port,
          Protocol: "TCP",
          TargetType: "ip",
          HealthCheckProtocol: "HTTP",
          HealthCheckPort: "8080",
          HealthCheckPath: "/ready",
          TargetGroupAttributes: Match.arrayWith([{ Key: "preserve_client_ip.enabled", Value: "true" }]),
        }),
      ).not.toThrow();
      expect(() =>
        staging.hasResourceProperties("AWS::ElasticLoadBalancingV2::TargetGroup", {
          Port: port,
          TargetGroupAttributes: Match.arrayWith([{ Key: "deregistration_delay.timeout_seconds", Value: "120" }]),
        }),
      ).not.toThrow();
    }
  });

  it("services roll without dropping capacity and roll back on failure", () => {
    expect(() =>
      staging.hasResourceProperties("AWS::ECS::Service", {
        LaunchType: "FARGATE",
        DesiredCount: 1,
        EnableExecuteCommand: true,
        DeploymentConfiguration: {
          MinimumHealthyPercent: 100,
          MaximumPercent: 200,
          DeploymentCircuitBreaker: { Enable: true, Rollback: true },
        },
      }),
    ).not.toThrow();
    expect(() => production.resourceCountIs("AWS::ECS::Service", 2)).not.toThrow();
    expect(() => staging.resourceCountIs("AWS::ECS::Service", 1)).not.toThrow();
  });

  it("every task runs on arm and the app containers run the release tag", () => {
    expect(() =>
      staging.allResourcesProperties("AWS::ECS::TaskDefinition", {
        RuntimePlatform: { CpuArchitecture: "ARM64", OperatingSystemFamily: "LINUX" },
        RequiresCompatibilities: ["FARGATE"],
      }),
    ).not.toThrow();
    const web = containers("dimmi-staging-web", staging)[0];

    expect(JSON.stringify(web.Image)).toMatch(/:release"/);
    expect(web.Command).toEqual(["serve"]);
    expect(variable(web, "DIMMI_APP__ROLES")).toBe("web,worker,scheduler");
    expect(variable(web, "DIMMI_APP__URL")).toBe("https://staging.trydimmi.com");
    expect(variable(web, "DIMMI_MIGRATIONS__AUTO")).toBe("false");
    expect(variable(web, "DIMMI_GATE__ENABLED")).toBe("true");
  });

  it("production runs web and worker roles in separate services with the gate off", () => {
    const web = containers("dimmi-production-web", production)[0];
    const worker = containers("dimmi-production-worker", production)[0];

    expect(variable(web, "DIMMI_APP__ROLES")).toBe("web");
    expect(variable(worker, "DIMMI_APP__ROLES")).toBe("worker,scheduler");
    expect(variable(web, "DIMMI_GATE__ENABLED")).toBe("false");
    expect(!web.Secrets?.some((secret) => secret.Name.startsWith("DIMMI_GATE__"))).toBeTruthy();
    expect(String(variable(web, "DIMMI_CUSTOM_DOMAINS__TLS__CERTMAGIC__CA"))).toMatch(/acme-v02/);
  });

  it("secrets reach containers from secrets manager, never as plain values", () => {
    const web = containers("dimmi-staging-web", staging)[0];
    const secretNames = web.Secrets?.map((secret) => secret.Name) ?? [];

    for (const name of ["DIMMI_DATABASE__URL", "DIMMI_REDIS__URL", "DIMMI_APP__KEY", "DIMMI_GATE__CI_KEY"]) {
      expect(secretNames.includes(name), `missing secret ${name}`).toBeTruthy();
      expect(variable(web, name), `${name} must not be plain`).toBe(undefined);
    }
  });

  it("migrate runs as the schema owner and provision as the master user", () => {
    const migrate = containers("dimmi-staging-migrate", staging)[0];
    const provision = containers("dimmi-staging-provision", staging)[0];
    const url = migrate.Secrets?.find((secret) => secret.Name === "DIMMI_DATABASE__URL");

    expect(migrate.Command).toEqual(["migrate", "--force"]);
    expect(JSON.stringify(url?.ValueFrom)).toMatch(/owner_database_url/);
    expect(String(variable(provision, "PROVISION_SQL"))).toMatch(/NOBYPASSRLS/);
    expect(provision.Secrets?.some((secret) => secret.Name === "PGPASSWORD")).toBeTruthy();
  });

  it("task roles are named for the environment so release can pass only them", () => {
    const names = resources(staging, "AWS::IAM::Role").map((role) => String(role.Properties.RoleName ?? ""));

    expect(names.includes("dimmi-staging-app-web-task")).toBeTruthy();
    expect(names.includes("dimmi-staging-app-migrate-execution")).toBeTruthy();
  });

  it("the release script finds everything it needs in the outputs", () => {
    for (const output of [
      "LoadBalancerAddresses",
      "ClusterName",
      "ServiceNames",
      "TaskSubnetIds",
      "TaskSecurityGroupId",
    ]) {
      expect(() => staging.hasOutput(output, {})).not.toThrow();
    }
  });
});
