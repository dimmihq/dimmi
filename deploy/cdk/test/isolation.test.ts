import { describe, expect, it } from "vitest";
import { synthEnvironment } from "./support.ts";

const ACCOUNT_ID = /(?<![0-9A-Za-z])[0-9]{12}(?![0-9A-Za-z])/;

describe("environment isolation", () => {
  for (const [name, other] of [
    ["staging", "production"],
    ["production", "staging"],
  ] as const) {
    const stacks = synthEnvironment(name);

    it(`${name} templates hold no account id`, () => {
      for (const [role, template] of Object.entries(stacks)) {
        expect(JSON.stringify(template.toJSON()), `${role} holds an account id`).not.toMatch(ACCOUNT_ID);
      }
    });

    it(`${name} resources never take ${other}'s names`, () => {
      for (const [role, template] of Object.entries(stacks)) {
        const text = JSON.stringify(template.toJSON());
        const allowed = role === "ci" && name === "production" ? text.replace(/dimmi-staging"/g, "") : text;

        expect(allowed, `${role} names ${other}`).not.toMatch(new RegExp(`dimmi-${other}\\b`));
      }
    });
  }
});
