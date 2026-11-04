import { readFileSync } from "node:fs";
import type { StackProps } from "aws-cdk-lib";
import { CfnOutput, Duration, Fn, RemovalPolicy, Stack } from "aws-cdk-lib";
import { Alarm, ComparisonOperator, TreatMissingData } from "aws-cdk-lib/aws-cloudwatch";
import { SnsAction } from "aws-cdk-lib/aws-cloudwatch-actions";
import { SubnetType } from "aws-cdk-lib/aws-ec2";
import type { IRepository } from "aws-cdk-lib/aws-ecr";
import {
  Cluster,
  ContainerImage,
  ContainerInsights,
  CpuArchitecture,
  Secret as EcsSecret,
  FargateService,
  FargateTaskDefinition,
  LogDrivers,
  OperatingSystemFamily,
} from "aws-cdk-lib/aws-ecs";
import { Role, ServicePrincipal } from "aws-cdk-lib/aws-iam";
import { LogGroup, RetentionDays } from "aws-cdk-lib/aws-logs";
import type { ITopic } from "aws-cdk-lib/aws-sns";
import type { Construct } from "constructs";
import type { EnvironmentConfig, ServiceConfig } from "../config/environment.ts";
import type { AppSettings } from "../constructs/app-settings.ts";
import { appSettings, SHUTDOWN_SECONDS } from "../constructs/app-settings.ts";
import { DATABASE_NAME, ROLES } from "../constructs/database.ts";
import { LoadBalancer } from "../constructs/load-balancer.ts";
import type { DataStack } from "./data-stack.ts";
import type { MailStack } from "./mail-stack.ts";
import type { NetworkStack } from "./network-stack.ts";
import { APP_PORTS } from "./network-stack.ts";
import { RELEASE_TAG } from "./registry-stack.ts";

export type AppStackProps = StackProps & {
  config: EnvironmentConfig;
  /**
   * Runs every service at zero tasks while everything else stays. Used for
   * the first deploy, before the database has roles and a schema, and to stop
   * an environment's compute without losing anything.
   */
  paused?: boolean;
  network: NetworkStack;
  data: DataStack;
  mail: MailStack;
  repository: IRepository;
  alarms: ITopic;
};

/** The PostgreSQL client image the provisioning task runs. */
const POSTGRES_IMAGE = "public.ecr.aws/docker/library/postgres:18-alpine";

/**
 * The running application: an ECS cluster on ARM Fargate, the web service
 * behind the load balancer, an optional worker service, and two one-off
 * task definitions the release script runs: `provision` sets up database
 * roles and `migrate` changes the schema as its owner.
 *
 * Tasks are disposable. Every deploy starts fresh containers, nothing is kept
 * on local disk, and a deploy that fails its health checks rolls back on its
 * own through the deployment circuit breaker.
 */
export class AppStack extends Stack {
  readonly cluster: Cluster;
  readonly services: FargateService[] = [];
  readonly loadBalancer: LoadBalancer;
  private readonly props: AppStackProps;
  private readonly settings: AppSettings;
  private readonly logs: LogGroup;

  constructor(scope: Construct, id: string, props: AppStackProps) {
    super(scope, id, props);

    this.props = props;
    const { config } = props;
    this.settings = appSettings(config, props.data, props.mail);

    this.cluster = new Cluster(this, "Cluster", {
      clusterName: `dimmi-${config.name}`,
      vpc: props.network.vpc,
      containerInsightsV2: config.protectData ? ContainerInsights.ENHANCED : ContainerInsights.DISABLED,
    });

    this.logs = new LogGroup(this, "Logs", {
      logGroupName: `/dimmi/${config.name}`,
      retention: config.protectData ? RetentionDays.THREE_MONTHS : RetentionDays.TWO_WEEKS,
      removalPolicy: config.protectData ? RemovalPolicy.RETAIN : RemovalPolicy.DESTROY,
    });

    this.loadBalancer = new LoadBalancer(this, "LoadBalancer", {
      environmentName: config.name,
      vpc: props.network.vpc,
    });

    const web = this.service("web", config.services.web);
    this.loadBalancer.attach(web);

    if (config.services.worker) {
      this.service("worker", config.services.worker);
    }

    this.migrateTask();
    this.provisionTask();
    this.alarms(web);
    this.outputs();
  }

