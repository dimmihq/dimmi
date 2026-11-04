import type { EnvironmentConfig } from "./environment.ts";
import { validate } from "./environment.ts";

const REPOSITORY = "dimmihq/dimmi";
const LETS_ENCRYPT_STAGING = "https://acme-staging-v02.api.letsencrypt.org/directory";
const LETS_ENCRYPT = "https://acme-v02.api.letsencrypt.org/directory";

/**
 * Staging is production in miniature: the same services on the smallest
 * sizes, one task running every role, and data that can be thrown away.
 */
const staging: EnvironmentConfig = {
  name: "staging",
  region: "us-east-2",
  host: "staging.trydimmi.com",
  dnsZone: "trydimmi.com",
  mail: { inbound: "inbound.staging.trydimmi.com", system: "mail.staging.trydimmi.com" },
  alarmEmail: "ops@trydimmi.com",
  acmeDirectory: LETS_ENCRYPT_STAGING,
  gate: true,
  github: { repository: REPOSITORY, environment: "staging" },
  network: { maxAzs: 2, natGateways: 0 },
  database: {
    instanceType: "t4g.micro",
    allocatedStorageGiB: 20,
    maxAllocatedStorageGiB: 50,
    multiAz: false,
    backupRetentionDays: 1,
    poolSize: 5,
  },
  cache: { nodeType: "cache.t4g.micro", replicas: 0 },
  services: {
    web: { roles: ["web", "worker", "scheduler"], cpu: 256, memoryMiB: 1024, desiredCount: 1 },
  },
  protectData: false,
  budgetUsd: 100,
};

/**
 * Production runs several web tasks across zones and a separate worker
 * service, so a slow job never holds up a request and either side scales
 * alone.
 */
const production: EnvironmentConfig = {
  name: "production",
  region: "us-east-1",
  host: "trydimmi.com",
  dnsZone: "trydimmi.com",
  mail: { inbound: "inbound.trydimmi.com", system: "mail.trydimmi.com" },
  alarmEmail: "ops@trydimmi.com",
  acmeDirectory: LETS_ENCRYPT,
  gate: false,
  promotesFrom: "staging",
  github: { repository: REPOSITORY, environment: "production" },
  network: { maxAzs: 2, natGateways: 0 },
  database: {
    instanceType: "t4g.medium",
    allocatedStorageGiB: 50,
    maxAllocatedStorageGiB: 500,
    multiAz: true,
    backupRetentionDays: 14,
    poolSize: 10,
  },
  cache: { nodeType: "cache.t4g.small", replicas: 1 },
  services: {
    web: { roles: ["web"], cpu: 512, memoryMiB: 1024, desiredCount: 2 },
    worker: { roles: ["worker", "scheduler"], cpu: 512, memoryMiB: 2048, desiredCount: 2 },
  },
  protectData: true,
  budgetUsd: 600,
};

/** Every environment this app can deploy, by name. */
export const environments: Record<string, EnvironmentConfig> = Object.fromEntries(
  [staging, production].map((config) => [config.name, validate(config)]),
);

/** The environment called `name`, or an error listing the ones that exist. */
export function environmentNamed(name: string | undefined): EnvironmentConfig {
  const config = name ? environments[name] : undefined;
  if (!config) {
    const known = Object.keys(environments).join(", ");
    throw new Error(`choose an environment with -c env=<name>; known: ${known}`);
  }

  return config;
}
