import { Secret as EcsSecret } from "aws-cdk-lib/aws-ecs";
import type { EnvironmentConfig } from "../config/environment.ts";
import type { DataStack } from "../stacks/data-stack.ts";
import type { MailStack } from "../stacks/mail-stack.ts";
import { APP_PORTS } from "../stacks/network-stack.ts";

/**
 * Seconds the binary gets to finish requests, drain realtime sockets and
 * hand back jobs after SIGTERM. ECS waits a little longer before it kills.
 */
export const SHUTDOWN_SECONDS = 55;

export type AppSettings = {
  environment: Record<string, string>;
  secrets: Record<string, EcsSecret>;
};

/**
 * Every setting the binary runs with, as `DIMMI_` variables, in one place.
 * Plain values go in `environment`; anything secret is injected by ECS from
 * Secrets Manager at task start, so it never appears in a task definition.
 *
 * `migrations.auto` is off: no serving task changes the schema. The one-off
 * migrate task swaps in the owner's connection string instead.
 */
export function appSettings(config: EnvironmentConfig, data: DataStack, mail: MailStack): AppSettings {
  const https = `https://${config.host}`;
  const fromKey = (secret: Parameters<typeof EcsSecret.fromSecretsManager>[0], key?: string) =>
    EcsSecret.fromSecretsManager(secret, key);

  const environment: Record<string, string> = {
    DIMMI_APP__ENV: "production",
    DIMMI_APP__URL: https,
    DIMMI_TELEMETRY__ENVIRONMENT: config.name,
    DIMMI_HTTP__ADDR: `:${APP_PORTS.http}`,
    DIMMI_HTTP__TLS_ADDR: `:${APP_PORTS.https}`,
    DIMMI_HTTP__SHUTDOWN_TIMEOUT: `${SHUTDOWN_SECONDS - 5}s`,
    DIMMI_MIGRATIONS__AUTO: "false",
    DIMMI_DATABASE__MAX_CONNS: String(config.database.poolSize),
    DIMMI_QUEUE__MAX_CONNS: String(config.database.poolSize),
    DIMMI_AWS__REGION: config.region,
    DIMMI_FILESYSTEMS__DEFAULT: "s3",
    DIMMI_FILESYSTEMS__DISKS__S3__DRIVER: "s3",
    DIMMI_FILESYSTEMS__DISKS__S3__BUCKET: data.filesBucket.bucketName,
    DIMMI_FILESYSTEMS__DISKS__S3__REGION: config.region,
    DIMMI_FILESYSTEMS__DISKS__SES_INBOUND__DRIVER: "s3",
    DIMMI_FILESYSTEMS__DISKS__SES_INBOUND__BUCKET: mail.inboundBucket.bucketName,
    DIMMI_FILESYSTEMS__DISKS__SES_INBOUND__REGION: config.region,
    DIMMI_ENCRYPTION__KEK: "awskms",
    DIMMI_ENCRYPTION__KEKS__AWSKMS__DRIVER: "awskms",
    DIMMI_ENCRYPTION__KEKS__AWSKMS__KEY_ARN: data.kek.keyArn,
    DIMMI_MAIL__DEFAULT: "system",
    DIMMI_MAIL__FROM__EMAIL: `no-reply@${config.mail.system}`,
    DIMMI_MAIL__MAILERS__SYSTEM__DRIVER: "ses",
    DIMMI_MAIL__MAILERS__SYSTEM__REGION: config.region,
    DIMMI_MAIL__MAILERS__SYSTEM__CONFIGURATION_SET: mail.systemConfigurationSetName,
    DIMMI_MAIL__MAILERS__CUSTOMER__DRIVER: "ses",
    DIMMI_MAIL__MAILERS__CUSTOMER__REGION: config.region,
    DIMMI_MAIL__MAILERS__CUSTOMER__CONFIGURATION_SET: mail.customerConfigurationSetName,
    DIMMI_MAILBOX__DEFAULT: "ses",
    DIMMI_MAILBOX__SOURCES__SES__DRIVER: "ses",
    DIMMI_MAILBOX__SOURCES__SES__TOPIC_ARNS: mail.inboundTopic.topicArn,
    DIMMI_CHANNELS__EMAIL__INBOUND_DOMAIN: config.mail.inbound,
    DIMMI_CHANNELS__EMAIL__DEFAULT_SENDER_DOMAIN: config.mail.inbound,
    DIMMI_CHANNELS__EMAIL__INBOUND__DRIVERS: "ses",
    DIMMI_CUSTOM_DOMAINS__DEFAULT: "certmagic",
    DIMMI_CUSTOM_DOMAINS__TLS__CERTMAGIC__DRIVER: "certmagic",
    DIMMI_CUSTOM_DOMAINS__TLS__CERTMAGIC__CA: config.acmeDirectory,
    DIMMI_CUSTOM_DOMAINS__TLS__CERTMAGIC__EMAIL: config.alarmEmail,
    DIMMI_SOCIALITE__PROVIDERS__GITHUB__REDIRECT_URL: `${https}/auth/github/callback`,
    DIMMI_SOCIALITE__PROVIDERS__GOOGLE__REDIRECT_URL: `${https}/auth/google/callback`,
    DIMMI_GATE__ENABLED: String(config.gate),
  };

  const secrets: Record<string, EcsSecret> = {
    DIMMI_DATABASE__URL: fromKey(data.connections, "database_url"),
    DIMMI_REDIS__URL: fromKey(data.connections, "redis_url"),
    DIMMI_APP__KEY: fromKey(data.appSecret, "app_key"),
    DIMMI_SOCIALITE__PROVIDERS__GITHUB__CLIENT_ID: fromKey(data.appSecret, "github_client_id"),
    DIMMI_SOCIALITE__PROVIDERS__GITHUB__CLIENT_SECRET: fromKey(data.appSecret, "github_client_secret"),
    DIMMI_SOCIALITE__PROVIDERS__GOOGLE__CLIENT_ID: fromKey(data.appSecret, "google_client_id"),
    DIMMI_SOCIALITE__PROVIDERS__GOOGLE__CLIENT_SECRET: fromKey(data.appSecret, "google_client_secret"),
    ...(config.gate
      ? { DIMMI_GATE__PASSWORD: fromKey(data.gatePassword), DIMMI_GATE__CI_KEY: fromKey(data.gateCiKey) }
      : {}),
  };

  return { environment, secrets };
}
