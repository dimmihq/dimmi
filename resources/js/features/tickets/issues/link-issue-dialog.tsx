/*
 * Linking an issue or incident to a ticket, a step at a time in the
 * same dialog as picking a workflow's trigger. First the tool, picked
 * from the workspace's connected trackers and incident tools (skipped
 * when there is only one). Then a choice: a new issue in the tool, or
 * an existing one. A new issue is filed in a team and project picked
 * from lists pulled from the tool (an incident tool asks for a
 * severity, PagerDuty for the service to page); an existing one is
 * found by search or pasted as a URL or key. Nothing is typed as an id.
 */
import { RiArrowLeftLine, RiArrowRightSLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type ReactNode, useState } from "react";

import { SearchComboBox } from "@/components/application/search-combobox/search-combobox";
import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { Select, SelectItem } from "@/components/base/select/select";
import { toast } from "@/components/base/toast/toast";
import { StepMotion } from "@/features/knowledge/step-motion";
import type { Integration } from "@/lib/api/integrations";
import {
  type IssueFound,
  type IssueList,
  type IssueSettings,
  issueOptionsQuery,
  issueSearchQuery,
  issueSettingsQuery,
  linkTicketIssue,
  type NewIssue,
  refreshTicketKeys,
} from "@/lib/api/ticket-issues";

import { ToolMark, ToolTile } from "./tool-logos";
import { groupTint, toolLook } from "./tools";

type Mode = "create" | "link";

/*
 * Picker is a select fed by one of the tool's lists.
 */
function Picker({
  workspace,
  installation,
  list,
  parent = "",
  label,
  value,
  optional = false,
  onChange,
}: {
  workspace: string;
  installation: string;
  list: IssueList;
  parent?: string;
  label: string;
  value: string;
  optional?: boolean;
  onChange: (id: string) => void;
}) {
  const { data, isPending, isError } = useQuery(issueOptionsQuery(workspace, installation, list, parent));
  const options = data ?? [];

  return (
    <Select
      label={label}
      className="w-full"
      placeholder={isPending ? "Loading…" : isError ? "Could not load the list" : optional ? "None" : "Pick one"}
      isDisabled={isPending || isError}
      selectedKey={value || null}
      onSelectionChange={(key) => onChange(key ? String(key) : "")}
    >
      {options.map((o) => (
        <SelectItem key={o.id} id={o.id} textValue={o.name}>
          {o.name}
          {o.detail ? <span className="ms-2 text-text-tertiary">{o.detail}</span> : null}
        </SelectItem>
      ))}
    </Select>
  );
}

/*
 * CreateFields are the pickers a new issue or incident needs in the
 * tool.
 */
