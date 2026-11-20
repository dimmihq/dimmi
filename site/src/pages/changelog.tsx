import {
  CalendarCheck,
  ChartBar,
  ChatCircleText,
  Code,
  ImageSquare,
  MarkdownLogo,
  RssSimple,
  Tag,
  UsersThree,
} from "@phosphor-icons/react";

import { FeatureGrid, ProductHero, Questions, Rows, Steps } from "../components/product-page";
import {
  ChangelogPageFragment,
  EmailFragment,
  EntryFooterFragment,
  EntryFragment,
  MessengerChangelogFragment,
  PublishFragment,
} from "../fragments/changelog-fragments";
import { FinalCta } from "../sections/final-cta";

const steps = [
  {
    title: "Write the update",
    body: "The same editor as your articles, with labels, a version, a cover, and everyone who worked on it.",
    visual: <EntryFragment />,
  },
  {
    title: "Publish now, or later",
    body: "Pick a time and it goes live on its own. Change your mind and it goes back to a draft.",
    visual: <PublishFragment />,
  },
  {
    title: "Customers hear about it",
    body: "On your changelog page, in the messenger's changelog tab, and in subscribers' inboxes.",
    visual: <MessengerChangelogFragment compact />,
  },
];

const rows = [
  {
    label: "Messenger",
    title: "News where customers already are.",
    body: "The messenger gets a changelog tab. A dot marks what a customer hasn't seen yet.",
    visual: <MessengerChangelogFragment />,
  },
  {
    label: "Email",
    title: "One email when you ship.",
    body: "Readers subscribe from the changelog and confirm by email. Each update can email them once, in your brand.",
    visual: <EmailFragment />,
  },
  {
    label: "Feedback",
    title: "Find out what they think.",
    body: "Readers react with an emoji, or ask your team about it in the messenger, straight from the update.",
    visual: <EntryFooterFragment />,
  },
];

const more = [
  { icon: Tag, title: "Labels", body: "Starts with New, Improved, and Fixed. Add your own, with a color each." },
  { icon: CalendarCheck, title: "Scheduling", body: "Write ahead and pick a time. It publishes on its own." },
  { icon: ImageSquare, title: "Covers", body: "A picture at the top of an update, on the page and in the email." },
  { icon: UsersThree, title: "Authors", body: "Everyone who worked on an update is named on it." },
  {
    icon: ChartBar,
    title: "Release strip",
    body: "A year of releases at a glance, one mark for each week you shipped.",
  },
  { icon: RssSimple, title: "RSS", body: "A feed for every label, for readers who'd rather not get email." },
  {
    icon: ChatCircleText,
    title: "Questions, in place",
    body: "A button on every update opens the messenger to ask about it.",
  },
  { icon: MarkdownLogo, title: "Markdown copies", body: "Every update has a plain Markdown version for AI tools." },
  { icon: Code, title: "API and MCP", body: "Write, publish, and list updates from your own tools or an AI client." },
];

const questions = [
  {
    question: "Is this one more subdomain to set up?",
    answer:
      "No. It lives on your help center, under /changelog, with the same header, brand, and domain. It also shows in the messenger's changelog tab.",
  },
  {
    question: "How do people subscribe, and can they leave?",
    answer:
      "By email, and yes. They enter it on the changelog page and confirm with a link. Every email has a link to unsubscribe. There's an RSS feed too.",
  },
  {
    question: "Do subscribers get emailed again when I fix a typo?",
    answer:
      "No. Edits wait as unpublished changes until you publish them, and subscribers are never emailed about the same update twice.",
  },
  {
    question: "Can I run the changelog without a help center?",
    answer:
      "No. The changelog is part of a help center and uses its address, header, and brand. If release notes are all you want, you still set up a help center first.",
  },
  {
    question: "Can our release script publish updates?",
    answer:
      "Yes. The REST API and the MCP server can write, label, and publish updates, so a script or an AI client can draft them for you.",
  },
];

export function ChangelogPage() {
  return (
    <>
      <ProductHero
        eyebrow="Changelog"
        title="Tell customers what you shipped."
        body="Publish updates on a public changelog, in the messenger, and in their inbox. Then find out what they think."
        visual={<ChangelogPageFragment />}
      />
      <Steps title="One update, three places." items={steps} />
      <Rows title="Updates people actually see." rows={rows} />
      <FeatureGrid title="The small stuff, done right." items={more} />
      <Questions body="What people ask before they publish their first update." items={questions} />
      <FinalCta />
    </>
  );
}
