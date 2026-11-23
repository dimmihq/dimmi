/*
 * The routine builder's sections, the same for a new routine and a saved
 * one, laid out the way a procedure and an agent are: its name and what
 * it is for, the instructions Operator follows, written in the shared
 * prompt writer where @ names a tool, when it runs, what it may read and
 * change, the GitHub repositories it reads, its model and budget, and
 * who hears about each run. Every choice is picked from a dropdown of
 * what the workspace has. Nothing saves here; the page decides when.
 */
import {
  RiBookOpenLine,
  RiChat3Line,
  RiEditLine,
  RiFlashlightLine,
  RiGithubFill,
  RiLineChartLine,
  RiMailLine,
  RiNotification3Line,
  RiSlackFill,
} from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import type { ComponentType } from "react";

import { CheckboxCard } from "@/components/base/checkbox/checkbox-card";
import { ComboBox, ComboBoxItem } from "@/components/base/combobox/combobox";
import { Input } from "@/components/base/input/input";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { Select, SelectItem } from "@/components/base/select/select";
import { Textarea } from "@/components/base/textarea/textarea";
import { ModelSelect } from "@/features/ai/agent/model-section";
import { modelsFor, usableKeys } from "@/features/ai/format";
import { MultiPick } from "@/features/automation/pick";
import { Action } from "@/features/settings/controls";
import { Row, Section } from "@/features/settings/layout";
import { agentQuery } from "@/lib/api/ai";
import { inboxesQuery } from "@/lib/api/inboxes";
import { integrationsQuery } from "@/lib/api/integrations";
import { settingOptions } from "@/lib/api/knowledge-sources";
import { destinationsQuery } from "@/lib/api/notification-destinations";
import type { NotifyPolicy, RoutineCadence, RoutineToolGroup } from "@/lib/api/routines";
import { membersQuery } from "@/lib/api/workspace";

import type { NotifyChannel, RoutineDraft } from "./draft";
import { RoutinePrompt } from "./routine-prompt";
import { ordinal, summarize, type Timing, WEEKDAYS } from "./timing";

type Change = (next: RoutineDraft) => void;

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

const policies: { id: NotifyPolicy; title: string }[] = [
  { id: "on_findings", title: "When it suggests something" },
  { id: "always", title: "After every run" },
  { id: "on_error_only", title: "Only when it fails" },
];

const channelChoices: [NotifyChannel, string][] = [
  ["in_app", "In dimmi"],
  ["email", "Email"],
  ["slack", "Slack"],
];

const channelIcons: Record<string, ComponentType<{ className?: string }>> = {
  in_app: RiNotification3Line,
  email: RiMailLine,
  slack: RiSlackFill,
};

