/*
 * Testing a procedure before anyone else meets it. The simulator talks
 * to dimmi as a made-up customer, signed in or not, on any channel, with
 * any attributes, following the working copy. No action reaches the
 * team's systems: each answers with the mock written here, which starts
 * as its last live test's result. A real conversation can be replayed.
 * Beside the chat, the steps light up as they are done, decided, locked
 * or retried. A write that waits on the customer or a teammate waits
 * here too, until the tester confirms or approves it.
 *
 * Any chat can be saved as a test with what must hold after it; saved
 * tests run against the working copy in one click, and the editor shows
 * how many pass beside Publish.
 */
import { RiDeleteBinLine, RiPlayLine } from "@remixicon/react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Badge } from "@/components/base/badges/badge";
import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { Input } from "@/components/base/input/input";
import { Textarea } from "@/components/base/textarea/textarea";
import { toast } from "@/components/base/toast/toast";
import { Action } from "@/features/settings/controls";
import { Row, Section } from "@/features/settings/layout";
import {
  type ActionMock,
  type Assertion,
  type AssertionType,
  createTestCase,
  deleteTestCase,
  type ProcedureTestCase,
  runTestCases,
  type SimMessage,
  type SimPersona,
  type SimulateInput,
  type Simulation,
  simulateProcedure,
  testCasesQuery,
  updateTestCase,
} from "@/lib/api/procedures";
import { cx } from "@/utils/cx";

import type { Choices } from "../choices";
import { Pick } from "../pick";
import type { ChipSources } from "../procedure-writer/procedure-suggestions";
import { actionsOf, stepNumbers, walk } from "../procedures";
import type { useProcedureDraft } from "../use-procedure-draft";
import { assertionChoices, defaultAssertions, readJSON } from "./simulator";

type Draft = ReturnType<typeof useProcedureDraft>;

const channelChoices: Choices = [
  ["widget", "Messenger"],
  ["email", "Email"],
  ["whatsapp", "WhatsApp"],
  ["slack", "Slack"],
  ["api", "API"],
];

interface Turn {
  message: string;
  result: Simulation;
}

function Persona({ persona, onChange }: { persona: SimPersona; onChange: (p: SimPersona) => void }) {
  return (
    <Section title="Customer" description="Who dimmi talks to. Attributes count as trusted.">
      <div className="flex flex-wrap items-end gap-3 px-3 py-2.5">
        <Checkbox isSelected={persona.verified ?? false} onChange={(verified) => onChange({ ...persona, verified })}>
          Signed in
        </Checkbox>
        <Pick
          label="Channel"
          choices={channelChoices}
          value={persona.channel ?? "widget"}
          onChange={(channel) => onChange({ ...persona, channel })}
        />
        <div className="w-40">
          <Input
            aria-label="User id"
            size="small"
            placeholder="User id"
            defaultValue={persona.external_id}
            onChange={(external_id) => onChange({ ...persona, external_id })}
          />
        </div>
        <div className="w-48">
          <Input
            aria-label="Email"
            size="small"
            placeholder="Email"
            defaultValue={persona.email}
            onChange={(email) => onChange({ ...persona, email })}
          />
        </div>
      </div>
      <div className="px-3 pb-2.5">
        <Textarea
          aria-label="Attributes"
          placeholder='{"plan": "pro"}'
          rows={2}
          defaultValue={persona.attributes ? JSON.stringify(persona.attributes) : ""}
          onChange={(text) => {
            const attributes = readJSON(text);
            onChange({
              ...persona,
              attributes:
                typeof attributes === "object" && attributes ? (attributes as Record<string, unknown>) : undefined,
            });
          }}
        />
      </div>
    </Section>
  );
}

function Mocks({
  actions,
  sources,
  mocks,
  onChange,
}: {
  actions: string[];
  sources: ChipSources;
  mocks: Record<string, ActionMock>;
  onChange: (mocks: Record<string, ActionMock>) => void;
}) {
  if (actions.length === 0) {
    return null;
  }

  return (
    <Section title="What the actions answer" description="Never sent anywhere. Starts as each action's last live test.">
      {actions.map((id) => {
        const action = sources.actions.find((a) => a.id === id);
        const mock = mocks[id];
        return (
          <div key={id} className="flex flex-col gap-1 px-3 py-2.5">
            <span className="text-body-2-medium text-text-secondary">{action?.title || action?.name || id}</span>
            <Textarea
              aria-label={`${action?.name ?? id} answer`}
              rows={2}
              placeholder='{"status": "delivered"}'
              defaultValue={mock?.body === undefined ? "" : JSON.stringify(mock.body)}
              onChange={(text) => onChange({ ...mocks, [id]: { status: mock?.status ?? 200, body: readJSON(text) } })}
            />
          </div>
        );
      })}
    </Section>
  );
}

