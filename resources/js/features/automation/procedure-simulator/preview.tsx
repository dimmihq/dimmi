/*
 * Previewing a procedure: a back-and-forth chat with dimmi as a
 * customer, following the working copy, in the same shape as a chat
 * with Operator. What the tester writes shows at once. dimmi's turn
 * opens with a log of what it did, drawn step by step: the procedure it
 * followed, each step it went through, every action it called or was
 * refused, where a guardrail stepped in, and a handover. The reply
 * follows, and the log folds behind one line. Nothing reaches a
 * customer, and no action is called for real: the server answers each
 * with the result of its last live test.
 *
 * The plus button in the reply box adds what a preview can start from:
 * a real customer to test as, found in a pop-up, or a real conversation
 * to replay, found the same way. What was added shows above the box and
 * can be taken off again; a conversation to replay is sent with the
 * send button, nothing typed. A write that waits on the customer or a
 * teammate waits here too, until the tester confirms or approves it. A
 * chat worth keeping is saved as a simulation in one click.
 */
import { RiChat3Line, RiRestartLine, RiTestTubeLine } from "@remixicon/react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";

import { Avatar } from "@/components/base/avatar/avatar";
import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { AlertDialog } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { subjectOf } from "@/features/inbox/conversations";
import { MessageBody } from "@/features/inbox/message-body";
import { ActivityLog } from "@/features/operator/activity-log";
import { avatarColor } from "@/features/shell/avatar-color";
import { ScrollArea } from "@/features/shell/page";
import { Reveal } from "@/features/shell/reveal";
import type { Conversation } from "@/lib/api/inbox";
import type { OperatorStep } from "@/lib/api/operator";
import {
  createTestCase,
  type SimMessage,
  type SimPersona,
  type SimulateInput,
  type Simulation,
  simulateProcedure,
  testCasesQuery,
} from "@/lib/api/procedures";
import { callerQuery } from "@/lib/api/workspace";
import type { Envelope } from "@/lib/realtime/bridge";
import { channels } from "@/lib/realtime/keys";
import { useChannel } from "@/lib/realtime/use-channel";

import type { ChipSources } from "../procedure-writer/procedure-suggestions";
import { stepNumbers, walk } from "../procedures";
import { stepTitle } from "../step-words";
import { PreviewAddMenu } from "./preview-add-menu";
import { Added, PreviewComposer } from "./preview-composer";
import {
  actionName,
  card,
  type Draft,
  defaultAssertions,
  fromCustomer,
  hint,
  type LiveStep,
  liveStep,
  words,
} from "./simulator";

interface Turn {
  message: string;
  /* When it was sent: a simulation saved from the chat runs on this day. */
  at: string;
  result: Simulation;
  /* What dimmi did in the turn, in the order it did it. */
  steps: OperatorStep[];
}

/*
 * Streamed is what a preview turn sends while it runs: each tool dimmi
 * finished, under the id the preview gave the turn.
 */
interface Streamed {
  stream_id?: string;
  step?: LiveStep;
}

const madeUp: SimPersona = { verified: true, channel: "widget" };

/*
 * historyOf is the chat so far as the simulator reads it.
 */
function historyOf(turns: Turn[]): SimMessage[] {
  return turns.flatMap((t) => [
    ...(t.message ? [{ role: "customer" as const, text: t.message }] : []),
    ...(t.result.reply ? [{ role: "agent" as const, text: t.result.reply }] : []),
  ]);
}

/*
 * eventsOf reads a reply as the things dimmi did in that turn, for when
 * none of it was streamed: the procedure when it was first followed,
 * the steps it finished since the turn before, the actions, the
 * guardrails and a handover.
 */
