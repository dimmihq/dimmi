/*
 * Imported teammates came over from another tool and wait, without
 * access, until an admin enables them. Each one stands somewhere: they
 * signed up and wait, enabling already mailed them an invitation, or
 * they have not signed up yet. Enabling someone who signed up lets them
 * in at once; anyone else gets an invitation.
 */
import type { Member } from "@/lib/api/workspace";

export type ImportState = "waiting" | "invited" | "not_signed_up";

/*
 * importState says where an imported teammate stands. Signing up counts
 * once they proved their address: until then a stranger may have used it.
 */
export function importState(member: Member): ImportState {
  if (member.user.email_verified) {
    return "waiting";
  }

  return member.enable_invited_at ? "invited" : "not_signed_up";
}

export const stateLabels: Record<ImportState, string> = {
  waiting: "Signed up, waiting",
  invited: "Invite sent",
  not_signed_up: "Not signed up",
};

export const stateColors: Record<ImportState, "orange" | "blue" | "neutral"> = {
  waiting: "orange",
  invited: "blue",
  not_signed_up: "neutral",
};

/*
 * sourceName is how the tool a teammate came from reads: intercom
 * becomes Intercom.
 */
export function sourceName(source: string | null | undefined): string {
  if (!source) {
    return "another tool";
  }

  return source
    .split("_")
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(" ");
}

/*
 * sectionLine says how many imported teammates wait, and from where.
 */
export function sectionLine(members: Member[]): string {
  const sources = [...new Set(members.map((m) => m.imported_from ?? null))];
  const from = sources.length === 1 ? ` from ${sourceName(sources[0])}` : "";
  const count = members.length === 1 ? "1 teammate" : `${members.length} teammates`;

  return `${count}${from} ${members.length === 1 ? "waits" : "wait"} for access. Their history is already here.`;
}

/*
 * roleWords is the role an imported teammate is enabled with, as words.
 */
function roleWords(member: Member): string {
  return member.role === "admin" ? "an admin" : "a member";
}

/*
 * teamWords lists the teams they join, as a clause, "" for none.
 */
function teamWords(teams: string[]): string {
  if (teams.length === 0) {
    return "";
  }
  if (teams.length === 1) {
    return ` and joins ${teams[0]}`;
  }

  return ` and joins ${teams.slice(0, -1).join(", ")} and ${teams[teams.length - 1]}`;
}

export interface EnableText {
  title: string;
  description: string;
  confirm: string;
}

/*
 * enableText is what the confirmation of enabling one teammate says:
 * access now for someone who signed up, an invitation for anyone else.
 */
export function enableText(member: Member, teams: string[]): EnableText {
  const name = member.user.name;
  const first = name.split(" ")[0] || name;

  switch (importState(member)) {
    case "waiting":
      return {
        title: `Enable ${name}?`,
        description: `${first} gets access straight away as ${roleWords(member)}${teamWords(teams)}. We'll email them.`,
        confirm: "Enable",
      };
    case "invited":
      return {
        title: `Send ${name} a new invitation?`,
        description: `${first} hasn't accepted yet. The old link stops working.`,
        confirm: "Resend invite",
      };
    default:
      return {
        title: `Enable ${name}?`,
        description: `${first} hasn't signed up yet. We'll email an invitation; access starts when they accept.`,
        confirm: "Send invite",
      };
  }
}

/*
 * enableAllText is what the confirmation of enabling everyone says, with
 * how many get in now and how many get an invitation.
 */
export function enableAllText(members: Member[]): EnableText {
  const now = members.filter((m) => importState(m) === "waiting").length;
  const invited = members.length - now;
  const parts: string[] = [];
  if (now > 0) {
    parts.push(`${now} who signed up ${now === 1 ? "gets" : "get"} access now`);
  }
  if (invited > 0) {
    parts.push(`${invited} ${invited === 1 ? "gets" : "get"} an invitation by email`);
  }
  const count = members.length === 1 ? "1 teammate" : `${members.length} teammates`;

  return {
    title: `Enable ${count}?`,
    description: `${parts.join("; ")}.`,
    confirm: "Enable all",
  };
}

/*
 * actionLabel is the row's button: enable, or send again once invited.
 */
export function actionLabel(member: Member): string {
  return importState(member) === "invited" ? "Resend" : "Enable";
}
