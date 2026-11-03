/*
 * What the messenger shows besides its own chrome: the team, the visitor's
 * recent conversations and one open thread, the customer's links, help
 * collections and the changelog. The server will send these in the same
 * shape; until it does, the sample below fills the views so the design can
 * be judged with real-looking content.
 */

export interface Member {
  name: string;
}

export interface Conversation {
  number: number;
  title: string;
  preview: string;
  ago: string;
}

/*
 * Incoming is a message the team sends while the visitor is elsewhere,
 * with the conversation it belongs to.
 */
export interface Incoming extends Message {
  conversation: number;
}

export interface Link {
  title: string;
  icon: "calendar" | "book";
  url: string;
}

export interface Block {
  kind: "p" | "h2" | "list" | "steps" | "code" | "note" | "image";
  text?: string;
  items?: string[];
}

export interface Article {
  slug: string;
  title: string;
  summary: string;
  author: string;
  updated: string;
  body: Block[];
}

export interface Collection {
  slug: string;
  title: string;
  description: string;
  articles: string[];
}

export interface Entry {
  tag: string;
  title: string;
  summary: string;
  date: string;
  body: Block[];
}

export interface Attachment {
  kind: "file" | "image" | "voice";
  name: string;
  size?: string;
  duration?: string;
}

export interface Source {
  title: string;
}

export interface Message {
  from: "visitor" | "team" | "agent";
  author?: string;
  body?: string;
  attachment?: Attachment;
  sources?: Source[];
  at: string;
}

export interface Content {
  company: string;
  reply_time: string;
  team: Member[];
  thread: Message[];
  incoming?: Incoming;
  conversations: Conversation[];
  links: Link[];
  collections: Collection[];
  articles: Article[];
  popular: string[];
  changelog: Entry[];
  unread_changelog: number;
}

