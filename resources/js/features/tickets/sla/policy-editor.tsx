/*
 * One SLA policy's editor, or a new policy's. Its settings are a draft
 * saved in one go from a bar that floats over the page while anything
 * changed; leaving with unsaved changes asks first. A new policy is
 * created from the same bar and the page moves to it. Archiving sits at
 * the end and asks first; an archived policy reads but no longer
 * changes.
 */
import { RiArrowLeftLine, RiTimerLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useBlocker, useNavigate } from "@tanstack/react-router";
import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/base/buttons/button";
import { toast } from "@/components/base/toast/toast";
import { Danger } from "@/features/settings/controls";
import { usePermission } from "@/features/settings/inboxes";
import { PageSkeleton, Row, Section, SettingsPage } from "@/features/settings/layout";
import { useAccount } from "@/features/settings/use-account";
import { EmptyState, Panel } from "@/features/shell/page";
import { schedulesQuery } from "@/lib/api/schedules";
import { createSLAPolicy, type SLAPolicy, slaPoliciesQuery, updateSLAPolicy } from "@/lib/api/sla";
import { teamsQuery } from "@/lib/api/teams";
import { ticketTypesQuery } from "@/lib/api/tickets";

import { ArchivePolicyDialog } from "./archive-dialog";
import { blankDraft, draftOf, draftProblem, type PolicyDraft, policyBody, policyChanges } from "./form";
import { AppliesSection, BreachSection, ClockSection, DetailsSection, TargetsSection } from "./policy-sections";

function BackToPolicies() {
  const { t } = useTranslation("sla");
  const navigate = useNavigate();

  return (
    <div>
      <Button
        variant="secondary"
        size="small"
        leadingIcon={RiArrowLeftLine}
        onClick={() => navigate({ to: "/d/settings/$section", params: { section: "sla" } })}
      >
        {t("editor.back")}
      </Button>
    </div>
  );
}

/*
 * usePolicyDraft keeps the unsaved settings of the policy, or of a new
 * one when there is none, and saves them.
 */
function usePolicyDraft(workspace: string, policy: SLAPolicy | null, start: PolicyDraft) {
  const { t } = useTranslation("sla");
  const client = useQueryClient();
  const navigate = useNavigate();
  const [saved, setSaved] = useState<PolicyDraft | null>(() => (policy ? draftOf(policy) : null));
  const [draft, setDraft] = useState(() => (policy ? draftOf(policy) : start));
  const leaving = useRef(false);
  const changes = policyChanges(saved ?? start, draft);
  const dirty = Object.keys(changes).length > 0;
  const problem = draftProblem(draft);
  const save = useMutation({
    mutationFn: () =>
      policy ? updateSLAPolicy(workspace, policy.id, changes) : createSLAPolicy(workspace, policyBody(draft)),
    onSuccess: async (next) => {
      const fresh = draftOf(next);
      setSaved(fresh);
      setDraft(fresh);
      await client.invalidateQueries({ queryKey: slaPoliciesQuery(workspace).queryKey });
      toast.success(policy ? t("editor.saved") : t("editor.created"));
      if (!policy) {
        leaving.current = true;
        await navigate({ to: "/d/settings/sla/$policy", params: { policy: next.id }, replace: true });
      }
    },
  });

  return {
    draft,
    dirty,
    problem,
    save,
    leaving,
    creating: policy === null,
    set: (change: Partial<PolicyDraft>) => setDraft((was) => ({ ...was, ...change })),
    discard: () => setDraft(saved ?? start),
  };
}

type Edit = ReturnType<typeof usePolicyDraft>;