function Progress({ draft, last }: { draft: Draft; last?: Simulation }) {
  const numbers = stepNumbers(draft.definition.steps);
  const trace = last?.procedure_trace;
  if (!trace) {
    return (
      <p className="px-3 py-2.5 text-body-2-regular text-text-secondary">
        Steps light up once dimmi follows the procedure.
      </p>
    );
  }
  const status = (s: (typeof trace.steps)[number]) =>
    s.status === "locked"
      ? `locked, needs ${s.needs?.join(", ")}`
      : s.branch_taken
        ? `took ${s.branch_taken}`
        : s.status;

  return (
    <div className="flex flex-col">
      {trace.steps.map((s) => (
        <Row key={s.step_id} label={`Step ${numbers.get(s.step_id.split("/").pop() ?? "") ?? s.step_id}`}>
          <Badge color={s.status === "done" || s.branch_taken ? "primary" : "neutral"}>{status(s)}</Badge>
        </Row>
      ))}
      {trace.guard_events.map((g, index) => (
        <Row
          // biome-ignore lint/suspicious/noArrayIndexKey: guard events have no id and never reorder.
          key={index}
          label={g.type.replaceAll("_", " ")}
          description={[g.attempted_tool, g.missing_tool].filter(Boolean).join(" → ")}
        />
      ))}
    </div>
  );
}

function Simulator({ workspace, draft, sources }: { workspace: string; draft: Draft; sources: ChipSources }) {
  const queryClient = useQueryClient();
  const actions: string[] = [];
  walk(draft.definition.steps, (step) => {
    for (const id of actionsOf(step)) {
      if (!actions.includes(id)) {
        actions.push(id);
      }
    }
  });
  const [persona, setPersona] = useState<SimPersona>({ verified: true, channel: "widget" });
  const [mocks, setMocks] = useState<Record<string, ActionMock>>(() =>
    Object.fromEntries(
      actions.flatMap((id) => {
        const sample = sources.actions.find((a) => a.id === id)?.last_test_response;
        return sample === undefined ? [] : [[id, { status: 200, body: sample }]];
      }),
    ),
  );
  const [turns, setTurns] = useState<Turn[]>([]);
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const [replay, setReplay] = useState("");
  const [name, setName] = useState("");
  const last = turns.at(-1)?.result;

  const send = async (input: Partial<SimulateInput>, shown: string) => {
    setBusy(true);
    try {
      await draft.flush();
      const history: SimMessage[] = turns.flatMap((t) => [
        ...(t.message ? [{ role: "customer" as const, text: t.message }] : []),
        ...(t.result.reply ? [{ role: "agent" as const, text: t.result.reply }] : []),
      ]);
      const result = await simulateProcedure(workspace, draft.server.id, {
        persona,
        action_mocks: mocks,
        history: input.source_conversation_id ? undefined : history,
        active_state: last?.active_state,
        ...input,
      });
      setTurns((before) => [...before, { message: shown, result }]);
      setMessage("");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "The simulation did not run.");
    } finally {
      setBusy(false);
    }
  };

  const save = async () => {
    try {
      await createTestCase(workspace, draft.server.id, {
        name: name.trim() || `Test ${new Date().toLocaleString()}`,
        persona,
        action_mocks: mocks,
        messages: turns.filter((t) => t.message).map((t) => ({ text: t.message })),
        assertions: defaultAssertions(turns.map((t) => t.result)),
      });
      void queryClient.invalidateQueries(testCasesQuery(workspace, draft.server.id));
      toast.success("Saved as a test.");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "The test was not saved.");
    }
  };

  return (
    <>
      <Persona persona={persona} onChange={setPersona} />
      <Mocks actions={actions} sources={sources} mocks={mocks} onChange={setMocks} />
      <Section
        title="Conversation"
        description="dimmi follows the working copy. Nothing reaches a customer or your systems."
        action={
          turns.length > 0 ? (
            <Action size="xs" onPress={() => setTurns([])}>
              Start over
            </Action>
          ) : null
        }
      >
        <div className="flex flex-col gap-2 px-3 py-2.5">
          {turns.length === 0 ? (
            <div className="flex flex-wrap items-end gap-2">
              <div className="w-64">
                <Input
                  aria-label="Conversation to replay"
                  size="small"
                  placeholder="Conversation id, conv_…"
                  value={replay}
                  onChange={setReplay}
                />
              </div>
              <Button
                variant="secondary"
                size="small"
                disabled={busy || !replay.trim()}
                onClick={() => void send({ source_conversation_id: replay.trim() }, "(replayed conversation)")}
              >
                Load from real conversation
              </Button>
            </div>
          ) : null}
          {turns.map((t, index) => (
            // biome-ignore lint/suspicious/noArrayIndexKey: turns only grow.
            <div key={index} className="flex flex-col gap-1">
              {t.message ? (
                <p className="self-end rounded-xl bg-background-secondary-default px-3 py-2 text-body-regular">
                  {t.message}
                </p>
              ) : null}
              <p
                className={cx(
                  "self-start whitespace-pre-wrap rounded-xl px-3 py-2 text-body-regular",
                  t.result.handoff ? "bg-status-orange-background" : "bg-status-blue-background",
                )}
              >
                {t.result.reply || `(${t.result.outcome})`}
              </p>
              <span className="text-body-2-regular text-text-tertiary">
                {[
                  t.result.outcome,
                  t.result.matched ? "followed this procedure" : "",
                  ...t.result.action_calls.map((c) => `${c.blocked ? "blocked" : "called"} ${c.tool}`),
                  t.result.handoff ? `handed over: ${t.result.handoff.reason}` : "",
                ]
                  .filter(Boolean)
                  .join(" · ")}
              </span>
            </div>
          ))}
          {last?.active_state.pending.map((p) => (
            <Row key={p.action_id} label={p.summary} description={p.state.replaceAll("_", " ")}>
              <Button
                size="small"
                disabled={busy}
                onClick={() => void send({ confirmed_action_ids: [p.action_id] }, "")}
              >
                {p.state === "awaiting_confirmation" ? "Simulate customer confirm" : "Simulate teammate approve"}
              </Button>
            </Row>
          ))}
          <Textarea
            aria-label="Customer message"
            placeholder="Write as the customer…"
            rows={2}
            value={message}
            onChange={setMessage}
          />
          <span className="flex flex-wrap items-center gap-2">
            <Button
              size="small"
              loading={busy}
              disabled={busy || !message.trim()}
              onClick={() => void send({ message }, message)}
            >
              Send
            </Button>
            {turns.length > 0 ? (
              <>
                <div className="w-56">
                  <Input aria-label="Test name" size="small" placeholder="Test name" value={name} onChange={setName} />
                </div>
                <Button variant="secondary" size="small" onClick={() => void save()}>
                  Save as test case
                </Button>
              </>
            ) : null}
          </span>
        </div>
      </Section>
      <Section title="Steps">
        <Progress draft={draft} last={last} />
      </Section>
    </>
  );
}

