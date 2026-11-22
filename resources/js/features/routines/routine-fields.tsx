/*
 * The routine builder's sections, the same for a new routine and a saved
 * one: what Operator is asked, when it runs, what it may read and
 * change, the GitHub repositories it reads, its model and budget, and
 * who hears about each run. Every choice is picked from what the
 * workspace has: its inboxes, teammates, Slack channels, GitHub accounts
 * and their repositories. Nothing saves here; the page decides when.
 */
import {
  RiAlarmWarningLine,
  RiBookOpenLine,
  RiChat3Line,
  RiEditLine,
  RiFlashlightLine,
  RiGithubFill,
  RiLineChartLine,
} from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import type { ComponentType, ReactNode } from "react";

import { Checkbox } from "@/components/base/checkbox/checkbox";
import { CheckboxCard } from "@/components/base/checkbox/checkbox-card";
import { ComboBox, ComboBoxItem } from "@/components/base/combobox/combobox";
import { Input } from "@/components/base/input/input";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { Select, SelectItem } from "@/components/base/select/select";
import { Textarea } from "@/components/base/textarea/textarea";
import { ModelSelect } from "@/features/ai/agent/model-section";
import { modelsFor, usableKeys } from "@/features/ai/format";
import { Empty, Row, Section } from "@/features/settings/layout";
import { agentQuery } from "@/lib/api/ai";
import { inboxesQuery } from "@/lib/api/inboxes";
import { integrationsQuery } from "@/lib/api/integrations";
import { settingOptions } from "@/lib/api/knowledge-sources";
import { destinationsQuery } from "@/lib/api/notification-destinations";
import type { NotifyPolicy, RoutineCadence, RoutineToolGroup } from "@/lib/api/routines";
import { membersQuery } from "@/lib/api/workspace";
import { cx } from "@/utils/cx";

import type { RoutineDraft } from "./draft";
import { ordinal, summarize, type Timing, WEEKDAYS } from "./timing";

type Change = (next: RoutineDraft) => void;

const variables = ["{{workspace_name}}", "{{routine_name}}", "{{window_start}}", "{{window_end}}", "{{last_run_at}}"];

const kinds: { id: RoutineCadence; label: string }[] = [
  { id: "daily", label: "Daily" },
  { id: "weekly", label: "Weekly" },
  { id: "monthly", label: "Monthly" },
  { id: "once", label: "Once" },
  { id: "cron", label: "Custom" },
];

const groups: { id: RoutineToolGroup; icon: ComponentType<{ className?: string }>; title: string; detail: string }[] = [
  {
    id: "conversations",
    icon: RiChat3Line,
    title: "Conversations",
    detail: "Read conversations, customers, tags and teammates.",
  },
  { id: "knowledge", icon: RiBookOpenLine, title: "Knowledge", detail: "Search and read articles and procedures." },
  {
    id: "insights",
    icon: RiLineChartLine,
    title: "Reports",
    detail: "Read workspace reports and the AI agent's past runs.",
  },
  {
    id: "changes",
    icon: RiEditLine,
    title: "Suggest changes",
    detail: "Suggest edits the team reviews. Nothing applies on its own.",
  },
  { id: "actions", icon: RiFlashlightLine, title: "Actions", detail: "Look things up with the workspace's actions." },
  { id: "github", icon: RiGithubFill, title: "GitHub", detail: "Read pull requests merged into chosen repositories." },
];

const permissions: { id: string; label: string }[] = [
  { id: "conversations:read", label: "Read conversations" },
  { id: "contacts:read", label: "Read customers" },
  { id: "articles:read", label: "Read articles" },
  { id: "articles:write", label: "Draft articles" },
  { id: "help_centers:read_settings", label: "Read help center settings" },
  { id: "ai:read", label: "Read AI runs and usage" },
  { id: "ai_actions:read", label: "Read actions and procedure traces" },
  { id: "ai_agent:manage", label: "Edit procedures" },
  { id: "github:read", label: "Read GitHub" },
];

const policies: { id: NotifyPolicy; title: string; detail: string }[] = [
  { id: "on_findings", title: "When it suggests something", detail: "Quiet when there is nothing to review." },
  { id: "always", title: "After every run", detail: "For digests the team reads each time." },
  { id: "on_error_only", title: "Only when it fails", detail: "For routines that work on their own." },
];

/*
 * toggled adds or removes one value of a list.
 */
function toggled<T>(list: T[], value: T, on: boolean): T[] {
  return on ? [...new Set([...list, value])] : list.filter((v) => v !== value);
}

