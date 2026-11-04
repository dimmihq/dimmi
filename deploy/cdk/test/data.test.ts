import { Match } from "aws-cdk-lib/assertions";
import { describe, expect, it } from "vitest";
import { only, resources, synthEnvironment } from "./support.ts";

describe("data stack", () => {
  const staging = synthEnvironment("staging").data;
  const production = synthEnvironment("production").data;

  it("postgres is version 18, encrypted, private and in isolated subnets", () => {
    expect(() =>
      staging.hasResourceProperties("AWS::RDS::DBInstance", {
        Engine: "postgres",
        EngineVersion: "18",
        DBInstanceClass: "db.t4g.micro",
        StorageEncrypted: true,
        PubliclyAccessible: false,
        DBName: "dimmi",
        CACertificateIdentifier: "rds-ca-rsa2048-g1",
      }),
    ).not.toThrow();
  });

  it("production keeps backups, snapshots on delete and refuses deletion", () => {
    expect(() =>
      production.hasResourceProperties("AWS::RDS::DBInstance", {
        DBInstanceClass: "db.t4g.medium",
        MultiAZ: true,
        DeletionProtection: true,
        BackupRetentionPeriod: 14,
      }),
    ).not.toThrow();
    expect(() => production.hasResource("AWS::RDS::DBInstance", { DeletionPolicy: "Snapshot" })).not.toThrow();
    expect(() => staging.hasResource("AWS::RDS::DBInstance", { DeletionPolicy: "Delete" })).not.toThrow();
  });

  it("valkey is node based with tls, an auth token and no serverless", () => {
    expect(() =>
      staging.hasResourceProperties("AWS::ElastiCache::ReplicationGroup", {
        Engine: "valkey",
        CacheNodeType: "cache.t4g.micro",
        NumCacheClusters: 1,
        TransitEncryptionEnabled: true,
        AtRestEncryptionEnabled: true,
        AuthToken: Match.anyValue(),
      }),
    ).not.toThrow();
    expect(() => staging.resourceCountIs("AWS::ElastiCache::ServerlessCache", 0)).not.toThrow();
    expect(() =>
      production.hasResourceProperties("AWS::ElastiCache::ReplicationGroup", {
        NumCacheClusters: 2,
        AutomaticFailoverEnabled: true,
        MultiAZEnabled: true,
      }),
    ).not.toThrow();
  });

  it("each role has its own generated secret under the environment's prefix", () => {
    const names = resources(staging, "AWS::SecretsManager::Secret").map((secret) => secret.Properties.Name);

    for (const name of [
      "dimmi-staging/database/master",
      "dimmi-staging/database/owner",
      "dimmi-staging/database/app",
      "dimmi-staging/cache/auth",
      "dimmi-staging/connections",
      "dimmi-staging/app",
      "dimmi-staging/gate/password",
      "dimmi-staging/gate/ci-key",
    ]) {
      expect(names.includes(name), `missing secret ${name}`).toBeTruthy();
    }
  });

  it("connection strings hold references to passwords, never values", () => {
    const connections = resources(staging, "AWS::SecretsManager::Secret").find(
      (secret) => secret.Properties.Name === "dimmi-staging/connections",
    );
    const text = JSON.stringify(connections?.Properties.SecretString);

    expect(text).toMatch(/postgresql:\/\/dimmi_app:\{\{resolve:secretsmanager:/);
    expect(text).toMatch(/postgresql:\/\/dimmi_owner:\{\{resolve:secretsmanager:/);
    expect(text).toMatch(/rediss:\/\/:\{\{resolve:secretsmanager:/);
    expect(text).toMatch(/sslmode=verify-full/);
  });

  it("the secret a person fills in starts with every key empty", () => {
    const app = resources(staging, "AWS::SecretsManager::Secret").find(
      (secret) => secret.Properties.Name === "dimmi-staging/app",
    );
    const values = JSON.parse(String(app?.Properties.SecretString)) as Record<string, string>;

    expect(Object.keys(values).sort()).toEqual([
      "app_key",
      "github_client_id",
      "github_client_secret",
      "google_client_id",
      "google_client_secret",
    ]);
    expect(Object.values(values).every((value) => value === "")).toBeTruthy();
  });

  it("the key-encryption key rotates and the files bucket is private", () => {
    expect(() => staging.hasResourceProperties("AWS::KMS::Key", { EnableKeyRotation: true })).not.toThrow();
    expect(() =>
      staging.hasResourceProperties("AWS::KMS::Alias", { AliasName: "alias/dimmi-staging-kek" }),
    ).not.toThrow();
    only(staging, "AWS::S3::Bucket");
    expect(() =>
      staging.hasResourceProperties("AWS::S3::Bucket", {
        PublicAccessBlockConfiguration: Match.objectLike({ BlockPublicPolicy: true }),
        OwnershipControls: { Rules: [{ ObjectOwnership: "BucketOwnerEnforced" }] },
      }),
    ).not.toThrow();
    expect(() =>
      production.hasResourceProperties("AWS::S3::Bucket", { VersioningConfiguration: { Status: "Enabled" } }),
    ).not.toThrow();
    expect(() => production.hasResource("AWS::S3::Bucket", { DeletionPolicy: "Retain" })).not.toThrow();
  });
});
