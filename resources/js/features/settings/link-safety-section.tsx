/*
 * How the inbox treats links customers send: warnings on suspicious
 * links, domains that skip them, domains that can never be opened, and
 * whether email links show as plain text. Checks run where messages are
 * shown, so a change applies at once to every message, old and new.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Switch } from "@/components/base/switch/switch";
import { Textarea } from "@/components/base/textarea/textarea";
import type { LinkSafety } from "@/features/inbox/links/link-checks";
import { updateWorkspace, workspaceQuery } from "@/lib/api/workspace";

import { Row, Section } from "./layout";
import { failureText } from "./teams";
import { useAccount } from "./use-account";

function lines(text: string): string[] {
  return text
    .split(/[\s,]+/)
    .map((line) => line.trim())
    .filter((line) => line !== "");
}

function useSave(workspace: string) {
  const client = useQueryClient();

  return useMutation({
    mutationFn: (change: Partial<LinkSafety>) => updateWorkspace(workspace, { link_safety: change }),
    onSuccess: (saved) => client.setQueryData(workspaceQuery(workspace).queryKey, saved),
  });
}

function DomainLists({ settings, canManage }: { settings: LinkSafety; canManage: boolean }) {
  const workspace = useAccount().workspace.id;
  const save = useSave(workspace);
  const [allow, setAllow] = useState(settings.allow_domains.join("\n"));
  const [block, setBlock] = useState(settings.block_domains.join("\n"));

  return (
    <div className="flex flex-col gap-3 py-3 pe-3">
      <Textarea
        label="Allowed domains"
        hint="Links to these domains and the sites under them open without a warning. One per line."
        rows={3}
        autoResize
        maxRows={10}
        isDisabled={!canManage}
        value={allow}
        onChange={setAllow}
      />
      <Textarea
        label="Blocked domains"
        hint="Links to these domains cannot be opened from a conversation. One per line."
        rows={3}
        autoResize
        maxRows={10}
        isDisabled={!canManage}
        value={block}
        onChange={setBlock}
      />
      {save.error ? <p className="text-body-2-regular text-text-error-primary">{failureText(save.error)}</p> : null}
      {canManage ? (
        <Button
          size="small"
          variant="secondary"
          className="self-start"
          loading={save.isPending}
          onClick={() => save.mutate({ allow_domains: lines(allow), block_domains: lines(block) })}
        >
          Save domains
        </Button>
      ) : null}
    </div>
  );
}

export function LinkSafetySection({ canManage }: { canManage: boolean }) {
  const workspace = useAccount().workspace.id;
  const { data } = useQuery(workspaceQuery(workspace));
  const save = useSave(workspace);
  const settings = data?.link_safety;
  if (!settings) {
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
          isSelected={settings.enabled}
          isDisabled={!canManage}
          onChange={(on) => save.mutate({ enabled: on })}
        />
      </Row>
      <Row
        label="Hide links in email"
        description="Shows the links of incoming email as plain text. Teammates who can read raw email can show them."
      >
        <Switch
          aria-label="Hide links in email"
          size="sm"
          isSelected={settings.strip_inbound_email_links}
          isDisabled={!canManage}
          onChange={(on) => save.mutate({ strip_inbound_email_links: on })}
        />
      </Row>
      <DomainLists
        key={`${settings.allow_domains.join()}|${settings.block_domains.join()}`}
        settings={settings}
        canManage={canManage}
      />
    </Section>
  );
}