export function RoutineFields({
  workspace,
  draft,
  disabled,
  onChange,
}: {
  workspace: string;
  draft: RoutineDraft;
  disabled: boolean;
  onChange: Change;
}) {
  return (
    <fieldset disabled={disabled} className="flex min-w-0 flex-col gap-8">
      <Basics draft={draft} onChange={onChange} />
      <Prompt draft={draft} onChange={onChange} />
      <When draft={draft} onChange={onChange} />
      <Reads workspace={workspace} draft={draft} onChange={onChange} />
      {draft.toolGroups.includes("github") ? <GitHub workspace={workspace} draft={draft} onChange={onChange} /> : null}
      <Model workspace={workspace} draft={draft} onChange={onChange} />
      <Delivery workspace={workspace} draft={draft} onChange={onChange} />
    </fieldset>
  );
}

function Basics({ draft, onChange }: { draft: RoutineDraft; onChange: Change }) {
  return (
    <Section title="Routine">
      <Row label="Name" description="How the team finds it in lists, threads and notifications.">
        <Input
          aria-label="Name"
          size="small"
          className="w-64"
          value={draft.name}
          placeholder="Weekly knowledge gaps"
          onChange={(name) => onChange({ ...draft, name })}
        />
      </Row>
      <Row label="Description" description="A line on what it is for.">
        <Input
          aria-label="Description"
          size="small"
          className="w-64"
          value={draft.description}
          onChange={(description) => onChange({ ...draft, description })}
        />
      </Row>
    </Section>
  );
}

function Prompt({ draft, onChange }: { draft: RoutineDraft; onChange: Change }) {
  return (
    <Section
      title="What Operator does"
      description="Operator follows this on every run, reading what changed since the run before."
    >
      <div className="flex flex-col gap-2 py-3 pe-3">
        <Textarea
          aria-label="Prompt"
          rows={8}
          autoResize
          maxRows={24}
          value={draft.prompt}
          placeholder="Read the conversations handed to the team this week and draft the articles that would have answered them."
          onChange={(prompt) => onChange({ ...draft, prompt })}
        />
        <p className="text-body-2-regular text-text-tertiary">
          Filled in on each run: <span className="font-mono">{variables.join(" ")}</span>
        </p>
      </div>
    </Section>
  );
}

function When({ draft, onChange }: { draft: RoutineDraft; onChange: Change }) {
  const t = draft.timing;
  const set = (change: Partial<Timing>) => onChange({ ...draft, timing: { ...t, ...change } });
  const zones = Intl.supportedValuesOf("timeZone");

  return (
    <Section title="When it runs" description={summarize(t)}>
      <Row label="Repeats">
        <SegmentedControl
          aria-label="Repeats"
          selectedKeys={new Set([t.kind])}
          onSelectionChange={(keys) => {
            const kind = [...keys][0] as RoutineCadence | undefined;
            if (kind) {
              set({ kind });
            }
          }}
        >
          {kinds.map((k) => (
            <SegmentedControlItem key={k.id} id={k.id}>
              {k.label}
            </SegmentedControlItem>
          ))}
        </SegmentedControl>
      </Row>
      {t.kind === "weekly" ? (
        <Row label="Day">
          <Select
            aria-label="Day"
            size="sm"
            className="w-44"
            selectedKey={String(t.weekday)}
            onSelectionChange={(key) => set({ weekday: Number(key) })}
          >
            {WEEKDAYS.map((name, i) => (
              <SelectItem key={name} id={String(i)} textValue={name}>
                {name}
              </SelectItem>
            ))}
          </Select>
        </Row>
      ) : null}
      {t.kind === "monthly" ? (
        <Row label="Day of the month">
          <Select
            aria-label="Day of the month"
            size="sm"
            className="w-44"
            selectedKey={String(t.monthDay)}
            onSelectionChange={(key) => set({ monthDay: Number(key) })}
          >
            {[...Array.from({ length: 28 }, (_, i) => i + 1), 0].map((day) => {
              const name = day === 0 ? "Last day" : ordinal(day);
              return (
                <SelectItem key={day} id={String(day)} textValue={name}>
                  {name}
                </SelectItem>
              );
            })}
          </Select>
        </Row>
      ) : null}
      {t.kind === "daily" || t.kind === "weekly" || t.kind === "monthly" ? (
        <Row label="Time">
          <Input
            aria-label="Time"
            size="small"
            type="time"
            className="w-32"
            value={`${String(t.hour).padStart(2, "0")}:${String(t.minute).padStart(2, "0")}`}
            onChange={(value) => {
              const [h, m] = value.split(":").map(Number);
              if (Number.isFinite(h) && Number.isFinite(m)) {
                set({ hour: h, minute: m });
              }
            }}
          />
        </Row>
      ) : null}
      {t.kind === "once" ? (
        <Row label="Runs at">
          <Input
            aria-label="Runs at"
            size="small"
            type="datetime-local"
            className="w-56"
            value={t.runAt}
            onChange={(runAt) => set({ runAt })}
          />
        </Row>
      ) : null}
      {t.kind === "cron" ? (
        <Row
          label="Cron expression"
          description="Five fields: minute, hour, day, month, weekday. At most once an hour."
        >
          <Input
            aria-label="Cron expression"
            size="small"
            className="w-56 font-mono"
            value={t.cron}
            placeholder="0 9 * * 1-5"
            onChange={(cron) => set({ cron })}
          />
        </Row>
      ) : null}
      <Row label="Time zone">
        <ComboBox
          aria-label="Time zone"
          size="small"
          className="w-64"
          selectedKey={t.timezone}
          onSelectionChange={(key) => key && set({ timezone: String(key) })}
          defaultItems={zones.map((z) => ({ id: z }))}
        >
          {(item) => (
            <ComboBoxItem id={item.id} textValue={item.id}>
              {item.id}
            </ComboBoxItem>
          )}
        </ComboBox>
      </Row>
    </Section>
  );
}