function SaveBar({ edit, locked }: { edit: Edit; locked: boolean }) {
  const { t } = useTranslation("sla");
  if (locked || !(edit.creating || edit.dirty || edit.save.error)) {
    return null;
  }
  const message = edit.save.error?.message ?? (edit.problem ? t(`editor.${edit.problem}`) : t("editor.unsaved"));

  return (
    <div className="flex items-center gap-3 rounded-2xl border border-border-button-default bg-background-primary-default py-2 ps-4 pe-2 shadow-dropdown">
      <span
        role={edit.save.error ? "alert" : "status"}
        className={
          edit.save.error
            ? "min-w-0 flex-1 truncate text-body-2-regular text-text-error-primary"
            : "min-w-0 flex-1 truncate text-body-2-regular text-text-secondary"
        }
      >
        {message}
      </span>
      {edit.creating ? null : (
        <Button size="small" variant="secondary" disabled={edit.save.isPending} onClick={edit.discard}>
          {t("editor.discard")}
        </Button>
      )}
      <Button
        size="small"
        loading={edit.save.isPending}
        disabled={!(edit.creating || edit.dirty) || edit.problem !== null}
        onClick={() => edit.save.mutate()}
      >
        {edit.creating ? t("editor.create") : t("editor.save")}
      </Button>
    </div>
  );
}

function ArchiveSection({ workspace, policy }: { workspace: string; policy: SLAPolicy }) {
  const { t } = useTranslation("sla");
  const [open, setOpen] = useState(false);

  return (
    <Section title={t("archive.title")}>
      <Row label={t("archive.button")} description={t("archive.description")}>
        <Danger onPress={() => setOpen(true)}>{t("archive.button")}</Danger>
      </Row>
      {open ? <ArchivePolicyDialog workspace={workspace} policy={policy} onClose={() => setOpen(false)} /> : null}
    </Section>
  );
}

function Editor({ workspace, policy }: { workspace: string; policy: SLAPolicy | null }) {
  const { t } = useTranslation("sla");
  const { data: schedules = [] } = useQuery({ ...schedulesQuery(workspace), enabled: workspace !== "" });
  const { data: types = [] } = useQuery(ticketTypesQuery(workspace));
  const { data: teams = [] } = useQuery({ ...teamsQuery(workspace), enabled: workspace !== "" });
  const fallback = schedules.find((s) => s.is_default)?.id ?? null;
  const edit = usePolicyDraft(workspace, policy, blankDraft(fallback));
  const archived = Boolean(policy?.archived_at);
  const locked = !usePermission(workspace, "settings:write") || archived;
  useBlocker({
    shouldBlockFn: () => edit.dirty && !edit.leaving.current && !window.confirm(t("editor.leave")),
    enableBeforeUnload: () => edit.dirty && !edit.leaving.current,
  });
  const props = { draft: edit.draft, set: edit.set, locked };
  let description: string | undefined;
  if (archived) {
    description = t("editor.archivedDescription");
  } else if (!policy) {
    description = t("editor.newDescription");
  }

  return (
    <SettingsPage
      title={policy ? policy.name : t("editor.newTitle")}
      description={description}
      footer={<SaveBar edit={edit} locked={locked} />}
    >
      <BackToPolicies />
      <DetailsSection {...props} />
      <TargetsSection {...props} />
      <ClockSection {...props} schedules={schedules} />
      <AppliesSection {...props} types={types} />
      <BreachSection {...props} teams={teams} />
      {policy && !locked ? <ArchiveSection workspace={workspace} policy={policy} /> : null}
    </SettingsPage>
  );
}

export function SLAPolicyPage({ policy: id }: { policy: string }) {
  const { t } = useTranslation("sla");
  const workspace = useAccount().workspace.id;
  const { data: policies, isPending } = useQuery(slaPoliciesQuery(workspace));
  const schedules = useQuery({ ...schedulesQuery(workspace), enabled: workspace !== "" });

  if (id === "new") {
    return schedules.isPending ? <PageSkeleton /> : <Editor workspace={workspace} policy={null} />;
  }
  if (isPending) {
    return <PageSkeleton />;
  }
  const policy = policies?.find((p) => p.id === id);
  if (!policy) {
    return (
      <Panel label={t("page.title")} className="h-full flex-1">
        <EmptyState icon={RiTimerLine} title={t("editor.missing")} description={t("editor.missingHint")} />
      </Panel>
    );
  }

  return <Editor key={policy.id} workspace={workspace} policy={policy} />;
}