function eventsOf(
  result: Simulation,
  before: Simulation | undefined,
  draft: Draft,
  sources: ChipSources,
): OperatorStep[] {
  const trace = result.procedure_trace;
  const started: OperatorStep[] = [];
  if (result.matched && !before?.matched) {
    started.push({
      tool: "follow_procedure",
      summary: `Followed ${trace?.procedure_name ?? "this procedure"}`,
      ok: true,
    });
  }
  if (!result.matched) {
    started.push({ tool: "follow_procedure", summary: "Did not follow this procedure", ok: false });
  }
  const calls = result.action_calls.map((call): OperatorStep => {
    const name = actionName(sources, call.action_id || call.tool);

    return call.blocked
      ? { tool: "action_blocked", summary: `Refused ${name}`, ok: false }
      : { tool: "action_call", summary: `Called ${name}`, ok: call.succeeded };
  });
  const guards = (trace?.guard_events ?? []).map(
    (guard): OperatorStep => ({
      tool: "guardrail",
      summary: `Guardrail: ${words(guard.type).toLowerCase()}`,
      ok: false,
    }),
  );
  const handover: OperatorStep[] = result.handoff
    ? [{ tool: "handoff", summary: `Handed over: ${words(result.handoff.reason).toLowerCase()}`, ok: true }]
    : [];

  return [...started, ...stepEvents(result, before, draft, sources), ...calls, ...guards, ...handover];
}

function sideOf(branch: "then" | "else" | undefined): string {
  if (!branch) {
    return "";
  }

  return branch === "then" ? " (if so)" : " (if not)";
}

/*
 * stepEvents are the steps dimmi went through, each by its number and
 * words, with the side a rule took or what a locked step waits for.
 */
function stepEvents(
  result: Simulation,
  before: Simulation | undefined,
  draft: Draft,
  sources: ChipSources,
): OperatorStep[] {
  const numbers = stepNumbers(draft.definition.steps);
  const steps = new Map<string, Parameters<typeof stepTitle>[0]>();
  walk(draft.definition.steps, (step) => {
    steps.set(step.id, step);
  });

  const old = new Set((before?.procedure_trace?.steps ?? []).filter((s) => s.status === "done").map((s) => s.step_id));

  return (result.procedure_trace?.steps ?? [])
    .filter((s) => s.status !== "pending" && !old.has(s.step_id))
    .map((s) => {
      const id = s.step_id.split("/").pop() ?? s.step_id;
      const title = `Step ${numbers.get(id) ?? "–"}: ${stepTitle(steps.get(id), sources)}`;
      if (s.status === "locked") {
        const needs = s.needs?.join(", ") || "an earlier step";

        return { tool: "step", summary: `${title} (waiting for ${needs})`, ok: false };
      }

      return { tool: "step", summary: `${title}${sideOf(s.branch_taken)}`, ok: true };
    });
}

/*
 * Reply is dimmi's turn: the log of what it did, folded behind one
 * line, then what it said. A reply that just arrived eases in.
 */
function Reply({ turn, fresh }: { turn: Turn; fresh: boolean }) {
  return (
    <div className="flex flex-col gap-3">
      <ActivityLog steps={turn.steps} working={false} />
      <Reveal animate={fresh}>
        <MessageBody
          text={turn.result.reply || words(turn.result.outcome)}
          className="text-body-regular text-text-primary"
        />
      </Reveal>
    </div>
  );
}

/*
 * AddedRow lists what was added to the preview, or nothing: the
 * customer dimmi answers as, with their face and email, and the
 * conversation being replayed.
 */
function AddedRow({
  persona,
  replay,
  onNoCustomer,
  onNoReplay,
}: {
  persona: SimPersona;
  replay: Conversation | null;
  onNoCustomer: () => void;
  onNoReplay: () => void;
}) {
  const customer = persona.name || persona.email;
  if (!customer && !replay) {
    return null;
  }

  return (
    <div className="flex flex-wrap gap-1.5 px-0.5 pt-0.5">
      {customer ? (
        <Added
          mark={<Avatar size="md" color={avatarColor(customer)} initials={customer.charAt(0).toUpperCase()} />}
          title={customer}
          detail={persona.name && persona.email ? `Testing as ${persona.email}` : "Testing as this customer"}
          onRemove={onNoCustomer}
        />
      ) : null}
      {replay ? (
        <Added
          mark={
            <span className="flex size-8 items-center justify-center rounded-full bg-background-tertiary-default">
              <RiChat3Line aria-hidden className="size-4 text-foreground-icon-secondary" />
            </span>
          }
          title={subjectOf(replay)}
          detail={`Replaying conversation #${replay.number}`}
          onRemove={onNoReplay}
        />
      ) : null}
    </div>
  );
}