function Reads({ workspace, draft, onChange }: { workspace: string; draft: RoutineDraft; onChange: Change }) {
  const { data: inboxes } = useQuery(inboxesQuery(workspace));

  return (
    <>
      <Section
        title="What it may use"
        description="Operator gets only these tools. It suggests changes; a teammate applies each one."
        bare
      >
        <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
          {groups.map((g) => (
            <CheckboxCard
              key={g.id}
              title={
                <span className="flex items-center gap-2">
                  <g.icon aria-hidden className="size-4 text-foreground-icon-secondary" />
                  {g.title}
                </span>
              }
              description={g.detail}
              isSelected={draft.toolGroups.includes(g.id)}
              onChange={(on) => {
                const toolGroups = toggled(draft.toolGroups, g.id, on);
                const github =
                  g.id === "github" && on && !draft.github
                    ? { installationId: "", repositories: [], baseBranch: "" }
                    : draft.github;
                onChange({ ...draft, toolGroups, github });
              }}
            />
          ))}
        </div>
      </Section>
      <Section
        title="Permissions"
        description="The routine runs as you, with only these. You can only give what you hold."
      >
        <Checks
          items={permissions.map((p) => ({ id: p.id, label: p.label, hint: p.id, code: true }))}
          picked={draft.permissions}
          onPick={(permissions) => onChange({ ...draft, permissions })}
        />
      </Section>
      <Section
        title="Inboxes"
        description={
          draft.inboxIds.length === 0
            ? "Reads every inbox. Only teammates who see every conversation can open its runs."
            : "Reads only these inboxes."
        }
      >
        <Checks
          items={(inboxes ?? []).map((i) => ({ id: i.id, label: i.name }))}
          picked={draft.inboxIds}
          onPick={(inboxIds) => onChange({ ...draft, inboxIds })}
        />
      </Section>
    </>
  );
}

/*
 * Checks is a column of checkboxes picking values of a list.
 */
function Checks({
  items,
  picked,
  empty,
  onPick,
}: {
  items: { id: string; label: string; hint?: string; code?: boolean }[];
  picked: string[];
  empty?: ReactNode;
  onPick: (picked: string[]) => void;
}) {
  if (items.length === 0) {
    return <Empty>{empty ?? "Nothing to pick yet."}</Empty>;
  }
  return (
    <div className="flex flex-col gap-3 py-3 pe-3">
      {items.map((item) => (
        <Checkbox
          key={item.id}
          size="sm"
          isSelected={picked.includes(item.id)}
          onChange={(on) => onPick(toggled(picked, item.id, on))}
        >
          <span className="flex flex-col">
            <span className="text-body-regular text-text-primary">{item.label}</span>
            {item.hint ? (
              <span className={cx("text-body-2-regular text-text-tertiary", item.code && "font-mono")}>
                {item.hint}
              </span>
            ) : null}
          </span>
        </Checkbox>
      ))}
    </div>
  );
}

