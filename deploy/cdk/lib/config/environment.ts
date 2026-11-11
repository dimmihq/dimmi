/**
 * Everything that differs between environments, in one place. Adding an
 * environment is adding one of these to `environments.ts`; no stack reads
 * anything else.
 *
 * AWS holds only mail for an environment: SES sending and receiving, the
 * SNS topics that carry mail events to the application, and the alarms on
 * them. The application itself, its database, cache and files run on the
 * k3s server.
 *
 * Nothing here identifies an AWS account. The account comes from the
 * credentials of whoever deploys, so the repository stays publishable.
 */
export type EnvironmentConfig = {
  /** Lowercase name that prefixes and tags every resource, such as `staging`. */
  name: string;
  /**
   * Where the environment's mail lives. Environments in one account must use
   * different Regions: SES keeps one active receipt rule set, one sandbox
   * status and one reputation per Region.
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
  /**
   * The public IPv4 addresses of the load balancer in front of the k3s
   * server. The host gets one A record for each.
   */
  ingressAddresses: string[];
  /** Who is mailed when an alarm fires. */
  alarmEmail: string;
  /** Monthly AWS spend, in US dollars, at which the alarm address is mailed. */
  budgetUsd: number;
};

const NAME = /^[a-z][a-z0-9-]{1,20}$/;
const HOST = /^(?=.{1,253}$)([a-z0-9]([a-z0-9-]*[a-z0-9])?\.)+[a-z]{2,}$/;
const IPV4 = /^((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)\.){3}(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)$/;

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
  check(config.ingressAddresses.length >= 1, "ingressAddresses must hold at least one address");
  for (const address of config.ingressAddresses) {
    check(IPV4.test(address), `ingress address ${address} is not an IPv4 address`);
  }

  if (problems.length > 0) {
    throw new Error(`environment ${config.name} is invalid:\n  ${problems.join("\n  ")}`);
  }

  return config;
}
