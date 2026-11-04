/**
 * The size and shape of one Fargate service. `roles` is what the binary runs
 * in that task: any mix of web, worker and scheduler.
 */
export type ServiceConfig = {
  roles: ("web" | "worker" | "scheduler")[];
  /** Fargate CPU units: 256 is a quarter of a vCPU. */
  cpu: number;
  memoryMiB: number;
  desiredCount: number;
};

/**
 * Everything that differs between environments, in one place. Adding an
 * environment is adding one of these to `environments.ts`; no stack reads
 * anything else to decide how big or how protected it is.
 *
 * Nothing here identifies an AWS account. The account comes from the
 * credentials of whoever deploys, so the repository stays publishable.
 */
export type EnvironmentConfig = {
  /** Lowercase name that prefixes and tags every resource, such as `staging`. */
  name: string;
  /**
   * Where everything for this environment lives. Environments in one account
   * must use different Regions: SES keeps one active receipt rule set, one
   * sandbox status and one reputation per Region.
   */
  region: string;
  /** The one public host. It serves the dashboard, help center and marketing. */
  host: string;
  /** The DNS zone at Vercel that holds the host and the mail domains. */
  dnsZone: string;
  /**
   * Mail domains, which are DNS only and never websites. `inbound` receives
   * customer mail and sends from customer addresses; `system` sends sign-up,
   * reset and notification mail.
   */
  mail: { inbound: string; system: string };
  /** Who is mailed when an alarm fires. */
  alarmEmail: string;
  /**
   * The ACME directory certificates come from. Both environments use Let's
   * Encrypt's real one: SNS refuses certificates from its staging CA, and
   * certificates live in PostgreSQL, so restarts never order again.
   */
  acmeDirectory: string;
  /** Turns on the shared-password gate in front of the application. */
  gate: boolean;
  /**
   * The environment whose tested images this one deploys. Production takes
   * the exact image staging ran rather than building its own.
   */
  promotesFrom?: string;
  /** GitHub repository and environment whose workflow runs may deploy here. */
  github: { repository: string; environment: string };
  network: {
    maxAzs: number;
    /**
     * Zero puts tasks in public subnets with public addresses, which costs
     * a few dollars where a NAT gateway costs over thirty per zone.
     */
    natGateways: number;
  };
  database: {
    instanceType: string;
    allocatedStorageGiB: number;
    maxAllocatedStorageGiB: number;
    multiAz: boolean;
    backupRetentionDays: number;
    /** Connections each process opens; RDS caps the total by instance memory. */
    poolSize: number;
  };
  cache: { nodeType: string; replicas: number };
  /**
   * `web` always exists and sits behind the load balancer. `worker`, when set,
   * is a second service with no listener; without it the web tasks also run
   * the worker and scheduler roles.
   */
  services: { web: ServiceConfig; worker?: ServiceConfig };
  /** On in production: deletion protection, retained data and final snapshots. */
  protectData: boolean;
  /** Monthly spend, in US dollars, at which the alarm address is mailed. */
  budgetUsd: number;
};

const NAME = /^[a-z][a-z0-9-]{1,20}$/;
const HOST = /^(?=.{1,253}$)([a-z0-9]([a-z0-9-]*[a-z0-9])?\.)+[a-z]{2,}$/;

/**
 * Fails synth early, with the field named, instead of letting CloudFormation
 * fail halfway through a deploy.
 */
export function validate(config: EnvironmentConfig): EnvironmentConfig {
  const problems: string[] = [];
  const check = (ok: boolean, problem: string) => {
    if (!ok) {
      problems.push(problem);
    }
  };

  check(NAME.test(config.name), `name ${config.name}: lowercase letters, digits and dashes`);
  for (const [field, host] of Object.entries({ host: config.host, ...config.mail })) {
    check(HOST.test(host), `${field} ${host} is not a lowercase host name`);
  }
  for (const host of [config.host, config.mail.inbound, config.mail.system]) {
    check(host === config.dnsZone || host.endsWith(`.${config.dnsZone}`), `${host} is not inside ${config.dnsZone}`);
  }
  check(config.mail.inbound !== config.mail.system, "mail.inbound and mail.system must differ");
  check(config.services.web.roles.includes("web"), "services.web must run the web role");
  check(config.services.web.desiredCount >= 1, "services.web.desiredCount must be at least 1");
  check(config.network.maxAzs >= 2, "network.maxAzs must be at least 2: RDS and the load balancer need two zones");
  check(config.database.poolSize >= 1, "database.poolSize must be at least 1");

  const roles = [...config.services.web.roles, ...(config.services.worker?.roles ?? [])];
  for (const role of ["worker", "scheduler"] as const) {
    check(roles.includes(role), `no service runs the ${role} role`);
  }

  if (problems.length > 0) {
    throw new Error(`environment ${config.name} is invalid:\n  ${problems.join("\n  ")}`);
  }

  return config;
}
