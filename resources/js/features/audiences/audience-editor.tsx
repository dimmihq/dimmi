/*
 * The audience editor: its name and what it is for, the rule built as
 * sentences, and beside it who the rule matches now, a way to test one
 * person, and what uses the audience. Members who may only read see the
 * same page without controls. An audience something live uses cannot be
 * deleted; the page lists what to change first.
 */
import { RiArrowLeftSLine, RiCloseLine, RiGroupLine, RiUserSearchLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Drawer } from "@/components/base/drawer/drawer";
import { Input } from "@/components/base/input/input";
import { AlertDialog } from "@/components/base/modal/modal";
import { Textarea } from "@/components/base/textarea/textarea";
import { usePermission } from "@/features/settings/inboxes";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { useWide } from "@/features/shell/use-wide";
import { type Audience, type AudienceUser, audienceFieldsQuery, audiencesQuery } from "@/lib/api/audiences";
import { tagsQuery } from "@/lib/api/tags";

import { PreviewPane } from "./preview-pane";
import { type Lookups, RuleBuilder } from "./rule-builder";
import { summary } from "./rules";
import { TestPerson } from "./test-person";
import { type AudienceDraft, useAudienceDraft } from "./use-audience-draft";

function Card({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section aria-label={title} className="flex flex-col gap-2">
      <h2 className="px-3 text-body-2-medium text-text-secondary">{title}</h2>
      <div className="flex flex-col gap-3 rounded-2xl bg-background-secondary-default p-4">{children}</div>
    </section>
  );
}

const linkClass = "truncate text-body-2-regular text-text-primary underline";

function userKind(u: AudienceUser): string {
  if (u.object === "workflow") {
    return u.live ? "Live workflow" : "Workflow draft";
  }

  return { audience: "Audience", article: "Article", collection: "Collection" }[u.object];
}

function UserLink({ user }: { user: AudienceUser }) {
  switch (user.object) {
    case "workflow":
      return (
        <Link to="/d/workflows/$workflow" params={{ workflow: user.id }} className={linkClass}>
          {user.name}
        </Link>
      );
    case "article":
      return (
        <Link to="/d/docs/$article" params={{ article: user.id }} className={linkClass}>
          {user.name}
        </Link>
      );
    case "audience":
      return (
        <Link to="/d/customers/audiences/$audience" params={{ audience: user.id }} className={linkClass}>
          {user.name}
        </Link>
      );
    default:
      return <span className="truncate text-body-2-regular text-text-primary">{user.name}</span>;
  }
}

function UsedBy({ draft }: { draft: AudienceDraft }) {
  const users = draft.server.used_by ?? [];
  if (users.length === 0) {
    return <p className="text-body-2-regular text-text-secondary">Nothing uses this audience yet.</p>;
  }

  return (
    <ul className="flex flex-col gap-2">
      {users.map((u) => (
        <li key={u.id} className="flex items-center justify-between gap-2">
          <UserLink user={u} />
          <span className="shrink-0 text-caption-1-regular text-text-tertiary">{userKind(u)}</span>
        </li>
      ))}
    </ul>
  );
}

function Side({
  workspace,
  draft,
  names,
  onClose,
  className,
}: {
  workspace: string;
  draft: AudienceDraft;
  names: Record<string, string>;
  onClose?: () => void;
  className?: string;
}) {
  return (
    <Panel label="Who matches" ground className={className}>
      <PageHeader title="Who matches" level={2}>
        {onClose ? <IconButton size="small" icon={RiCloseLine} aria-label="Close" onClick={onClose} /> : null}
      </PageHeader>
      <ScrollArea ground className="px-4 pb-6">
        <div className="flex flex-col gap-6 pt-2">
          <Card title="Right now">
            <PreviewPane preview={draft.preview} loading={draft.previewing} error={draft.problems.rest[0] ?? null} />
          </Card>
          <Card title="Test a person">
            <TestPerson workspace={workspace} rules={draft.rules} names={names} />
          </Card>
          <Card title="Used by">
            <UsedBy draft={draft} />
          </Card>
        </div>
      </ScrollArea>
    </Panel>
  );
}

function DeleteAudience({ draft }: { draft: AudienceDraft }) {
  const [open, setOpen] = useState(false);

  return (
    <>
      <Button variant="secondary" size="small" onClick={() => setOpen(true)}>
        Delete
      </Button>
      <AlertDialog
        isOpen={open}
        onOpenChange={(next) => {
          setOpen(next);
          if (!next) {
            draft.remove.reset();
          }
        }}
        title={`Delete ${draft.server.name}?`}
        confirmLabel="Delete audience"
        onConfirm={() => draft.remove.mutateAsync()}
      >
        <p className="text-body-2-regular text-text-secondary">
          {draft.remove.error
            ? draft.remove.error.message
            : "Workflow drafts that name it will need another audience before they publish."}
        </p>
      </AlertDialog>
    </>
  );
}