  /**
   * One long-running service. 100% minimum and 200% maximum start the new
   * tasks before stopping the old, so a deploy never drops below capacity.
   * Every task checks itself with `dimmi health`, which asks its own admin
   * listener, so a worker with no load balancer is replaced when it hangs.
   */
  private service(name: string, service: ServiceConfig): FargateService {
    const task = this.taskDefinition(name, service.cpu, service.memoryMiB);
    const ports = service.roles.includes("web") ? [APP_PORTS.https, APP_PORTS.http] : [];

    task.addContainer("app", {
      image: ContainerImage.fromEcrRepository(this.props.repository, RELEASE_TAG),
      command: ["serve"],
      environment: { ...this.settings.environment, DIMMI_APP__ROLES: service.roles.join(",") },
      secrets: this.settings.secrets,
      portMappings: ports.map((containerPort) => ({ containerPort })),
      logging: LogDrivers.awsLogs({ logGroup: this.logs, streamPrefix: name }),
      stopTimeout: Duration.seconds(SHUTDOWN_SECONDS + 5),
      healthCheck: {
        command: ["CMD", "/dimmi", "health"],
        interval: Duration.seconds(30),
        timeout: Duration.seconds(5),
        retries: 3,
        startPeriod: Duration.seconds(60),
      },
    });

    const fargate = new FargateService(this, `${capitalize(name)}Service`, {
      serviceName: `dimmi-${this.props.config.name}-${name}`,
      cluster: this.cluster,
      taskDefinition: task,
      desiredCount: this.props.paused ? 0 : service.desiredCount,
      minHealthyPercent: 100,
      maxHealthyPercent: 200,
      circuitBreaker: { enable: true, rollback: true },
      healthCheckGracePeriod: ports.length > 0 ? Duration.seconds(60) : undefined,
      assignPublicIp: this.props.network.tasksArePublic,
      securityGroups: [this.props.network.appSecurityGroup],
      enableExecuteCommand: true,
    });

    this.services.push(fargate);

    return fargate;
  }

  /**
   * Runs `migrate --force` as the schema owner. The release script starts it
   * before rolling the services; migrations only ever add, so the old release
   * keeps working on the new schema while the new one starts.
   */
  private migrateTask(): void {
    const task = this.taskDefinition("migrate", 256, 512);
    task.addContainer("app", {
      image: ContainerImage.fromEcrRepository(this.props.repository, RELEASE_TAG),
      command: ["migrate", "--force"],
      environment: this.settings.environment,
      secrets: {
        ...this.settings.secrets,
        DIMMI_DATABASE__URL: EcsSecret.fromSecretsManager(this.props.data.connections, "owner_database_url"),
      },
      logging: LogDrivers.awsLogs({ logGroup: this.logs, streamPrefix: "migrate" }),
    });
  }

  /**
   * Runs the provisioning SQL as the RDS master user with psql. The SQL
   * travels as a variable so the task needs no image of our own, and the
   * passwords arrive as secrets, never in the task definition.
   */
  private provisionTask(): void {
    const sql = readFileSync(new URL("../database/provision.sql", import.meta.url), "utf8");
    const { database } = this.props.data;
    const task = this.taskDefinition("provision", 256, 512);
    const psql = [
      'printf "%s" "$PROVISION_SQL" | psql -X',
      '-v owner_role="$OWNER_ROLE" -v app_role="$APP_ROLE" -v database="$DATABASE"',
      '-v owner_secret="$OWNER_SECRET" -v app_secret="$APP_SECRET"',
    ].join(" ");

    task.addContainer("psql", {
      image: ContainerImage.fromRegistry(POSTGRES_IMAGE),
      entryPoint: ["sh", "-c"],
      command: [psql],
      environment: {
        PROVISION_SQL: sql,
        OWNER_ROLE: ROLES.owner,
        APP_ROLE: ROLES.app,
        DATABASE: DATABASE_NAME,
        PGHOST: database.instance.dbInstanceEndpointAddress,
        PGPORT: database.instance.dbInstanceEndpointPort,
        PGDATABASE: DATABASE_NAME,
        PGSSLMODE: "require",
      },
      secrets: {
        PGUSER: EcsSecret.fromSecretsManager(database.masterSecret, "username"),
        PGPASSWORD: EcsSecret.fromSecretsManager(database.masterSecret, "password"),
        OWNER_SECRET: EcsSecret.fromSecretsManager(database.ownerSecret, "password"),
        APP_SECRET: EcsSecret.fromSecretsManager(database.appSecret, "password"),
      },
      logging: LogDrivers.awsLogs({ logGroup: this.logs, streamPrefix: "provision" }),
    });
  }

