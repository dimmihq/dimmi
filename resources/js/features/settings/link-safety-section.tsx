/*
 * How the inbox treats links customers send: warnings on suspicious
 * links, domains that skip them, domains that can never be opened, and
 * whether email links show as plain text. Checks run where messages are
 * shown, so a saved change applies at once to every message, old and
 * new. Changes wait in the page's draft until its Save is pressed.
 */
import { Switch } from "@/components/base/switch/switch";
import { Textarea } from "@/components/base/textarea/textarea";

import { Row, Section } from "./layout";
import type { LinkSafetyDraft } from "./link-safety-draft";

export function LinkSafetySection({ canManage, draft }: { canManage: boolean; draft: LinkSafetyDraft }) {
  const shown = draft.shown;
  if (!shown) {
    return <Section title="Link safety" loading rows={2} />;
  }

  return (
    <Section
      title="Link safety"
      description="Checks run in your browser when a message is shown. No one outside your workspace sees the links."
    >
      <Row
        label="Warn about suspicious links"
        description="Asks before opening a link whose text names another site, uses look-alike letters, a shortener or a bare address."
      >
        <Switch
          aria-label="Warn about suspicious links"
          size="sm"
          isSelected={shown.enabled}
          isDisabled={!canManage}
          onChange={(on) => draft.set("enabled", on)}
        />
      </Row>
      <Row
        label="Hide links in email"
        description="Shows the links of incoming email as plain text. Teammates who can read raw email can show them."
      >
        <Switch
          aria-label="Hide links in email"
          size="sm"
          isSelected={shown.strip_inbound_email_links}
          isDisabled={!canManage}
          onChange={(on) => draft.set("strip_inbound_email_links", on)}
        />
      </Row>
      <div className="flex flex-col gap-3 py-3 pe-3">
        <Textarea
          label="Allowed domains"
          hint="Links to these domains and the sites under them open without a warning. One per line."
          rows={3}
          autoResize
          maxRows={10}
          isDisabled={!canManage}
          value={shown.allow}
          onChange={(value) => draft.set("allow", value)}
        />
        <Textarea
          label="Blocked domains"
          hint="Links to these domains cannot be opened from a conversation. One per line."
          rows={3}
          autoResize
          maxRows={10}
          isDisabled={!canManage}
          value={shown.block}
          onChange={(value) => draft.set("block", value)}
        />
      </div>
    </Section>
  );
}