export function AudienceEditor({ workspace, audience }: { workspace: string; audience: Audience }) {
  const navigate = useNavigate();
  const canManage = usePermission(workspace, "audiences:manage");
  const draft = useAudienceDraft(workspace, audience, () => void navigate({ to: "/d/customers/audiences" }));
  const fields = useQuery(audienceFieldsQuery(workspace)).data ?? [];
  const tags = useQuery(tagsQuery(workspace)).data ?? [];
  const saved = useQuery(audiencesQuery(workspace)).data ?? [];
  const lookups: Lookups = {
    tags: tags.map((t) => [t.id, t.name]),
    audiences: saved.filter((a) => a.id !== audience.id).map((a) => [a.id, a.name]),
  };
  const names = Object.fromEntries([...lookups.tags, ...lookups.audiences]);
  const beside = useWide(1280);
  const [drawer, setDrawer] = useState(false);

  return (
    <div className="flex h-full min-h-0 min-w-0 flex-1 gap-3">
      <Panel label="Audience" ground className="h-full flex-1">
        <header className="flex min-h-12 shrink-0 flex-wrap items-center justify-between gap-x-3 gap-y-1 ps-2.5 pe-2.5 pt-[7px]">
          <div className="flex min-w-40 flex-1 items-center gap-2">
            <Link
              to="/d/customers/audiences"
              aria-label="Back to audiences"
              className="flex size-8 shrink-0 items-center justify-center rounded-lg text-foreground-icon-secondary outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
            >
              <RiArrowLeftSLine aria-hidden className="size-5 rtl:-scale-x-100" />
            </Link>
            <h1 className="min-w-0 truncate text-body-medium text-text-primary">{draft.server.name}</h1>
            <span className="hidden truncate text-body-2-regular text-text-tertiary sm:inline">
              {draft.changed ? "Unsaved changes" : "Saved"}
            </span>
          </div>
          <div className="ms-auto flex shrink-0 items-center gap-2">
            {beside ? null : (
              <IconButton
                size="small"
                icon={RiUserSearchLine}
                aria-label="Who matches"
                onClick={() => setDrawer(true)}
              />
            )}
            {canManage ? <DeleteAudience draft={draft} /> : null}
            {canManage ? (
              <Button
                size="small"
                loading={draft.save.isPending}
                disabled={!draft.changed || draft.form === null}
                onClick={() => draft.save.mutate()}
              >
                Save
              </Button>
            ) : null}
          </div>
        </header>
        <ScrollArea ground>
          <fieldset disabled={!canManage} className="min-w-0">
            <div className="mx-auto flex w-full max-w-220 flex-col gap-8 px-4 pt-6 pb-16 sm:px-6">
              <Card title="Audience">
                <Input label="Name" value={draft.name} onChange={draft.setName} />
                <Textarea
                  label="What it is for"
                  placeholder="Paying customers on the Pro plan"
                  value={draft.description}
                  onChange={draft.setDescription}
                />
              </Card>
              <Card title="Rule">
                <p className="text-body-2-regular text-text-secondary">{summary(draft.rules, names)}</p>
                {draft.form ? (
                  <RuleBuilder
                    form={draft.form}
                    fields={fields}
                    lookups={lookups}
                    problems={draft.problems.byPath}
                    quiet={!draft.save.error}
                    onChange={draft.setForm}
                  />
                ) : (
                  <div className="flex items-center gap-2 text-body-2-regular text-text-secondary">
                    <RiGroupLine aria-hidden className="size-4 text-foreground-icon-tertiary" />
                    This rule was written through the API and nests deeper than the builder shows.
                  </div>
                )}
                {draft.save.error && draft.problems.rest.length > 0 ? (
                  <p className="text-body-2-regular text-text-error-primary">{draft.problems.rest.join(" ")}</p>
                ) : null}
              </Card>
            </div>
          </fieldset>
        </ScrollArea>
      </Panel>
      {beside ? (
        <Side workspace={workspace} draft={draft} names={names} className="h-full w-[360px] shrink-0" />
      ) : (
        <Drawer
          isOpen={drawer}
          onOpenChange={setDrawer}
          placement="end"
          size="md"
          aria-label="Who matches"
          className="p-0"
        >
          <Side workspace={workspace} draft={draft} names={names} onClose={() => setDrawer(false)} className="h-full" />
        </Drawer>
      )}
    </div>
  );
}