function CreateFields({
  workspace,
  tool,
  issue,
  onChange,
}: {
  workspace: string;
  tool: Integration;
  issue: NewIssue;
  onChange: (issue: NewIssue) => void;
}) {
  const look = toolLook(tool.provider);

  if (look.kind === "incidents") {
    return (
      <div className="flex flex-col gap-4">
        {tool.provider === "pagerduty" ? (
          <Picker
            workspace={workspace}
            installation={tool.id}
            list="services"
            label="Service to page"
            value={issue.service_id ?? ""}
            onChange={(id) => onChange({ ...issue, service_id: id })}
          />
        ) : null}
        {tool.provider === "rootly" ? (
          <Picker
            workspace={workspace}
            installation={tool.id}
            list="services"
            label="Service"
            value={issue.service_id ?? ""}
            optional
            onChange={(id) => onChange({ ...issue, service_id: id })}
          />
        ) : null}
        <Picker
          workspace={workspace}
          installation={tool.id}
          list="severities"
          label="Severity"
          value={issue.severity_id ?? ""}
          optional
          onChange={(id) => onChange({ ...issue, severity_id: id })}
        />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      <Picker
        workspace={workspace}
        installation={tool.id}
        list="teams"
        label={look.team ?? "Team"}
        value={issue.team_id ?? ""}
        onChange={(id) => onChange({ ...issue, team_id: id, project_id: "" })}
      />
      {look.project && issue.team_id ? (
        <Picker
          workspace={workspace}
          installation={tool.id}
          list="projects"
          parent={issue.team_id}
          label={look.project}
          value={issue.project_id ?? ""}
          optional={tool.provider === "linear"}
          onChange={(id) => onChange({ ...issue, project_id: id })}
        />
      ) : null}
    </div>
  );
}

/*
 * LinkFields find an existing issue with an autocomplete over the tool's
 * search, or take a pasted URL.
 */
function LinkFields({
  workspace,
  tool,
  onPick,
}: {
  workspace: string;
  tool: Integration;
  onPick: (ref: string) => void;
}) {
  const [words, setWords] = useState("");
  const [typed, setTyped] = useState("");
  const pasted = /^https?:\/\//.test(typed.trim());
  const { data, isFetching } = useQuery({
    ...issueSearchQuery(workspace, tool.id, words),
    enabled: words !== "" && !pasted,
  });

  return (
    <div className="flex flex-col gap-3">
      <SearchComboBox
        label="Find the issue"
        placeholder={toolLook(tool.provider).search}
        autoFocus
        items={pasted ? [] : (data ?? [])}
        searching={isFetching}
        keyOf={(issue: IssueFound) => issue.external_id}
        textOf={(issue) => `${issue.key} ${issue.title}`}
        onSearch={setWords}
        onType={(text) => {
          setTyped(text);
          if (/^https?:\/\//.test(text.trim())) {
            onPick(text.trim());
          }
        }}
        onPick={(issue) => onPick(issue ? issue.url || issue.key : "")}
        empty={pasted ? "dimmi reads this issue from the link." : "Nothing matches. Try other words or paste the link."}
      >
        {(issue) => (
          <>
            <ToolMark provider={tool.provider} />
            <span className="shrink-0 text-body-2-medium text-text-secondary">{issue.key}</span>
            <span className="min-w-0 flex-1 truncate text-text-primary">{issue.title}</span>
            <Chip variant="caption" color={groupTint[issue.status_group] ?? "neutral"}>
              {issue.status}
            </Chip>
          </>
        )}
      </SearchComboBox>
      {pasted ? (
        <p className="text-caption-1-regular text-text-tertiary">dimmi reads this issue from the link.</p>
      ) : null}
    </div>
  );
}

/*
 * isReady says whether the details are enough to send.
 */
function isReady(tool: Integration | undefined, mode: Mode, issue: NewIssue, ref: string): boolean {
  if (!tool) {
    return false;
  }
  if (mode === "link") {
    return ref !== "";
  }
  if (toolLook(tool.provider).kind === "incidents") {
    return tool.provider !== "pagerduty" || !!issue.service_id;
  }

  return !!issue.team_id && (issue.title ?? "").trim() !== "";
}

/*
 * withDefaults fills the details the person has not picked yet with the
 * tool's defaults from its settings: the team, project, service and
 * severity.
 */
function withDefaults(issue: NewIssue, settings: IssueSettings): NewIssue {
  const pick = (current: string | undefined, fallback: string) => (current ? current : fallback || undefined);

  return {
    ...issue,
    team_id: pick(issue.team_id, settings.team_id),
    project_id: issue.team_id ? issue.project_id : pick(issue.project_id, settings.project_id),
    service_id: pick(issue.service_id, settings.service_id),
    severity_id: pick(issue.severity_id, settings.severity_id),
  };
}

/*
 * Choice is one card in a list of choices; picking it moves on.
 */
function Choice({
  mark,
  title,
  detail,
  onPick,
}: {
  mark: ReactNode;
  title: string;
  detail: string;
  onPick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onPick}
      className="flex w-full cursor-pointer items-center gap-3 rounded-xl px-2.5 py-2 text-start outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
    >
      {mark}
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="text-body-regular text-text-primary">{title}</span>
        <span className="truncate text-body-2-regular text-text-secondary">{detail}</span>
      </span>
      <RiArrowRightSLine aria-hidden className="size-5 shrink-0 text-foreground-icon-tertiary rtl:-scale-x-100" />
    </button>
  );
}

function Choices({ children }: { children: ReactNode }) {
  return <div className="flex flex-col gap-0.5 rounded-2xl bg-background-secondary-default p-1">{children}</div>;
}

/*
 * modeChoices name the two ways of linking for the tool.
 */
function modeChoices(tool: Integration): { id: Mode; title: string; detail: string; mark: ReactNode }[] {
  const look = toolLook(tool.provider);
  const incidents = look.kind === "incidents";

  return [
    {
      id: "create",
      title: incidents ? `Declare in ${look.label}` : `New ${look.label} issue`,
      detail: incidents
        ? `Open an incident in ${look.label} for this ticket.`
        : `Create it in ${look.label} from this ticket.`,
      mark: <ToolTile provider={tool.provider} />,
    },
    {
      id: "link",
      title: "Link an existing one",
      detail: `Search ${look.label} or paste a link.`,
      mark: <ToolTile provider={tool.provider} />,
    },
  ];
}

type Step = "tool" | "mode" | "details";

/*
 * heading names the dialog for the tool, or for linking in general
 * before one is picked.
 */
function heading(tool: Integration | undefined): { title: ReactNode; description: string } {
  if (!tool) {
    return { title: "Link an issue", description: "Customers see this ticket's states, never the tool's." };
  }
  const look = toolLook(tool.provider);

  return {
    title: `Link ${/^[aeiou]/i.test(look.label) ? "an" : "a"} ${look.label} ${look.kind === "incidents" ? "incident" : "issue"}`,
    description: `Customers see this ticket's states, never ${look.label}'s.`,
  };
}

/*
 * StepBody is what the step on screen asks for.
 */
