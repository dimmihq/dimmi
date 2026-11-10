/*
 * The series the pages show until the server keeps them: a few in every
 * state for the list, and one whole onboarding series, step by step, with
 * the numbers each step carries once it runs, so the builder and its
 * results can be judged on believable content before anything is stored.
 */

export type SeriesStatus = "live" | "draft" | "paused";

/*
 * SeriesRow is one series as the list shows it: who it starts for, where
 * it stands, how many people are in it now, how many ever entered and
 * how many reached its goal.
 */
export interface SeriesRow {
  id: string;
  name: string;
  entry: string;
  status: SeriesStatus;
  active: number;
  entered: number;
  goal: number;
  edited: string;
}

export const seriesList: SeriesRow[] = [
  {
    id: "srs_onboarding",
    name: "Onboarding: first week",
    entry: "Signed up, messenger not installed",
    status: "live",
    active: 1204,
    entered: 8930,
    goal: 3482,
    edited: "Nov 7",
  },
  {
    id: "srs_trial_end",
    name: "Trial ending",
    entry: "Trial ends within 5 days",
    status: "live",
    active: 318,
    entered: 2140,
    goal: 621,
    edited: "Oct 30",
  },
  {
    id: "srs_agent_adoption",
    name: "Turn on the agent",
    entry: "Help center has 10+ articles, agent off",
    status: "paused",
    active: 0,
    entered: 1288,
    goal: 402,
    edited: "Oct 22",
  },
  {
    id: "srs_winback",
    name: "Win back quiet workspaces",
    entry: "No conversation answered in 21 days",
    status: "draft",
    active: 0,
    entered: 0,
    goal: 0,
    edited: "Nov 9",
  },
  {
    id: "srs_nps_followup",
    name: "Follow up on low NPS",
    entry: "Answered NPS with 0–6",
    status: "live",
    active: 41,
    entered: 377,
    goal: 133,
    edited: "Sep 18",
  },
];

export type NodeKind = "entry" | "email" | "chat" | "post" | "banner" | "wait" | "branch" | "tag" | "webhook" | "exit";

/*
 * NodeStats are what one step did while the series ran: how many people
 * reached it, how many it was sent to and what they did, and how many
 * are waiting on it right now.
 */
export interface NodeStats {
  reached: number;
  sent?: number;
  opened?: number;
  clicked?: number;
  replied?: number;
  waiting?: number;
}

/*
 * SeriesNode is one step of a series: what it does, in a short sentence,
 * and the steps after it. A branch has two paths, each under its label.
 */
export interface SeriesNode {
  id: string;
  kind: NodeKind;
  summary: string;
  subject?: string;
  body?: string[];
  stats?: NodeStats;
  next?: SeriesNode[];
  label?: string;
}

export const onboarding: SeriesNode = {
  id: "entry",
  kind: "entry",
  summary: "Signed up in the last day and hasn't installed the messenger",
  stats: { reached: 8930 },
  next: [
    {
      id: "welcome",
      kind: "email",
      summary: "Welcome to Acme: three things to do first",
      subject: "Welcome to Acme: three things to do first",
      body: [
        'Hi {{ contact.first_name | default: "there" }},',
        "Thanks for signing up. Most teams get value out of Acme in their first hour by doing three things: install the messenger, connect your inbox, and invite a teammate.",
        "Start with the messenger. It takes two minutes.",
      ],
      stats: { reached: 8930, sent: 8902, opened: 5341, clicked: 2214 },
      next: [
        {
          id: "wait-2d",
          kind: "wait",
          summary: "Wait 2 days",
          stats: { reached: 8902, waiting: 412 },
          next: [
            {
              id: "installed",
              kind: "branch",
              summary: "Installed the messenger?",
              stats: { reached: 8490 },
              next: [
                {
                  id: "tips",
                  kind: "chat",
                  label: "Yes",
                  summary: "Nice! Want me to set up your first workflow?",
                  body: [
                    "Nice work getting the messenger live 🎉",
                    "Want me to set up your first workflow? Reply and I'll walk you through it.",
                  ],
                  stats: { reached: 4120, sent: 3876, opened: 2702, replied: 388 },
                  next: [
                    {
                      id: "tag-activated",
                      kind: "tag",
                      summary: "Tag as activated",
                      stats: { reached: 3876 },
                      next: [{ id: "exit-yes", kind: "exit", summary: "Leave the series", stats: { reached: 3876 } }],
                    },
                  ],
                },
                {
                  id: "nudge",
                  kind: "email",
                  label: "No",
                  summary: "Your messenger is two minutes away",
                  subject: "Your messenger is two minutes away",
                  body: [
                    'Hi {{ contact.first_name | default: "there" }},',
                    "You haven't installed the messenger yet. Paste one line before </body>, or use the WordPress, Shopify or Webflow plugin.",
                    "Stuck? Reply to this email and we'll do it with you.",
                  ],
                  stats: { reached: 4370, sent: 4311, opened: 2155, clicked: 734 },
                  next: [
                    {
                      id: "wait-3d",
                      kind: "wait",
                      summary: "Wait 3 days",
                      stats: { reached: 4311, waiting: 792 },
                      next: [
                        {
                          id: "notify-sales",
                          kind: "webhook",
                          summary: "Tell the CRM: needs a setup call",
                          stats: { reached: 3519 },
                          next: [
                            {
                              id: "exit-no",
                              kind: "exit",
                              summary: "Leave the series",
                              stats: { reached: 3519 },
                            },
                          ],
                        },
                      ],
                    },
                  ],
                },
              ],
            },
          ],
        },
      ],
    },
  ],
};

/*
 * seriesById finds a sample series, falling back to the first so any
 * address opens something.
 */
export function seriesById(id: string): SeriesRow {
  return seriesList.find((series) => series.id === id) ?? (seriesList[0] as SeriesRow);
}

/*
 * findNode looks a step up by its id anywhere in the series.
 */
export function findNode(node: SeriesNode, id: string): SeriesNode | undefined {
  if (node.id === id) {
    return node;
  }
  for (const child of node.next ?? []) {
    const found = findNode(child, id);
    if (found) {
      return found;
    }
  }

  return undefined;
}

export function percent(part: number, whole: number): string {
  if (whole === 0) {
    return "0%";
  }

  return `${Math.round((part / whole) * 1000) / 10}%`;
}
