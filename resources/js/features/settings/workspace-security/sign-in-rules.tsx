/*
 * Which ways of signing in reach the workspace. Every box ticked means no
 * rule at all, so the list stored is empty then, and a way the install
 * adds later stays open. Google can also be held to company domains.
 * The rule bites at the workspace's door, never at sign-in: a person may
 * belong to other workspaces without it.
 */
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { Input } from "@/components/base/input/input";

import { Row, Section } from "../layout";
import { storedMethods } from "./methods";
import { useAccessPolicy } from "./use-access-policy";

const labels: Record<string, string> = {
  password: "Password",
  magic_link: "Email link or code",
  google: "Google",
  github: "GitHub",
  sso: "Single sign-on",
};

export function SignInRulesSection() {
  const { policy, canManage, save, failure, prompt } = useAccessPolicy();
  const [domains, setDomains] = useState<string | null>(null);

  if (!policy) {
    return null;
  }

  const allowed = policy.allowed_sign_in_methods;
  const ticked = allowed.length > 0 ? allowed : policy.available_sign_in_methods;
  const shownDomains = domains ?? policy.google_allowed_domains.join(", ");

  const toggle = (method: string, on: boolean) => {
    const next = on ? [...ticked, method] : ticked.filter((value) => value !== method);

    void save({ allowed_sign_in_methods: storedMethods(next, policy.available_sign_in_methods) });
  };

  const saveDomains = async () => {
    const list = shownDomains
      .split(",")
      .map((domain) => domain.trim())
      .filter(Boolean);

    if (await save({ google_allowed_domains: list })) {
      setDomains(null);
    }
  };

  return (
    <Section
      title="Sign-in methods"
      description="How people must sign in to work in this workspace. Tokens are exempt."
    >
      {policy.available_sign_in_methods.map((method) => (
        <Row key={method} label={labels[method] ?? method}>
          <Checkbox
            aria-label={`Allow ${labels[method] ?? method}`}
            isDisabled={!canManage}
            isSelected={ticked.includes(method)}
            onChange={(on) => toggle(method, on)}
          />
        </Row>
      ))}
      {ticked.includes("google") ? (
        <Row label="Google domains" description="Only accounts of these domains. Leave empty for any Google account.">
          <div className="flex items-end gap-2">
            <Input
              aria-label="Google domains"
              placeholder="acme.com, acme.co.uk"
              isDisabled={!canManage}
              value={shownDomains}
              onChange={setDomains}
            />
            {canManage ? (
              <Button variant="secondary" size="small" onClick={() => void saveDomains()}>
                Save
              </Button>
            ) : null}
          </div>
        </Row>
      ) : null}
      {failure ? (
        <p role="alert" className="py-2.5 pe-3 text-body-2-regular text-text-error-primary">
          {failure}
        </p>
      ) : null}
      {prompt}
    </Section>
  );
}