function Blank() {
  return (
    <li className="flex flex-col items-center gap-3 text-center">
      <span className={`flex size-10 items-center justify-center rounded-full ${card}`}>
        <RiTestTubeLine aria-hidden className="size-5 text-foreground-icon-secondary" />
      </span>
      <div className="flex flex-col gap-1">
        <span className="text-body-medium text-text-primary">Write to dimmi as a customer would</span>
        <span className={hint}>Nothing reaches a customer or your systems.</span>
      </div>
    </li>
  );
}

export function ProcedurePreview({
  workspace,
  draft,
  sources,
  onSaved,
}: {
  workspace: string;
  draft: Draft;
  sources: ChipSources;
  /* Told when the chat was saved as a simulation. */
  onSaved: () => void;
}) {
  const queryClient = useQueryClient();
  const [persona, setPersona] = useState<SimPersona>(madeUp);
  const [replay, setReplay] = useState<Conversation | null>(null);
  const [turns, setTurns] = useState<Turn[]>([]);
  /* What was just sent and is being answered, shown before the answer is in. */
  const [asked, setAsked] = useState<string | null>(null);
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const [saving, setSaving] = useState(false);
  const [name, setName] = useState("");
  /* The reply being waited for; stopping forgets it, so a late answer is dropped. */
  const waiting = useRef(0);
  /* The id of the turn being streamed, and the steps it has sent so far. */
  const stream = useRef("");
  const streamed = useRef<OperatorStep[]>([]);
  const [live, setLive] = useState<OperatorStep[]>([]);
  /* How many turns were on screen when the newest arrived, so only it eases in. */
  const seen = useRef(0);
  const end = useRef<HTMLDivElement>(null);
  const last = turns.at(-1)?.result;
  const { data: caller } = useQuery(callerQuery(workspace));
  const member = caller?.member?.id ?? null;

  useChannel(member ? channels.member(member) : null, {
    onEnvelope: (envelope: Envelope) => {
      const data = (envelope.data ?? {}) as Streamed;
      if (envelope.event !== "procedure.preview.step" || !data.step || data.stream_id !== stream.current) {
        return;
      }
      streamed.current = [...streamed.current, liveStep(data.step, sources)];
      setLive(streamed.current);
    },
  });

  // biome-ignore lint/correctness/useExhaustiveDependencies: the newest line is kept in view as the chat grows.
  useEffect(() => {
    end.current?.scrollIntoView({ block: "end" });
  }, [turns.length, asked, live.length]);

  const simulate = async (input: Partial<SimulateInput>): Promise<Simulation | string> => {
    try {
      await draft.flush();

      return await simulateProcedure(workspace, draft.server.id, {
        persona,
        history: input.source_conversation_id ? undefined : historyOf(turns),
        active_state: last?.active_state,
        stream_id: stream.current,
        ...input,
      });
    } catch (error) {
      return error instanceof Error ? error.message : "The preview did not run.";
    }
  };

  const send = async (input: Partial<SimulateInput>, shown: string) => {
    waiting.current += 1;
    const turn = waiting.current;
    stream.current = crypto.randomUUID();
    streamed.current = [];
    setLive([]);
    setBusy(true);
    setAsked(shown);
    setMessage("");
    const result = await simulate(input);
    if (waiting.current !== turn) {
      return;
    }
    setBusy(false);
    setAsked(null);
    if (typeof result === "string") {
      setMessage(shown);
      toast.error(result);
      return;
    }
    setReplay(null);
    const steps = streamed.current.length > 0 ? streamed.current : eventsOf(result, last, draft, sources);
    seen.current = turns.length;
    setTurns((before) => [...before, { message: shown, at: new Date().toISOString(), result, steps }]);
  };

  const submit = () => {
    if (busy) {
      return;
    }
    if (replay) {
      void send({ source_conversation_id: replay.id }, "");
      return;
    }
    const text = message.trim();
    if (text !== "") {
      void send({ message: text }, text);
    }
  };

  const save = async () => {
    const said = turns.find((t) => t.message)?.message ?? "";
    try {
      await createTestCase(workspace, draft.server.id, {
        name: name.trim() || said.slice(0, 60) || "Simulation",
        persona,
        messages: turns.map((t) => (t.message ? { text: t.message } : { text: "", confirm: true })),
        assertions: defaultAssertions(turns.map((t) => t.result)),
        simulated_at: turns[0]?.at ?? new Date().toISOString(),
      });
      void queryClient.invalidateQueries(testCasesQuery(workspace, draft.server.id));
      setName("");
      toast.success("Saved as a simulation.");
      onSaved();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "The simulation was not saved.");
    }
  };

  const stop = () => {
    waiting.current += 1;
    setBusy(false);
    setAsked(null);
  };

  const restart = () => {
    stop();
    seen.current = 0;
    setTurns([]);
  };

  const blank = turns.length === 0 && asked === null;

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {turns.length > 0 ? (
        <div className="flex h-10 shrink-0 items-center justify-end gap-1 px-3">
          <Button variant="ghost" size="xs" disabled={busy} onClick={() => setSaving(true)}>
            Save as simulation
          </Button>
          <Button
            variant="ghost"
            size="xs"
            iconOnly
            leadingIcon={RiRestartLine}
            aria-label="Start over"
            onClick={restart}
          />
        </div>
      ) : null}
      <ScrollArea className="flex flex-col">
        <ol
          aria-label="Preview conversation"
          className={`flex flex-1 flex-col gap-5 px-4 pt-4 pb-6 ${blank ? "justify-center" : "justify-end"}`}
        >
          {blank ? <Blank /> : null}
          {turns.map((t, index) => (
            // biome-ignore lint/suspicious/noArrayIndexKey: turns only grow.
            <li key={index} className="flex flex-col gap-5">
              {t.message ? <p className={fromCustomer}>{t.message}</p> : null}
              <Reply turn={t} fresh={index >= seen.current} />
            </li>
          ))}
          {asked === null ? null : (
            <li className="flex flex-col gap-5">
              {asked ? (
                <Reveal>
                  <p className={fromCustomer}>{asked}</p>
                </Reveal>
              ) : null}
              <ActivityLog steps={live} working />
            </li>
          )}
          {busy
            ? null
            : last?.active_state.pending.map((p) => (
                <li key={p.action_id} className={`flex flex-col items-start gap-2 p-3 ${card}`}>
                  <span className="text-body-2-medium text-text-primary">{p.summary}</span>
                  <Button
                    variant="secondary"
                    size="small"
                    onClick={() => void send({ confirmed_action_ids: [p.action_id] }, "")}
                  >
                    {p.state === "awaiting_confirmation" ? "Confirm as the customer" : "Approve as a teammate"}
                  </Button>
                </li>
              ))}
        </ol>
        <div ref={end} />
      </ScrollArea>
      <div className="shrink-0 px-4 pb-4">
        <PreviewComposer
          value={message}
          onValueChange={setMessage}
          onSubmit={submit}
          onStop={stop}
          busy={busy}
          placeholder={replay ? "Press send to replay it" : "Write as the customer"}
          canSubmit={replay !== null}
          added={
            <AddedRow
              persona={persona}
              replay={replay}
              onNoCustomer={() => setPersona(madeUp)}
              onNoReplay={() => setReplay(null)}
            />
          }
          menu={
            <PreviewAddMenu
              workspace={workspace}
              replayable={turns.length === 0}
              onConversation={setReplay}
              onCustomer={setPersona}
            />
          }
        />
      </div>
      <AlertDialog
        isOpen={saving}
        onOpenChange={setSaving}
        variant="default"
        icon={false}
        title="Save as simulation"
        description="dimmi must answer this chat the same way every time the simulations run."
        confirmLabel="Save"
        onConfirm={save}
      >
        <Input aria-label="Simulation name" placeholder="Name it, or leave empty" value={name} onChange={setName} />
      </AlertDialog>
    </div>
  );
}
