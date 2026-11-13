/*
 * The draft of the workspace's link safety: the switches and domain
 * lists changed but not saved yet, and the change a save sends.
 */
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import type { LinkSafety } from "@/features/inbox/links/link-checks";
import { updateWorkspace, workspaceQuery } from "@/lib/api/workspace";

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
