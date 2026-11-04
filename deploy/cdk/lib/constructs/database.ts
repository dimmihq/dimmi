import { Duration, RemovalPolicy, SecretValue } from "aws-cdk-lib";
import type { ISecurityGroup, IVpc } from "aws-cdk-lib/aws-ec2";
import { InstanceType, SubnetType } from "aws-cdk-lib/aws-ec2";
import {
  CaCertificate,
  Credentials,
  DatabaseInstance,
  DatabaseInstanceEngine,
  PostgresEngineVersion,
  StorageType,
} from "aws-cdk-lib/aws-rds";
import { Secret } from "aws-cdk-lib/aws-secretsmanager";
import { Construct } from "constructs";
import type { EnvironmentConfig } from "../config/environment.ts";

export type DatabaseProps = {
  config: EnvironmentConfig;
  vpc: IVpc;
  securityGroup: ISecurityGroup;
};

/** The database the application uses, created by RDS with the instance. */
export const DATABASE_NAME = "dimmi";

/**
 * Where the RDS certificate bundle sits inside the application image. The
 * connection strings verify the server against it.
 */
export const RDS_CA_BUNDLE = "/etc/ssl/certs/rds-global-bundle.pem";

/**
 * The three PostgreSQL roles. The master user only provisions. The owner owns
 * the schema and runs migrations. The application role owns nothing, so
 * row-level security applies to every query it makes; a table owner would
 * bypass it.
 */
export const ROLES = { master: "dimmi_admin", owner: "dimmi_owner", app: "dimmi_app" } as const;

/**
 * RDS PostgreSQL 18 with a secret per role. Passwords leave out punctuation
 * so they sit in a connection string without escaping.
 */
export class Database extends Construct {
  readonly instance: DatabaseInstance;
  readonly masterSecret: Secret;
  readonly ownerSecret: Secret;
  readonly appSecret: Secret;

  constructor(scope: Construct, id: string, props: DatabaseProps) {
    super(scope, id);

    const { config } = props;
    const { database } = config;
    const prefix = `dimmi-${config.name}/database`;
    const removalPolicy = config.protectData ? RemovalPolicy.SNAPSHOT : RemovalPolicy.DESTROY;

    this.masterSecret = this.roleSecret("MasterSecret", `${prefix}/master`, ROLES.master);
    this.ownerSecret = this.roleSecret("OwnerSecret", `${prefix}/owner`, ROLES.owner);
    this.appSecret = this.roleSecret("AppSecret", `${prefix}/app`, ROLES.app);

    this.instance = new DatabaseInstance(this, "Instance", {
      instanceIdentifier: `dimmi-${config.name}`,
      engine: DatabaseInstanceEngine.postgres({ version: PostgresEngineVersion.VER_18 }),
      instanceType: new InstanceType(database.instanceType),
      vpc: props.vpc,
      vpcSubnets: { subnetType: SubnetType.PRIVATE_ISOLATED },
      securityGroups: [props.securityGroup],
      credentials: Credentials.fromSecret(this.masterSecret),
      databaseName: DATABASE_NAME,
      multiAz: database.multiAz,
      allocatedStorage: database.allocatedStorageGiB,
      maxAllocatedStorage: database.maxAllocatedStorageGiB,
      storageType: StorageType.GP3,
      storageEncrypted: true,
      caCertificate: CaCertificate.RDS_CA_RSA2048_G1,
      backupRetention: Duration.days(database.backupRetentionDays),
      deletionProtection: config.protectData,
      removalPolicy,
      autoMinorVersionUpgrade: true,
      enablePerformanceInsights: false,
      cloudwatchLogsExports: ["postgresql"],
      publiclyAccessible: false,
    });
  }

  /**
   * A connection string for `role`, verified against the RDS bundle. It is a
   * secret value: CloudFormation resolves the password when it writes the
   * secret, so the template only ever holds a reference.
   */
  url(role: "owner" | "app"): SecretValue {
    const secret = role === "owner" ? this.ownerSecret : this.appSecret;
    const user = ROLES[role];
    const password = secret.secretValueFromJson("password").unsafeUnwrap();
    const host = this.instance.dbInstanceEndpointAddress;
    const port = this.instance.dbInstanceEndpointPort;
    const query = `sslmode=verify-full&sslrootcert=${RDS_CA_BUNDLE}`;

    return SecretValue.unsafePlainText(`postgresql://${user}:${password}@${host}:${port}/${DATABASE_NAME}?${query}`);
  }

  private roleSecret(id: string, secretName: string, username: string): Secret {
    return new Secret(this, id, {
      secretName,
      description: `PostgreSQL role ${username}`,
      generateSecretString: {
        secretStringTemplate: JSON.stringify({ username }),
        generateStringKey: "password",
        excludePunctuation: true,
        passwordLength: 40,
      },
    });
  }
}