export const sample: Content = {
  company: "Acme",
  reply_time: "Usually replies in a few minutes",
  team: [{ name: "Maya Kerr" }, { name: "Jonas Reed" }],
  thread: [
    { from: "team", author: "Maya Kerr", body: "Hi there! How can we help today?", at: "10:02" },
    {
      from: "visitor",
      body: "I connected our GitHub repository but the sync never finishes.",
      at: "10:03",
    },
    { from: "visitor", attachment: { kind: "file", name: "sync-error.log", size: "12 KB" }, at: "10:03" },
    {
      from: "agent",
      author: "Dimmi agent",
      body: "That usually means the webhook was not created. Open Settings, then GitHub, and choose Reconnect.",
      sources: [{ title: "GitHub integration guide" }],
      at: "10:03",
    },
    { from: "visitor", attachment: { kind: "voice", name: "Voice note", duration: "0:14" }, at: "10:04" },
    { from: "team", author: "Maya Kerr", attachment: { kind: "image", name: "settings.png" }, at: "10:05" },
    { from: "team", author: "Maya Kerr", body: "Here is where the Reconnect button lives.", at: "10:05" },
  ],
  incoming: {
    conversation: 44308,
    from: "team",
    author: "Maya Kerr",
    body: "Good news: the sync just finished. Want me to walk you through connecting your docs next?",
    at: "now",
  },
  conversations: [
    {
      number: 44308,
      title: "GitHub integration guide",
      preview: "Hi, thanks for reaching out. Open GitHub settings and connect the repository first.",
      ago: "2h",
    },
    {
      number: 44298,
      title: "Workspace setup steps",
      preview: "Hi, thanks for reaching out. Setup takes about five minutes from the dashboard.",
      ago: "14h",
    },
  ],
  links: [
    { title: "Book a call", icon: "calendar", url: "https://trydimmi.com" },
    { title: "Read the docs", icon: "book", url: "https://trydimmi.com" },
  ],
  collections: [
    {
      slug: "getting-started",
      title: "Getting started",
      description: "Set up your workspace and invite your team",
      articles: ["install-the-widget", "invite-your-team"],
    },
    {
      slug: "integrations",
      title: "Integrations",
      description: "Connect the tools you already use",
      articles: ["connect-github"],
    },
    {
      slug: "account-and-billing",
      title: "Account and billing",
      description: "Plans, invoices and seats",
      articles: ["change-your-plan"],
    },
  ],
  articles: [
    {
      slug: "install-the-widget",
      title: "Install the messenger on your site",
      summary: "Add one script tag and the messenger appears on every page.",
      author: "Maya Kerr",
      updated: "Updated this week",
      body: [
        { kind: "p", text: "The messenger loads from a single script tag. It works on any site, framework or CMS." },
        { kind: "h2", text: "Add the snippet" },
        {
          kind: "steps",
          items: [
            "Open Settings, then Messenger.",
            "Copy the snippet under Install.",
            "Paste it just before the closing body tag on every page.",
          ],
        },
        {
          kind: "code",
          text: '<script async src="https://app.trydimmi.com/widget/v1/loader.js" data-app-id="wgt_..."></script>',
        },
        { kind: "note", text: "Using Next.js or React? Run npx dimmi init and the package sets everything up." },
        { kind: "h2", text: "Check that it works" },
        {
          kind: "p",
          text: "Reload your site. The launcher appears in the corner, and Settings shows the install as live.",
        },
      ],
    },
    {
      slug: "invite-your-team",
      title: "Invite your team",
      summary: "Bring teammates in so they can answer conversations.",
      author: "Jonas Reed",
      updated: "Updated 2 weeks ago",
      body: [
        { kind: "p", text: "Everyone you invite gets their own login and can reply from the shared inbox." },
        {
          kind: "steps",
          items: ["Open Settings, then Members.", "Choose Invite and enter their email.", "Pick a role and send."],
        },
        { kind: "note", text: "Invitations expire after seven days. You can resend them at any time." },
      ],
    },
    {
      slug: "connect-github",
      title: "Connect a GitHub repository",
      summary: "Sync docs and changelog drafts from your repository.",
      author: "Maya Kerr",
      updated: "Updated yesterday",
      body: [
        {
          kind: "p",
          text: "Connecting GitHub lets the agent read your docs and turns merged pull requests into changelog drafts.",
        },
        { kind: "image", text: "The GitHub settings page" },
        { kind: "h2", text: "Before you start" },
        {
          kind: "list",
          items: ["Admin access to the repository", "A fine-grained token with read access to contents"],
        },
        { kind: "h2", text: "Connect" },
        {
          kind: "steps",
          items: [
            "Open Settings, then GitHub.",
            "Paste your token and choose the repository.",
            "Choose Connect. The first sync starts right away.",
          ],
        },
        {
          kind: "note",
          text: "If the sync never finishes, the webhook was not created. Choose Reconnect to create it again.",
        },
      ],
    },
    {
      slug: "change-your-plan",
      title: "Change your plan",
      summary: "Upgrade, downgrade or cancel at any time.",
      author: "Jonas Reed",
      updated: "Updated last month",
      body: [
        { kind: "p", text: "Plan changes take effect right away. Upgrades are prorated for the rest of the month." },
        { kind: "steps", items: ["Open Settings, then Billing.", "Choose Change plan.", "Pick a plan and confirm."] },
      ],
    },
  ],
  popular: ["install-the-widget", "connect-github", "invite-your-team"],
  changelog: [
    {
      tag: "New",
      title: "Shared inboxes are here",
      summary: "Route every conversation to the right team, with assignment rules and saved replies built in.",
      date: "Nov 3",
      body: [
        { kind: "image", text: "Shared inboxes" },
        {
          kind: "p",
          text: "Every team now gets its own inbox. Conversations land in the right place from the first message.",
        },
        { kind: "h2", text: "What's new" },
        {
          kind: "list",
          items: [
            "Assignment rules by page, language or customer plan",
            "Saved replies shared across the team",
            "Collision detection when two people reply at once",
          ],
        },
        { kind: "note", text: "Existing conversations moved to your default inbox. Nothing was lost." },
      ],
    },
    {
      tag: "Improved",
      title: "Faster replies on mobile",
      summary: "The messenger now opens instantly on phones and keeps your place when you switch apps.",
      date: "Oct 21",
      body: [
        {
          kind: "p",
          text: "We rebuilt the messenger's loading path. It now opens in under 100 ms on a mid-range phone.",
        },
        { kind: "list", items: ["Drafts survive switching apps", "Voice notes upload in the background"] },
      ],
    },
  ],
  unread_changelog: 1,
};
