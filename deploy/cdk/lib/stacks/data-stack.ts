import type { StackProps } from "aws-cdk-lib";
import { CfnOutput, Duration, RemovalPolicy, SecretValue, Stack } from "aws-cdk-lib";
import type { ISecurityGroup, IVpc } from "aws-cdk-lib/aws-ec2";
import { Key } from "aws-cdk-lib/aws-kms";
import { BlockPublicAccess, Bucket, BucketEncryption, ObjectOwnership } from "aws-cdk-lib/aws-s3";
import { Secret } from "aws-cdk-lib/aws-secretsmanager";
import type { Construct } from "constructs";
import type { EnvironmentConfig } from "../config/environment.ts";
import { Database } from "../constructs/database.ts";
import { Valkey } from "../constructs/valkey.ts";

export type DataStackProps = StackProps & {
  config: EnvironmentConfig;
  vpc: IVpc;
  databaseSecurityGroup: ISecurityGroup;
  cacheSecurityGroup: ISecurityGroup;
};

/**
 * Keys a person fills in once, because AWS cannot make them. The app key is
 * `base64:` and 32 random bytes; the OAuth pairs come from each provider's
 * console. Every key must exist, empty or not, or ECS refuses to start a task.
 * The stack writes them empty once; a deploy that leaves this list alone never
 * touches the values a person has filled in.
 */
export const HUMAN_SECRET_KEYS = [
  "app_key",
  "github_client_id",
  "github_client_secret",
  "google_client_id",
  "google_client_secret",
] as const;

/**
 * Everything that holds state: PostgreSQL, Valkey, the files bucket, the
 * key-encryption key and the secrets. Containers keep nothing on disk, so
 * losing every task loses nothing; losing this stack loses everything, which
 * is why production retains and protects it.
 */
export class DataStack extends Stack {
  readonly database: Database;
  readonly valkey: Valkey;
  readonly filesBucket: Bucket;
  readonly kek: Key;
  /** `database_url`, `owner_database_url` and `redis_url`, composed by CloudFormation. */
  readonly connections: Secret;
  /** The values in `HUMAN_SECRET_KEYS`. */
  readonly appSecret: Secret;
  /** What people type to pass the gate. */
  readonly gatePassword: Secret;
  /** What automated checks send to pass the gate. */
  readonly gateCiKey: Secret;

  constructor(scope: Construct, id: string, props: DataStackProps) {
    super(scope, id, props);

    const { config } = props;
    const prefix = `dimmi-${config.name}`;
    const removalPolicy = config.protectData ? RemovalPolicy.RETAIN : RemovalPolicy.DESTROY;

    this.database = new Database(this, "Database", {
      config,
      vpc: props.vpc,
      securityGroup: props.databaseSecurityGroup,
    });
    this.valkey = new Valkey(this, "Valkey", { config, vpc: props.vpc, securityGroup: props.cacheSecurityGroup });

    this.kek = new Key(this, "KeyEncryptionKey", {
      alias: `alias/${prefix}-kek`,
      description: `Wraps the data keys dimmi ${config.name} encrypts secrets with`,
      enableKeyRotation: true,
      removalPolicy,
      pendingWindow: Duration.days(config.protectData ? 30 : 7),
    });

    this.filesBucket = new Bucket(this, "FilesBucket", {
      blockPublicAccess: BlockPublicAccess.BLOCK_ALL,
      objectOwnership: ObjectOwnership.BUCKET_OWNER_ENFORCED,
      encryption: BucketEncryption.S3_MANAGED,
      enforceSSL: true,
      versioned: config.protectData,
      removalPolicy,
      autoDeleteObjects: !config.protectData,
    });

    this.connections = new Secret(this, "Connections", {
      secretName: `${prefix}/connections`,
      description: "Connection strings for the application and for migrations",
      secretObjectValue: {
        database_url: this.database.url("app"),
        owner_database_url: this.database.url("owner"),
        redis_url: this.valkey.url(),
      },
    });

    this.appSecret = new Secret(this, "AppSecret", {
      secretName: `${prefix}/app`,
      description: "Values only a person can provide; see docs/deploy",
      secretStringValue: SecretValue.unsafePlainText(
        JSON.stringify(Object.fromEntries(HUMAN_SECRET_KEYS.map((key) => [key, ""]))),
      ),
    });

    this.gatePassword = this.generated("GatePassword", `${prefix}/gate/password`, "Shared password for the gate");
    this.gateCiKey = this.generated("GateCiKey", `${prefix}/gate/ci-key`, "Key automated checks pass the gate with");

    new CfnOutput(this, "DatabaseEndpoint", { value: this.database.instance.dbInstanceEndpointAddress });
    new CfnOutput(this, "FilesBucketName", { value: this.filesBucket.bucketName });
    new CfnOutput(this, "KeyEncryptionKeyArn", { value: this.kek.keyArn });
  }

  private generated(id: string, secretName: string, description: string): Secret {
    return new Secret(this, id, {
      secretName,
      description,
      generateSecretString: { excludePunctuation: true, passwordLength: 40 },
    });
  }
}