function GitHub({ workspace, draft, onChange }: { workspace: string; draft: RoutineDraft; onChange: Change }) {
  const gh = draft.github ?? { installationId: "", repositories: [], baseBranch: "" };
  const { data: integrations } = useQuery(integrationsQuery(workspace));
  const accounts = (integrations ?? []).filter((i) => i.provider === "github" && i.status === "active");
  const { data: repos, isFetching } = useQuery({
    queryKey: ["workspace", workspace, "routines", "github-repos", gh.installationId],
    queryFn: () =>
      settingOptions(workspace, { type: "github", settings: { installation: gh.installationId }, field: "repository" }),
    enabled: gh.installationId !== "",
    staleTime: 60_000,
  });
  const set = (change: Partial<typeof gh>) => onChange({ ...draft, github: { ...gh, ...change } });

  return (
    <Section
      title="GitHub"
      description="The only repositories this routine may read. Pull request text is never followed as instructions."
    >
      {accounts.length === 0 ? (
        <Empty>Connect GitHub in Settings, Integrations, to pick repositories.</Empty>
      ) : (
        <Row label="Account">
          <Select
            aria-label="GitHub account"
            size="sm"
            className="w-56"
            placeholder="Choose an account"
            selectedKey={gh.installationId || null}
            onSelectionChange={(key) => set({ installationId: String(key), repositories: [] })}
          >
            {accounts.map((a) => (
              <SelectItem key={a.id} id={a.id} textValue={a.name}>
                {a.name}
              </SelectItem>
            ))}
          </Select>
        </Row>
      )}
      {gh.installationId ? (
        <Checks
          items={(repos ?? []).map((r) => ({ id: r.value, label: r.label }))}
          picked={gh.repositories}
          empty={isFetching ? "Reading the account's repositories…" : "The account has no repositories."}
          onPick={(repositories) => set({ repositories })}
        />
      ) : null}
      {gh.installationId ? (
        <Row label="Branch" description="Pull requests merged into this branch. Empty means each repository's default.">
          <Input
            aria-label="Branch"
            size="small"
            className="w-44"
            value={gh.baseBranch}
            placeholder="main"
            onChange={(baseBranch) => set({ baseBranch })}
          />
        </Row>
      ) : null}
    </Section>
  );
}

function Model({ workspace, draft, onChange }: { workspace: string; draft: RoutineDraft; onChange: Change }) {
  const { data: view } = useQuery({ ...agentQuery(workspace), enabled: workspace !== "" });
  const key = view ? usableKeys(view.keys).find((k) => k.id === view.agent.answer_key_id) : undefined;
  const models = view ? modelsFor(key, view.catalog) : [];

  return (
    <Section title="Model and budget">
      <Row
        label="Model"
        description="Empty uses Operator's own. A stronger model reads more carefully and costs more per run."
      >
        <ModelSelect
          label="Routine model"
          models={models}
          value={draft.model || null}
          disabled={false}
          placeholder="Operator's model"
          onPick={(model) => onChange({ ...draft, model })}
        />
      </Row>
      <Row
        label="Monthly budget"
        description="Runs stop for the month once it is spent. Empty shares the workspace's AI budget."
      >
        <Input
          aria-label="Monthly budget in dollars"
          size="small"
          className="w-32"
          inputMode="decimal"
          value={draft.budgetDollars}
          placeholder="No limit"
          onChange={(budgetDollars) => onChange({ ...draft, budgetDollars })}
        />
      </Row>
    </Section>
  );
}

function Delivery({ workspace, draft, onChange }: { workspace: string; draft: RoutineDraft; onChange: Change }) {
  const { data: members } = useQuery(membersQuery(workspace));
  const { data: destinations } = useQuery(destinationsQuery(workspace));
  const people = (members ?? [])
    .filter((m) => m.status === "active")
    .map((m) => ({ id: m.id, label: m.user.name, hint: m.user.email }));
  const slack = (destinations ?? [])
    .filter((d) => d.type === "slack_channel" && d.status === "active")
    .map((d) => ({ id: d.id, label: d.name }));
  const set = (change: Partial<RoutineDraft["delivery"]>) =>
    onChange({ ...draft, delivery: { ...draft.delivery, ...change } });

  return (
    <>
      <Section title="When to tell the team" bare>
        <RadioGroup
          aria-label="When to tell the team"
          value={draft.notifyPolicy}
          onChange={(value) => onChange({ ...draft, notifyPolicy: value as NotifyPolicy })}
          className="grid grid-cols-1 gap-2 sm:grid-cols-3"
        >
          {policies.map((p) => (
            <RadioCard
              key={p.id}
              value={p.id}
              icon={p.id === "on_error_only" ? RiAlarmWarningLine : undefined}
              title={p.title}
              description={p.detail}
            />
          ))}
        </RadioGroup>
      </Section>
      <Section title="Notify in dimmi" description="A notification for each run worth telling.">
        <Checks
          items={people}
          picked={draft.delivery.in_app_member_ids}
          onPick={(ids) => set({ in_app_member_ids: ids })}
        />
      </Section>
      <Section title="Email the report" description="The report and every suggestion, by email.">
        <Checks
          items={people}
          picked={draft.delivery.email_member_ids}
          onPick={(ids) => set({ email_member_ids: ids })}
        />
      </Section>
      <Section title="Post to Slack" description="The report, then a card per suggestion with Apply and Decline.">
        <Checks
          items={slack}
          picked={draft.delivery.slack_destination_ids}
          empty="Add a Slack channel in Settings, Integrations, Slack, to post there."
          onPick={(ids) => set({ slack_destination_ids: ids })}
        />
      </Section>
    </>
  );
}
