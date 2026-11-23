/*
 * The tools a routine's prompt names. Each is a chip in the writer, kept
 * in the prompt as [[tool:name]], which the server spells out as the
 * tool's name for the model. A tool from a group the routine does not
 * have yet brings that group with it, so naming a tool is enough to let
 * the routine call it.
 */
import type { RoutineTool, RoutineToolGroup } from "@/lib/api/routines";

export const toolMarker = /\[\[tool:([a-z0-9_]+)\]\]/g;

export const groupNames: Record<RoutineTool["group"], string> = {
  routine: "Every routine",
  conversations: "Conversations",
  knowledge: "Knowledge",
  insights: "Reports",
  changes: "Suggest changes",
  actions: "Actions",
  github: "GitHub",
};

const words: Record<string, string> = { github: "GitHub", pr: "PR", prs: "PRs", noop: "nothing found" };

/*
 * toolLabel names a tool the way a teammate reads it:
 * list_recent_changesets is "List recent changesets".
 */
export function toolLabel(name: string): string {
  const out = name
    .split("_")
    .filter(Boolean)
    .map((w) => words[w] ?? w)
    .join(" ");
  return out.charAt(0).toUpperCase() + out.slice(1);
}

/*
 * firstSentence is a tool's description cut to its first sentence.
 */
function firstSentence(text: string): string {
  const end = text.search(/[.;:](\s|$)/);
  return end === -1 ? text : text.slice(0, end);
}

export interface ToolOption {
  tool: RoutineTool;
  label: string;
  hint: string;
  /* heading is the group's name, on the first option of each group. */
  heading: string | null;
  /* adds says naming the tool gives the routine its group. */
  adds: boolean;
}

/*
 * findTools lists the tools whose name matches what was typed after the
 * @, grouped, the groups the routine has first.
 */
export function findTools(tools: RoutineTool[], has: RoutineToolGroup[], query: string): ToolOption[] {
  const needle = query.trim().toLowerCase().replaceAll(" ", "_");
  const owned = (t: RoutineTool) => t.group === "routine" || has.includes(t.group);
  const matching = tools.filter((t) => !needle || t.name.includes(needle));
  const order = Object.keys(groupNames);
  const sorted = [...matching].sort((a, b) => order.indexOf(a.group) - order.indexOf(b.group));
  const ordered = [...sorted.filter(owned), ...sorted.filter((t) => !owned(t))];
  let group = "";
  return ordered.map((tool) => {
    const first = tool.group !== group;
    group = tool.group;
    const adds = !owned(tool);
    return {
      tool,
      label: toolLabel(tool.name),
      hint: firstSentence(tool.description),
      heading: first ? `${groupNames[tool.group]}${adds ? " · gives access" : ""}` : null,
      adds,
    };
  });
}

/*
 * toolsIn lists the tools a prompt names.
 */
export function toolsIn(prompt: string): string[] {
  return [...prompt.matchAll(toolMarker)].map((m) => m[1] ?? "");
}

/*
 * withNamedGroups adds the groups of the tools a prompt names to the
 * routine's groups.
 */
export function withNamedGroups(groups: RoutineToolGroup[], prompt: string, tools: RoutineTool[]): RoutineToolGroup[] {
  const out = new Set(groups);
  for (const name of toolsIn(prompt)) {
    const group = tools.find((t) => t.name === name)?.group;
    if (group && group !== "routine") {
      out.add(group);
    }
  }
  return out.size === groups.length ? groups : [...out];
}