function AssertionRow({
  assertion,
  onChange,
  onRemove,
}: {
  assertion: Assertion;
  onChange: (a: Assertion) => void;
  onRemove: () => void;
}) {
  const field = (key: keyof Assertion, placeholder: string) => (
    <div className="w-36">
      <Input
        aria-label={placeholder}
        size="small"
        placeholder={placeholder}
        defaultValue={String(assertion[key] ?? "")}
        onChange={(value) => onChange({ ...assertion, [key]: value })}
      />
    </div>
  );
  const t = assertion.type;

  return (
    <div className="flex flex-wrap items-center gap-2 px-3 py-1.5">
      <Pick
        label="Assertion"
        choices={assertionChoices}
        value={t}
        onChange={(type) => onChange({ type: type as AssertionType })}
      />
      {t === "action_called" || t === "action_not_called" || t === "action_order" ? field("action", "Action") : null}
      {t === "action_order" ? field("before", "Before action") : null}
      {t === "branch_taken" ? field("step_id", "Step id") : null}
      {t === "branch_taken" ? (
        <Pick
          label="Side"
          choices={[
            ["then", "then"],
            ["else", "else"],
          ]}
          value={assertion.branch ?? "then"}
          onChange={(branch) => onChange({ ...assertion, branch: branch as "then" | "else" })}
        />
      ) : null}
      {t === "reply_contains" || t === "reply_not_contains" ? field("text", "Text") : null}
      {t === "llm_rubric" ? field("rubric", "Rubric") : null}
      {t === "handed_off" ? field("team_id", "Team id (optional)") : null}
      <IconButton size="small" icon={RiDeleteBinLine} aria-label="Remove assertion" onClick={onRemove} />
    </div>
  );
}