  /**
   * A task definition with its own named roles, so the release role can be
   * allowed to pass exactly these. Every task may use the files bucket, the
   * key-encryption key and mail; nothing else in the account.
   */
  private taskDefinition(name: string, cpu: number, memoryLimitMiB: number): FargateTaskDefinition {
    const { data, mail, config } = this.props;
    const role = (suffix: string) =>
      new Role(this, `${capitalize(name)}${capitalize(suffix)}Role`, {
        roleName: `dimmi-${config.name}-app-${name}-${suffix}`,
        assumedBy: new ServicePrincipal("ecs-tasks.amazonaws.com"),
      });
    const task = new FargateTaskDefinition(this, `${capitalize(name)}Task`, {
      family: `dimmi-${config.name}-${name}`,
      taskRole: role("task"),
      executionRole: role("execution"),
      cpu,
      memoryLimitMiB,
      runtimePlatform: {
        cpuArchitecture: CpuArchitecture.ARM64,
        operatingSystemFamily: OperatingSystemFamily.LINUX,
      },
    });

    data.filesBucket.grantReadWrite(task.taskRole);
    data.filesBucket.grantDelete(task.taskRole);
    data.kek.grantEncryptDecrypt(task.taskRole);
    task.taskRole.addManagedPolicy(mail.applicationPolicy);

    return task;
  }

  private alarms(web: FargateService): void {
    const { config, alarms } = this.props;
    const action = new SnsAction(alarms);
    const minutes = (count: number) => Duration.minutes(count);

    const list = [
      new Alarm(this, "WebCpuAlarm", {
        alarmName: `dimmi-${config.name}-web-cpu`,
        metric: web.metricCpuUtilization({ period: minutes(5) }),
        threshold: 80,
        evaluationPeriods: 3,
        comparisonOperator: ComparisonOperator.GREATER_THAN_THRESHOLD,
        treatMissingData: TreatMissingData.MISSING,
      }),
      new Alarm(this, "WebMemoryAlarm", {
        alarmName: `dimmi-${config.name}-web-memory`,
        metric: web.metricMemoryUtilization({ period: minutes(5) }),
        threshold: 85,
        evaluationPeriods: 3,
        comparisonOperator: ComparisonOperator.GREATER_THAN_THRESHOLD,
        treatMissingData: TreatMissingData.MISSING,
      }),
      new Alarm(this, "DatabaseStorageAlarm", {
        alarmName: `dimmi-${config.name}-database-storage`,
        metric: this.props.data.database.instance.metricFreeStorageSpace({ period: minutes(15) }),
        threshold: 2 * 1024 ** 3,
        evaluationPeriods: 1,
        comparisonOperator: ComparisonOperator.LESS_THAN_THRESHOLD,
        treatMissingData: TreatMissingData.MISSING,
      }),
      new Alarm(this, "DatabaseCpuAlarm", {
        alarmName: `dimmi-${config.name}-database-cpu`,
        metric: this.props.data.database.instance.metricCPUUtilization({ period: minutes(5) }),
        threshold: 80,
        evaluationPeriods: 3,
        comparisonOperator: ComparisonOperator.GREATER_THAN_THRESHOLD,
        treatMissingData: TreatMissingData.MISSING,
      }),
    ];

    for (const alarm of list) {
      alarm.addAlarmAction(action);
    }
  }

  /**
   * What the release script and the DNS script read. Addresses are the load
   * balancer's fixed Elastic IPs, so the host is plain A records.
   */
  private outputs(): void {
    const addresses = this.loadBalancer.addresses.map((address) => address.attrPublicIp);
    new CfnOutput(this, "LoadBalancerAddresses", { value: Fn.join(",", addresses) });
    new CfnOutput(this, "LoadBalancerDnsName", { value: this.loadBalancer.nlb.loadBalancerDnsName });
    new CfnOutput(this, "ClusterName", { value: this.cluster.clusterName });
    new CfnOutput(this, "ServiceNames", { value: this.services.map((service) => service.serviceName).join(",") });
    new CfnOutput(this, "LogGroupName", { value: this.logs.logGroupName });
    new CfnOutput(this, "TaskSubnetIds", { value: this.taskSubnetIds().join(",") });
    new CfnOutput(this, "TaskSecurityGroupId", { value: this.props.network.appSecurityGroup.securityGroupId });
    new CfnOutput(this, "TaskAssignPublicIp", { value: this.props.network.tasksArePublic ? "ENABLED" : "DISABLED" });
  }

  /** Where one-off tasks run: the same subnets as the services. */
  private taskSubnetIds(): string[] {
    const { vpc, tasksArePublic } = this.props.network;
    const subnetType = tasksArePublic ? SubnetType.PUBLIC : SubnetType.PRIVATE_WITH_EGRESS;

    return vpc.selectSubnets({ subnetType }).subnetIds;
  }
}

function capitalize(value: string): string {
  return value.charAt(0).toUpperCase() + value.slice(1);
}
