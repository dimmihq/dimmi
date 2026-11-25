import { ChatsCircle, ShieldCheck, Smiley, Sparkle, Ticket, Timer } from "@phosphor-icons/react";

import { ChannelLinks, FeatureGrid, ProductHero, Questions, Rows } from "../../components/product-page";
import { ShotStill } from "../../components/shot-still";
import { ShotVideo } from "../../components/shot-video";
import { FinalCta } from "../../sections/final-cta";

const rows = [
  {
    label: "Channel modes",
    title: "Pick which channels open support threads.",
    body: "Set each Slack channel to Customer, Community, Help desk, or Team only. Everyday chatter and quick thanks stay out of your inbox.",
    visual: (
      <ShotStill
        name="channel-slack-rooms"
        alt="Slack channel settings in dimmi showing room mode choices, conversation triggers, and noise filters"
      />
    ),
  },
  {
    label: "Two-way sync",
    title: "Reply from Slack or dimmi and both stay in sync.",
    body: "Messages sync both ways in real time, with owners, reply timers, and linked tickets attached. Discuss tough questions privately under triage cards inside Slack.",
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
    title: "Let AI agents draft replies and Operator help your team.",
    body: "AI agents can answer customer threads or write drafts for you to check, then step back when a teammate joins. Ask Operator questions or approve updates in team rooms.",
    visual: (
      <ShotStill
        name="channel-slack-operator"
        alt="A dimmi support agent drafting a Slack reply alongside an Operator routine approval card in a team channel"
      />
    ),
  },
  {
    label: "Routing and privacy",
    title: "Send each Slack channel to the right team.",
    body: "Route every Slack channel to its own inbox and alert room. If you ever disconnect Slack, dimmi removes access at once and deletes Slack messages after 10 days.",
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
    title: "Emoji shortcuts",
    body: "React with eyes to claim a thread, a checkmark to close it, or a siren to mark it urgent.",
  },
  {
    icon: ChatsCircle,
    title: "Private team triage",
    body: "Talk through a question under its Slack alert card where customers cannot see it, or start with >> to reply.",
  },
  {
    icon: Sparkle,
    title: "One AI agent per channel",
    body: "Put a billing or technical agent in a specific Slack channel, set to reply directly or draft for review.",
  },
  {
    icon: Timer,
    title: "Agents step back for people",
    body: "Set a short wait time before the AI answers, and watch it stop the second a teammate replies.",
  },
  {
    icon: Ticket,
    title: "Internal IT help desk",
    body: "Turn a team channel into an internal help desk where coworkers track their requests in Slack Home.",
  },
  {
    icon: ShieldCheck,
    title: "Safe company switching",
    body: "Swap the customer linked to a shared channel and dimmi separates old history so nothing leaks.",
  },
];

const questions = [
  {
    question: "Can dimmi read every channel in our Slack workspace?",
    answer:
      "No. dimmi only sees channels where you invite the app. Private channels and DMs stay off limits unless you add dimmi yourself.",
  },
  {
    question: "How does dimmi know which teammate replied in Slack?",
    answer:
      "When a teammate's Slack email matches their dimmi account, their Slack replies show up under their name automatically.",
  },
  {
    question: "Will the AI agent talk over a teammate?",
    answer:
      "Never. The agent stops as soon as a teammate replies or gets mentioned. You can also switch any channel to Draft mode so a person approves replies first.",
  },
  {
    question: "Can customers see what we ask Operator in Slack?",
    answer:
      "No. Operator only replies to your team in Team only channels and DMs. It stays quiet in every room where customers are present.",
  },
  {
    question: "What happens if we uninstall the Slack app?",
    answer:
      "dimmi removes your Slack access token right away and stops reading messages. History stays for 10 days in case you reconnect, then Slack messages and files are deleted.",
  },
  {
    question: "Can we connect more than one Slack workspace?",
    answer:
      "One dimmi workspace connects to one Slack workspace. If you need to combine several Slack workspaces into one inbox, another tool will fit better.",
  },
];

export function SlackChannelPage() {
  return (
    <>
      <ProductHero
        eyebrow="Slack"
        breadcrumb={[
          { label: "Home", href: "/" },
          { label: "Channels", href: "/channels" },
        ]}
        title="Answer customers in Slack without losing messages."
        body="Connect customer Slack channels and internal help channels to dimmi. Every question lands in your inbox with an owner, a reply timer, and live two-way sync."
        visual={
          <ShotVideo
            name="channel-slack-hero"
            poster="channel-slack-hero"
            alt="A customer message in a Slack Connect channel on the left synced live with the dimmi inbox on the right"
            bleed
          />
        }
      />
      <Rows title="Fast for customers in Slack. Organized for your team." rows={rows} />
      <FeatureGrid title="Made for teams that work in Slack all day." items={more} />
      <ChannelLinks current="/channels/slack" />
      <Questions body="What teams ask before connecting Slack to dimmi." items={questions} />
      <FinalCta />
    </>
  );
}
