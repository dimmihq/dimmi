/*
 * How the inbox treats links customers send: warnings on suspicious
 * links, domains that skip them, domains that can never be opened, and
 * whether email links show as plain text. Checks run where messages are
 * shown, so a saved change applies at once to every message, old and
 * new. Changes wait in the page's draft until its Save is pressed.
 */
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { Switch } from "@/components/base/switch/switch";
import { Textarea } from "@/components/base/textarea/textarea";
import type { LinkSafety } from "@/features/inbox/links/link-checks";
import { updateWorkspace, workspaceQuery } from "@/lib/api/workspace";

import { Row, Section } from "./layout";

function lines(text: string): string[] {
  return text
    .split(/[\s,]+/)
    .map((line) => line.trim())
    .filter((line) => line !== "");
}

/*
 * What the section shows and edits: the two switches, and each domain
 * list as the text in its box, one domain per line.
 */
interface LinkFields {
  enabled: boolean;
  strip_inbound_email_links: boolean;
  allow: string;
  block: string;
}

function fieldsOf(settings: LinkSafety): LinkFields {
  return {
    enabled: settings.enabled,
    strip_inbound_email_links: settings.strip_inbound_email_links,
    allow: settings.allow_domains.join("\n"),
    block: settings.block_domains.join("\n"),
  };
}

/*
 * useLinkSafetyDraft holds link safety changes not saved yet. A field
 * set back to its saved value drops out, so the draft holds only what
 * differs. send writes what differs and leaves refreshing and clearing
 * to the page that owns the Save.
 */
export function useLinkSafetyDraft(workspace: string) {
  const { data } = useQuery(workspaceQuery(workspace));
  const saved = data?.link_safety ? fieldsOf(data.link_safety) : null;
  const [pending, setPending] = useState<Partial<LinkFields>>({});
  const change = (): Partial<LinkSafety> => ({
    ...(pending.enabled === undefined ? {} : { enabled: pending.enabled }),
    ...(pending.strip_inbound_email_links === undefined
      ? {}
      : { strip_inbound_email_links: pending.strip_inbound_email_links }),
    ...(pending.allow === undefined ? {} : { allow_domains: lines(pending.allow) }),
    ...(pending.block === undefined ? {} : { block_domains: lines(pending.block) }),
  });

  return {
    shown: saved ? { ...saved, ...pending } : null,
    dirty: Object.keys(pending).length > 0,
    set: <K extends keyof LinkFields>(key: K, value: LinkFields[K]) =>
      setPending((before) => {
        const { [key]: _, ...rest } = before;

        return saved && saved[key] === value ? rest : { ...rest, [key]: value };
      }),
    send: () =>
      Object.keys(pending).length > 0 ? updateWorkspace(workspace, { link_safety: change() }) : Promise.resolve(null),
    discard: () => setPending({}),
  };
}

export type LinkSafetyDraft = ReturnType<typeof useLinkSafetyDraft>;

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
