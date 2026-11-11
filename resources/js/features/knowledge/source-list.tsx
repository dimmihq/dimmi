/*
 * Settings → Knowledge: the sources dimmi reads besides the help center.
 * Each row says what kind of source it is, how many documents it holds,
 * when it last synced and how it stands; when it syncs next and what it
 * feeds wait on the source's own page, so the table fits without
 * scrolling sideways. A row opens the source's page. A source whose sync
 * failed raises a banner at the top, since the agent keeps answering
 * from an old copy until someone fixes it. Under the sources, what they
 * cost to embed and crawl this month.
 */
import { RiAddLine, RiErrorWarningLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Table, TableBody, TableCell, TableColumn, TableHeader, TableRow } from "@/components/base/table/table";
import { toast } from "@/components/base/toast/toast";
import { usePermission } from "@/features/settings/inboxes";
import { landingOf } from "@/features/settings/integrations";
import { RowsSkeleton, Section, SettingsPage } from "@/features/settings/layout";
import { useAccount } from "@/features/settings/use-account";
import { type KnowledgeSource, type SourceType, sourcesQuery, sourceTypesQuery } from "@/lib/api/knowledge-sources";

import { AddSourceModal, type Resume } from "./add-source";
import { SourceLogo } from "./logos";
import { ago, broken, sourceState } from "./model";
import { UsageSection } from "./usage-section";

export function KnowledgeSettings() {
  const workspace = useAccount().workspace.id;
  const canManage = usePermission(workspace, "knowledge:manage");
  const list = useQuery(sourcesQuery(workspace));
  const types = useQuery(sourceTypesQuery(workspace));
  const [resume] = useState(resumeOf);
  const [adding, setAdding] = useState(resume !== null);
  const failing = (list.data ?? []).filter(broken);

  return (
    <SettingsPage
      title="Knowledge"
      description="What dimmi reads to answer customers and help your team, besides your help center."
      actions={
        canManage ? (
          <Button size="small" leadingIcon={RiAddLine} onClick={() => setAdding(true)}>
            Add source
          </Button>
        ) : null
      }
    >
      {failing.length > 0 ? <BrokenBanner sources={failing} /> : null}
      <Section title="Sources" loading={list.isPending}>
        {list.isPending ? (
          <RowsSkeleton rows={3} />
        ) : (
          <SourceTable
            sources={list.data ?? []}
            types={types.data ?? []}
            onAdd={canManage ? () => setAdding(true) : undefined}
          />
        )}
      </Section>
      <UsageSection workspace={workspace} />
      <AddSourceModal workspace={workspace} isOpen={adding} resume={resume} onOpenChange={setAdding} />
    </SettingsPage>
  );
}

/*
 * resumeOf reads, once, where the add flow stood when the browser left
 * for a service's consent screen, and takes it out of the address.
 */
function resumeOf(): Resume | null {
  const params = new URLSearchParams(window.location.search);
  const type = params.get("add");
  if (!type) {
    return null;
  }
  const landing = landingOf(window.location.search);
  window.history.replaceState(window.history.state, "", window.location.pathname);
  if (landing?.kind === "error") {
    toast.error(landing.message);
  }

  return { type, installation: landing?.kind === "connected" ? landing.id : null };
}

/*
 * BrokenBanner names the sources whose last sync failed.
 */
export function BrokenBanner({ sources }: { sources: KnowledgeSource[] }) {
  const names = sources.map((s) => s.name).join(", ");

  return (
    <div
      role="alert"
      className="flex items-start gap-3 rounded-2xl border border-border-error-default bg-background-secondary-default p-4"
    >
      <RiErrorWarningLine aria-hidden className="mt-0.5 size-5 shrink-0 text-text-error-primary" />
      <div className="flex min-w-0 flex-col gap-0.5">
        <span className="text-body-medium text-text-primary">
          {sources.length === 1 ? `${names} stopped syncing` : `${sources.length} sources need attention`}
        </span>
        <span className="text-body-2-regular text-text-secondary">
          {sources.length === 1 && sources[0].last_error
            ? sources[0].last_error
            : `dimmi keeps answering from the last copy of ${names} until they sync again.`}
        </span>
      </div>
    </div>
  );
}

function SourceTable({
  sources,
  types,
  onAdd,
}: {
  sources: KnowledgeSource[];
  types: SourceType[];
  onAdd?: () => void;
}) {
  const navigate = useNavigate();
  const typeOf = (type: string) => types.find((t) => t.type === type);

  if (sources.length === 0) {
    return (
      <div className="flex flex-col items-start gap-3 py-4 pe-3">
        <p className="text-body-2-regular text-text-tertiary">
          No sources yet. Add your docs site, a repository, Notion, or import your Intercom or Zendesk help center.
        </p>
        {onAdd ? (
          <Button variant="secondary" size="small" leadingIcon={RiAddLine} onClick={onAdd}>
            Add source
          </Button>
        ) : null}
      </div>
    );
  }

  return (
    <div className="-ms-3">
      <Table
        aria-label="Knowledge sources"
        size="sm"
        className="[&_th]:border-t-0"
        onRowAction={(key) => navigate({ to: "/d/settings/knowledge/$source", params: { source: String(key) } })}
      >
        <TableHeader>
          <TableColumn isRowHeader>Source</TableColumn>
          <TableColumn>Documents</TableColumn>
          <TableColumn>Last sync</TableColumn>
          <TableColumn>Status</TableColumn>
        </TableHeader>
        <TableBody items={sources} dependencies={[types]}>
          {(s) => {
            const kind = typeOf(s.type);
            const state = sourceState(s);
            return (
              <TableRow id={s.id} className="cursor-pointer">
                <TableCell>
                  <span className="flex min-w-0 items-center gap-3">
                    <SourceLogo logo={kind?.logo ?? s.type} />
                    <span className="flex min-w-0 flex-col">
                      <span className="truncate text-body-2-medium text-text-primary">{s.name}</span>
                      <span className="text-caption-1-regular text-text-tertiary">
                        {kind?.name ?? s.type} · {s.mode === "import" ? "Import" : "Sync"}
                      </span>
                    </span>
                  </span>
                </TableCell>
                <TableCell>
                  <span className="text-body-2-regular text-text-secondary">{s.document_count}</span>
                </TableCell>
                <TableCell>
                  <span className="whitespace-nowrap text-body-2-regular text-text-secondary">
                    {ago(s.last_synced_at)}
                  </span>
                </TableCell>
                <TableCell>
                  <StateChip state={state} />
                </TableCell>
              </TableRow>
            );
          }}
        </TableBody>
      </Table>
    </div>
  );
}

const stateColors = { green: "lime", orange: "orange", red: "rose", gray: "gray", blue: "blue" } as const;

export function StateChip({ state }: { state: ReturnType<typeof sourceState> }) {
  return (
    <Chip variant="caption" color={stateColors[state.tone]}>
      {state.label}
    </Chip>
  );
}