function TestCard({
  workspace,
  procedure,
  testCase,
  readOnly,
  running,
  onRun,
}: {
  workspace: string;
  procedure: string;
  testCase: ProcedureTestCase;
  readOnly: boolean;
  running: boolean;
  onRun: () => void;
}) {
  const queryClient = useQueryClient();
  const [assertions, setAssertions] = useState(testCase.assertions);
  const result = testCase.last_result;
  const refresh = () => queryClient.invalidateQueries(testCasesQuery(workspace, procedure));
  const save = async (next: Assertion[]) => {
    setAssertions(next);
    try {
      await updateTestCase(workspace, procedure, testCase.id, { assertions: next });
      void refresh();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "The test was not saved.");
    }
  };

  return (
    <div className="flex flex-col gap-1 border-b border-separator-border py-2.5 last:border-b-0">
      <div className="flex items-center gap-2 px-3">
        <span className="min-w-0 flex-1 truncate text-body-medium">{testCase.name}</span>
        {result ? (
          <Badge color={result.passed ? "neutral" : "primary"}>{result.passed ? "Passing" : "Failing"}</Badge>
        ) : null}
        {readOnly ? null : (
          <>
            <IconButton
              size="small"
              icon={RiPlayLine}
              aria-label={`Run ${testCase.name}`}
              disabled={running}
              onClick={onRun}
            />
            <IconButton
              size="small"
              icon={RiDeleteBinLine}
              aria-label={`Delete ${testCase.name}`}
              onClick={() => void deleteTestCase(workspace, procedure, testCase.id).then(refresh)}
            />
          </>
        )}
      </div>
      <span className="px-3 text-body-2-regular text-text-tertiary">
        {testCase.messages.map((m) => `“${m.text}”`).join(" → ")}
      </span>
      {assertions.map((a, index) => (
        <AssertionRow
          // biome-ignore lint/suspicious/noArrayIndexKey: assertions change in place.
          key={index}
          assertion={a}
          onChange={(next) => void save(assertions.map((x, at) => (at === index ? next : x)))}
          onRemove={() => void save(assertions.filter((_, at) => at !== index))}
        />
      ))}
      {readOnly ? null : (
        <span className="px-3">
          <Action size="xs" onPress={() => void save([...assertions, { type: "reply_contains", text: "" }])}>
            Add assertion
          </Action>
        </span>
      )}
      {result?.assertion_results.map((r, index) => (
        <span
          // biome-ignore lint/suspicious/noArrayIndexKey: results follow the assertions' order.
          key={index}
          className={cx("px-3 text-body-2-regular", r.passed ? "text-text-secondary" : "text-text-error-primary")}
        >
          {r.skipped ? "Skipped" : r.passed ? "Passed" : "Failed"}: {r.message}
        </span>
      ))}
      {result?.error ? <span className="px-3 text-body-2-regular text-text-error-primary">{result.error}</span> : null}
    </div>
  );
}

function Suite({ workspace, draft, readOnly }: { workspace: string; draft: Draft; readOnly: boolean }) {
  const queryClient = useQueryClient();
  const tests = useQuery(testCasesQuery(workspace, draft.server.id));
  const [running, setRunning] = useState(false);
  const list = tests.data ?? [];
  const passing = list.filter((t) => t.last_result?.passed).length;

  const run = async (only?: string[]) => {
    setRunning(true);
    try {
      await draft.flush();
      await runTestCases(workspace, draft.server.id, only);
      await queryClient.invalidateQueries(testCasesQuery(workspace, draft.server.id));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "The tests did not run.");
    } finally {
      setRunning(false);
    }
  };

  return (
    <Section
      title={list.length > 0 ? `Saved tests · ${passing}/${list.length} passing` : "Saved tests"}
      description="Each runs against the working copy with its own customer and mocks."
      loading={tests.isPending}
      action={
        readOnly || list.length === 0 ? null : (
          <Action onPress={() => void run()} loading={running}>
            Run all
          </Action>
        )
      }
    >
      {list.length === 0 ? (
        <p className="px-3 py-2.5 text-body-2-regular text-text-secondary">
          No tests yet. Try the procedure above and save the chat as a test.
        </p>
      ) : null}
      {list.map((test) => (
        <TestCard
          key={test.id}
          workspace={workspace}
          procedure={draft.server.id}
          testCase={test}
          readOnly={readOnly}
          running={running}
          onRun={() => void run([test.id])}
        />
      ))}
    </Section>
  );
}

export function ProcedureTests({
  workspace,
  draft,
  sources,
  readOnly,
}: {
  workspace: string;
  draft: Draft;
  sources: ChipSources;
  readOnly: boolean;
}) {
  return (
    <div className="mx-auto flex w-full max-w-[760px] flex-col gap-8 px-6 pt-8 pb-16">
      {readOnly ? null : <Simulator workspace={workspace} draft={draft} sources={sources} />}
      <Suite workspace={workspace} draft={draft} readOnly={readOnly} />
    </div>
  );
}