function StepBody({
  workspace,
  step,
  tools,
  tool,
  mode,
  issue,
  onTool,
  onMode,
  onIssue,
  onPick,
}: {
  workspace: string;
  step: Step;
  tools: Integration[];
  tool: Integration | undefined;
  mode: Mode;
  issue: NewIssue;
  onTool: (id: string) => void;
  onMode: (mode: Mode) => void;
  onIssue: (issue: NewIssue) => void;
  onPick: (ref: string) => void;
}) {
  if (step === "tool" || !tool) {
    return (
      <Choices>
        {tools.map((t) => (
          <Choice
            key={t.id}
            mark={<ToolTile provider={t.provider} />}
            title={toolLook(t.provider).label}
            detail={`${t.name} · ${toolLook(t.provider).kind === "incidents" ? "Incidents" : "Issues"}`}
            onPick={() => onTool(t.id)}
          />
        ))}
      </Choices>
    );
  }

  if (step === "mode") {
    return (
      <Choices>
        {modeChoices(tool).map((c) => (
          <Choice key={c.id} mark={c.mark} title={c.title} detail={c.detail} onPick={() => onMode(c.id)} />
        ))}
      </Choices>
    );
  }

  if (mode === "link") {
    return <LinkFields workspace={workspace} tool={tool} onPick={onPick} />;
  }

  return (
    <>
      <Input label="Title" value={issue.title ?? ""} onChange={(value) => onIssue({ ...issue, title: value })} />
      <CreateFields workspace={workspace} tool={tool} issue={issue} onChange={onIssue} />
      <p className="text-caption-1-regular text-text-tertiary">
        The description lists the customers, companies and revenue this reaches, with a link back here, and stays
        current as conversations are linked.
      </p>
    </>
  );
}

export function LinkIssueDialog({
  workspace,
  ticket,
  title,
  tools,
  onClose,
}: {
  workspace: string;
  ticket: string;
  title: string;
  tools: Integration[];
  onClose: () => void;
}) {
  const queryClient = useQueryClient();
  const single = tools.length === 1;
  const [toolId, setToolId] = useState(single ? tools[0].id : "");
  const [step, setStep] = useState<Step>(single ? "mode" : "tool");
  const [forward, setForward] = useState(true);
  const [mode, setMode] = useState<Mode>("create");
  const [issue, setIssue] = useState<NewIssue>({ title });
  const [ref, setRef] = useState("");
  const [seeded, setSeeded] = useState("");
  const tool = tools.find((t) => t.id === toolId);
  const { data: settings } = useQuery({ ...issueSettingsQuery(workspace, toolId), enabled: toolId !== "" });

  if (settings && settings.installation_id === toolId && seeded !== toolId) {
    setSeeded(toolId);
    setIssue((current) => withDefaults(current, settings));
  }

  const look = tool ? toolLook(tool.provider) : undefined;
  const incidents = look?.kind === "incidents";
  const head = heading(step === "tool" ? undefined : tool);

  const link = useMutation({
    mutationFn: () => {
      if (!tool) {
        throw new Error("Pick a tool first.");
      }
      const body: NewIssue =
        mode === "link" ? { installation_id: tool.id, url: ref } : { ...issue, installation_id: tool.id };

      return linkTicketIssue(workspace, ticket, body);
    },
    onSuccess: async (made) => {
      await Promise.all(
        refreshTicketKeys(workspace, ticket).map((queryKey) => queryClient.invalidateQueries({ queryKey })),
      );
      toast.success(`${look?.label ?? "The issue"} ${made.key} is linked.`);
      onClose();
    },
    onError: (error) => toast.error(error.message),
  });

  function go(next: Step, ahead: boolean) {
    setForward(ahead);
    setStep(next);
  }

  function pickTool(id: string) {
    if (id !== toolId) {
      setToolId(id);
      setIssue((current) => ({ title: current.title }));
      setRef("");
      setSeeded("");
    }
    go("mode", true);
  }

  function pickMode(next: Mode) {
    setMode(next);
    go("details", true);
  }

  const back =
    step === "details" ? () => go("mode", false) : step === "mode" && !single ? () => go("tool", false) : null;
  const submit = mode === "link" ? "Link" : incidents ? "Declare" : "Create issue";

  return (
    <Modal
      size="md"
      className="h-[min(640px,calc(100dvh-3rem))]"
      isOpen
      onOpenChange={(open) => (open ? undefined : onClose())}
    >
      <ModalHeader title={head.title} description={head.description} />
      <ModalBody>
        <StepMotion view={step} forward={forward} className="flex flex-col gap-4 pb-2">
          <StepBody
            workspace={workspace}
            step={step}
            tools={tools}
            tool={tool}
            mode={mode}
            issue={issue}
            onTool={pickTool}
            onMode={pickMode}
            onIssue={setIssue}
            onPick={setRef}
          />
        </StepMotion>
      </ModalBody>
      <ModalFooter>
        {back ? (
          <Button variant="secondary" leadingIcon={RiArrowLeftLine} onClick={back}>
            Back
          </Button>
        ) : (
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
        )}
        {step === "details" ? (
          <Button disabled={!isReady(tool, mode, issue, ref)} loading={link.isPending} onClick={() => link.mutate()}>
            {submit}
          </Button>
        ) : null}
      </ModalFooter>
    </Modal>
  );
}
