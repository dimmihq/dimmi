import { ChatsCircle, ShieldCheck, Smiley, Sparkle, Ticket, Timer } from "@phosphor-icons/react";

import { FeatureGrid, ProductHero, Questions, Rows } from "../../components/product-page";
import { ShotStill } from "../../components/shot-still";
import { ShotVideo } from "../../components/shot-video";
import { FinalCta } from "../../sections/final-cta";

const rows = [
  {
    label: "Room modes",
    title: "Track customer requests while internal chatter stays out of your queue.",
    body: "Pick Shared with a customer, Community, Help desk, or Team only on every channel you invite dimmi into. Channel triggers keep casual thanks out of your queue.",
    visual: (
      <ShotStill
        name="channel-slack-rooms"
        alt="Slack channel settings in dimmi showing room mode choices, conversation triggers, and noise filters"
      />
    ),
  },
  {
    label: "Two-way sync",
    title: "Answer from dimmi or Slack, with SLAs and tickets attached.",
    body: "Replies sync both ways in real time, keeping SLAs, owners, and linked tickets tied to the thread. Leave private notes under triage cards without leaving Slack.",
    visual: (
      <ShotVideo
        name="channel-slack-sync"
        poster="channel-slack-sync"
        alt="Two-way sync between a Slack Connect thread and the dimmi inbox showing live replies, SLAs, assignment, and internal notes"
        maxWidth={520}
      />
    ),
  },
  {
    label: "Agents and Operator",
    title: "Put support agents in customer threads and Operator in your team rooms.",
    body: "Support agents answer in customer threads or draft replies for review, and step back when a teammate joins. Ask Operator questions or approve routine updates inside team rooms.",
    visual: (
      <ShotStill
        name="channel-slack-operator"
        alt="A dimmi support agent drafting a Slack reply alongside an Operator routine approval card in a team channel"
      />
    ),
  },
  {
    label: "Routing and privacy",
    title: "Route channels to the right inbox, and erase Slack data on disconnect.",
    body: "Route each Slack channel to its own inbox and alert channel. Disconnect the app and dimmi revokes tokens at once, then erases Slack messages and files after 10 days.",
    visual: (
      <ShotStill
        name="channel-slack-alerts"
        alt="Slack channel routing settings showing per-channel inbox selection, triage alert channels, and uninstall data deletion rules"
      />
    ),
  },
];

const more = [
  {
    icon: Smiley,
    title: "Emoji triage shortcuts",
    body: "React with eyes to claim a thread, a checkmark to close it, or a siren to mark it urgent.",
  },
  {
    icon: ChatsCircle,
    title: "Triage channels with private notes",
    body: "Discuss a tough question under its triage card where customers cannot read it, or start with >> to send a public answer.",
  },
  {
    icon: Sparkle,
    title: "Dedicated agents per channel",
    body: "Place a billing or technical support agent on one Slack channel, and choose direct replies or drafts for teammate review.",
  },
  {
    icon: Timer,
    title: "Agents step back for humans",
    body: "Add a wait delay before the agent answers, and watch it step aside the second a teammate replies or gets tagged.",
  },
  {
    icon: Ticket,
    title: "Internal help desk mode",
    body: "Turn an internal channel into an IT or ops desk where colleagues track their own requests in Slack's Home tab.",
  },
  {
    icon: ShieldCheck,
    title: "Company swap protection",
    body: "Change the external company in a shared channel and dimmi unlinks the old account so zero history leaks across orgs.",
  },
];

const questions = [
  {
    question: "Will dimmi read every channel in our Slack workspace?",
    answer:
      "dimmi only watches channels you invite the app into. Private rooms, DMs, and every other channel stay off limits unless you add dimmi there yourself.",
  },
  {
    question: "How does dimmi match Slack replies to the right teammate?",
    answer:
      "When a teammate's Slack email matches their dimmi account, their replies in Slack sync under their name and permissions right away. Anyone on your team can jump into a thread from Slack or the inbox.",
  },
  {
    question: "Will the AI support agent talk over a teammate who is already helping?",
    answer:
      "The agent stops the moment a teammate replies in the thread or a customer mentions someone on your team. You can also set a wait delay or switch a channel to Draft for review.",
  },
  {
    question: "Can customers ever see what we ask Operator in Slack?",
    answer:
      "Operator only answers linked teammates inside Team only channels and direct messages. It stays silent in every room customers can read, and routine approvals wait for a teammate's click.",
  },
  {
    question: "What happens to our Slack messages if we uninstall dimmi?",
    answer:
      "dimmi wipes stored Slack tokens right away when you disconnect and stops watching every channel. Your history stays intact for 10 days in case you reconnect, then dimmi deletes Slack messages, files, and IDs for good.",
  },
  {
    question: "Can we connect several Slack workspaces to one dimmi workspace?",
    answer:
      "One dimmi workspace connects to one Slack workspace. If you need one queue to bridge multiple Slack workspaces at once, or only want a lightweight Slack emoji widget without an inbox, another tool will fit better.",
  },
];

export function SlackChannelPage() {
  return (
    <>
      <ProductHero
        eyebrow="Slack"
        title="Support customers in Slack without losing a single thread."
        body="Connect customer Slack Connect channels, communities, and internal help desks to dimmi. Real requests sync both ways with SLAs, tickets, support agents, and Operator."
        visual={
          <ShotVideo
            name="channel-slack-hero"
            poster="channel-slack-hero"
            alt="A customer message in a Slack Connect channel opening a tracked conversation in the dimmi inbox with live two-way replies"
            bleed
          />
        }
      />
      <Rows title="Give customers the speed of Slack and your team a real inbox." rows={rows} />
      <FeatureGrid title="Built for teams who live in Slack all day." items={more} />
      <Questions body="What teams ask before connecting Slack Connect and internal channels." items={questions} />
      <FinalCta />
    </>
  );
}
