#!/usr/bin/env node
import { App } from "aws-cdk-lib";
import { environmentNamed } from "../lib/config/environments.ts";
import { buildEnvironment } from "../lib/environment-stacks.ts";

/**
 * One app, every environment. `-c env=staging` picks which one to
 * synthesize, so `cdk deploy --all -c env=staging` touches staging alone.
 * The account comes from the deploying credentials and is never written
 * down; synth runs without any.
 */
const app = new App();

buildEnvironment(app, environmentNamed(app.node.tryGetContext("env")), process.env.CDK_DEFAULT_ACCOUNT);