/* Room to spell out several picked names; the control still hugs what it says. */
const widePick = "max-w-96 shrink-0";

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
      <RoutinePrompt workspace={workspace} draft={draft} onChange={onChange} />
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
    <>
      <Section title="Name" description="How the team finds it in lists, threads and notifications.">
        <div className="py-2.5 pe-2.5">
          <Input
            aria-label="Name"
            size="small"
            className="w-full"
            value={draft.name}
            placeholder="Weekly knowledge gaps"
            onChange={(name) => onChange({ ...draft, name })}
          />
        </div>
      </Section>
      <Section title="What it is for" description="A line the team reads in the list of routines.">
        <div className="py-2.5 pe-2.5">
          <Textarea
            aria-label="What it is for"
            value={draft.description}
            placeholder="Finds what the AI agent could not answer and drafts the articles that would have."
            rows={2}
            autoResize
            maxRows={6}
            onChange={(description) => onChange({ ...draft, description })}
          />
        </div>
      </Section>
    </>
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
      <Section title="Access" description="The routine runs as you, with only these. You can only give what you hold.">
        <Row label="Permissions">
          <MultiPick
            label="Permissions"
            choices={permissions.map((p) => [p.id, p.label] as const)}
            value={draft.permissions}
            shown={2}
            allLabel="None"
            className={widePick}
            onChange={(picked) => onChange({ ...draft, permissions: picked })}
          />
        </Row>
        <Row
          label="Inboxes"
          description={
            draft.inboxIds.length === 0
              ? "Only teammates who see every conversation can open its runs."
              : "It reads conversations in these inboxes only."
          }
        >
          <MultiPick
            label="Inboxes"
            choices={(inboxes ?? []).map((i) => [i.id, i.name] as const)}
            value={draft.inboxIds}
            allLabel="Every inbox"
            empty="No inboxes yet"
            className={widePick}
            onChange={(inboxIds) => onChange({ ...draft, inboxIds })}
          />
        </Row>
      </Section>
    </>
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
  const navigate = useNavigate();

  return (
    <Section
      title="GitHub"
      description="The only repositories this routine may read. Pull request text is never followed as instructions."
    >
      {accounts.length === 0 ? (
        <Row label="Account">
          <Action onPress={() => void navigate({ to: "/d/settings/$section", params: { section: "integrations" } })}>
            Connect GitHub
          </Action>
        </Row>
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
        <Row label="Repositories">
          <MultiPick
            label="Repositories"
            choices={(repos ?? []).map((r) => [r.value, r.label] as const)}
            value={gh.repositories}
            allLabel="Pick repositories"
            empty={isFetching ? "Reading the account's repositories…" : "The account has no repositories."}
            className={widePick}
            onChange={(repositories) => set({ repositories })}
          />
        </Row>
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
  const people = (members ?? []).filter((m) => m.status === "active").map((m) => [m.id, m.user.name] as const);
  const n = draft.notify;
  const set = (change: Partial<RoutineDraft["notify"]>) => onChange({ ...draft, notify: { ...n, ...change } });
  const told = n.channels.includes("in_app") || n.channels.includes("email");

  return (
    <Section title="Who hears about it" description="Each run's report, and every suggestion with Apply and Decline.">
      <Row label="When">
        <Select
          aria-label="When to tell the team"
          size="sm"
          className="w-60"
          selectedKey={draft.notifyPolicy}
          onSelectionChange={(key) => onChange({ ...draft, notifyPolicy: key as NotifyPolicy })}
        >
          {policies.map((p) => (
            <SelectItem key={p.id} id={p.id} textValue={p.title}>
              {p.title}
            </SelectItem>
          ))}
        </Select>
      </Row>
      <Row label="Channels">
        <MultiPick
          label="Channels"
          choices={channelChoices}
          value={n.channels}
          shown={3}
          icons={channelIcons}
          allLabel="Nowhere"
          className={widePick}
          onChange={(channels) => set({ channels: channels as NotifyChannel[] })}
        />
      </Row>
      {told ? (
        <Row label="People" description="Told in dimmi and by email, as the channels say.">
          <MultiPick
            label="People"
            choices={people}
            value={n.people}
            allLabel="Nobody yet"
            empty="No teammates yet"
            className={widePick}
            onChange={(picked) => set({ people: picked })}
          />
        </Row>
      ) : null}
      {n.channels.includes("slack") ? (
        <SlackRow workspace={workspace} picked={n.slack} onPick={(picked) => set({ slack: picked })} />
      ) : null}
    </Section>
  );
}

/*
 * SlackRow picks the Slack channels a run posts to, or links to where a
 * channel is added when there is none yet.
 */
function SlackRow({
  workspace,
  picked,
  onPick,
}: {
  workspace: string;
  picked: string[];
  onPick: (picked: string[]) => void;
}) {
  const { data: destinations } = useQuery(destinationsQuery(workspace));
  const { data: integrations } = useQuery(integrationsQuery(workspace));
  const slack = (destinations ?? [])
    .filter((d) => d.type === "slack_channel" && d.status === "active")
    .map((d) => [d.id, `#${d.name.replace(/^#/, "")}`] as const);
  const app = (integrations ?? []).find((i) => i.provider === "slack" && i.status === "active");
  const navigate = useNavigate();

  return (
    <Row label="Slack channels">
      {slack.length > 0 ? (
        <MultiPick
          label="Slack channels"
          choices={slack}
          value={picked}
          allLabel="Pick channels"
          className={widePick}
          onChange={onPick}
        />
      ) : (
        <Action
          onPress={() =>
            void (app
              ? navigate({ to: "/d/settings/integrations/$integration", params: { integration: app.id } })
              : navigate({ to: "/d/settings/$section", params: { section: "integrations" } }))
          }
        >
          {app ? "Add a Slack channel" : "Connect Slack"}
        </Action>
      )}
    </Row>
  );
}
