import { describe, expect, it } from "vitest";
import { only, synthEnvironment } from "./support.ts";

describe("ci stack", () => {
  const staging = synthEnvironment("staging");
  const production = synthEnvironment("production");

  it("only the matching github environment may assume the deploy role", () => {
    const role = only(staging.ci, "AWS::IAM::Role");
    const trust = JSON.stringify(role.Properties.AssumeRolePolicyDocument);

    expect(trust).toMatch(/sts:AssumeRoleWithWebIdentity/);
    expect(trust).toMatch(/"token.actions.githubusercontent.com:sub":"repo:dimmihq\/dimmi:environment:staging"/);
    expect(trust).toMatch(/"token.actions.githubusercontent.com:aud":"sts.amazonaws.com"/);
  });

  it("the deploy role cannot change infrastructure", () => {
    const policy = JSON.stringify(staging.ci.toJSON());

    for (const action of ["cloudformation:CreateChangeSet", "cloudformation:UpdateStack", "iam:CreateRole", '"*"]']) {
      expect(!policy.includes(`"${action}"`), `deploy role must not hold ${action}`).toBeTruthy();
    }
    expect(policy).toMatch(/iam:PassedToService/);
  });

  it("the staging role names nothing in production, and the reverse only reads staging images", () => {
    expect(JSON.stringify(staging.ci.toJSON())).not.toMatch(/production/);

    const promotion = JSON.stringify(production.ci.toJSON());
    expect(promotion).toMatch(/PullPromotedImages/);
    expect(promotion).toMatch(/:ecr:us-east-2:/);
    expect(promotion).toMatch(/:repository\/dimmi-staging"/);
  });

  it("the github identity provider is created once for the account", () => {
    expect(() => staging.account.resourceCountIs("AWS::IAM::OIDCProvider", 1)).not.toThrow();
    expect(staging.account.toJSON()).toEqual(production.account.toJSON());
  });
});
