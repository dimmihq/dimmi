/*
 * The routine builder, the same for a new routine and a saved one, and
 * as short as a routine is: its name, what Operator should do in plain
 * words, when it runs, what it reads, and who hears about each run.
 * How the work is done is Operator's to know. Nothing saves here; the
 * page decides when.
 */
import { RiMailLine, RiNotification3Line, RiSlackFill } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import type { ComponentType } from "react";

import { ComboBox, ComboBoxItem } from "@/components/base/combobox/combobox";
import { Input } from "@/components/base/input/input";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { Select, SelectItem } from "@/components/base/select/select";
import { MultiPick } from "@/features/automation/pick";
import { Action } from "@/features/settings/controls";
import { Row, Section } from "@/features/settings/layout";
import { inboxesQuery } from "@/lib/api/inboxes";
import { integrationsQuery } from "@/lib/api/integrations";
import { settingOptions } from "@/lib/api/knowledge-sources";
import { destinationsQuery } from "@/lib/api/notification-destinations";
import type { NotifyPolicy, RoutineCadence } from "@/lib/api/routines";
import { membersQuery } from "@/lib/api/workspace";

import type { NotifyChannel, RoutineDraft } from "./draft";
import { RoutineWriter } from "./routine-writer";
import { ordinal, summarize, type Timing, WEEKDAYS } from "./timing";
import { useMentionables } from "./use-mentionables";

type Change = (next: RoutineDraft) => void;

const kinds: { id: RoutineCadence; label: string }[] = [
  { id: "daily", label: "Daily" },
  { id: "weekly", label: "Weekly" },
  { id: "monthly", label: "Monthly" },
  { id: "once", label: "Once" },
  { id: "cron", label: "Custom" },
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
      <Basics workspace={workspace} draft={draft} disabled={disabled} onChange={onChange} />
      <When draft={draft} onChange={onChange} />
      <Reads workspace={workspace} draft={draft} onChange={onChange} />
      <Delivery workspace={workspace} draft={draft} onChange={onChange} />
    </fieldset>
  );
}

function Basics({
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
  const known = useMentionables(workspace);

  return (
    <>
      <Section title="Name">
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
      <Section
        title="What should Operator do?"
        description="Say it the way you would ask a teammate. Type @ to name a team, a teammate, a tag or an article."
      >
        <div className="py-2.5 pe-2.5">
          <RoutineWriter
            label="What should Operator do?"
            markdown={draft.prompt}
            known={known}
            readOnly={disabled}
            placeholder="Find the questions customers asked this week that our help center does not answer, and draft articles for them."
            onChange={(prompt) => onChange({ ...draft, prompt })}
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
  const { data: integrations } = useQuery(integrationsQuery(workspace));
  const accounts = (integrations ?? []).filter((i) => i.provider === "github" && i.status === "active");

  return (
    <Section title="What it reads">
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
      {accounts.length > 0 ? (
        <Repositories workspace={workspace} accounts={accounts} draft={draft} onChange={onChange} />
      ) : null}
    </Section>
  );
}

/*
 * Repositories picks the GitHub repositories whose merged pull requests
 * the routine reads, once GitHub is connected. One account is picked for
 * you; with several, the account comes first.
 */
function Repositories({
  workspace,
  accounts,
  draft,
  onChange,
}: {
  workspace: string;
  accounts: { id: string; name: string }[];
  draft: RoutineDraft;
  onChange: Change;
}) {
  const gh = draft.github;
  const account = gh.installationId || (accounts.length === 1 ? (accounts[0]?.id ?? "") : "");
  const { data: repos, isFetching } = useQuery({
    queryKey: ["workspace", workspace, "routines", "github-repos", account],
    queryFn: () =>
      settingOptions(workspace, { type: "github", settings: { installation: account }, field: "repository" }),
    enabled: account !== "",
    staleTime: 60_000,
  });
  const set = (change: Partial<typeof gh>) =>
    onChange({ ...draft, github: { ...gh, installationId: account, ...change } });

  return (
    <>
      {accounts.length > 1 ? (
        <Row label="GitHub account">
          <Select
            aria-label="GitHub account"
            size="sm"
            className="w-56"
            placeholder="Choose an account"
            selectedKey={account || null}
            onSelectionChange={(key) => set({ installationId: String(key), repositories: [] })}
          >
            {accounts.map((a) => (
              <SelectItem key={a.id} id={a.id} textValue={a.name}>
                {a.name}
              </SelectItem>
            ))}
          </Select>
        </Row>
      ) : null}
      {account ? (
        <Row label="Repositories" description="It reads the pull requests merged into these.">
          <MultiPick
            label="Repositories"
            choices={(repos ?? []).map((r) => [r.value, r.label] as const)}
            value={gh.repositories}
            allLabel="None"
            empty={isFetching ? "Reading the account's repositories…" : "The account has no repositories."}
            className={widePick}
            onChange={(repositories) => set({ repositories })}
          />
        </Row>
      ) : null}
    </>
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
