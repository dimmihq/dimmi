import type { StackProps } from "aws-cdk-lib";
import { Stack } from "aws-cdk-lib";
import { CfnOIDCProvider } from "aws-cdk-lib/aws-iam";
import type { Construct } from "constructs";

/** GitHub's token issuer, without the scheme, as IAM names the provider. */
export const GITHUB_ISSUER = "token.actions.githubusercontent.com";

/** Where account-wide resources live. IAM is global; the stack needs a home. */
export const ACCOUNT_REGION = "us-east-1";

/**
 * What the account holds once, whatever the environments: the identity
 * provider that lets GitHub Actions exchange its OIDC token for short-lived
 * credentials. No access key is ever created for automation.
 *
 * It uses the CloudFormation resource directly; the higher-level construct
 * adds a Lambda function for something CloudFormation now does itself.
 */
export class AccountStack extends Stack {
  constructor(scope: Construct, id: string, props: StackProps) {
    super(scope, id, props);

    new CfnOIDCProvider(this, "GitHubProvider", {
      url: `https://${GITHUB_ISSUER}`,
      clientIdList: ["sts.amazonaws.com"],
    });
  }
}
