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
    body: "Use the same clean editor as your help center, with labels, versions, covers, and authors.",
    visual: <EntryFragment />,
  },
  {
    title: "Publish or schedule",
    body: "Ship right away or pick a launch time. Pull it back to draft anytime before it goes live.",
    visual: <PublishFragment />,
  },
  {
    title: "Reach customers everywhere",
    body: "Updates appear on your changelog page, inside the messenger tab, and in subscriber inboxes.",
    visual: <MessengerChangelogFragment compact />,
  },
];

const rows = [
  {
    label: "Messenger",
    title: "News right inside your app.",
    body: "Add a changelog tab to the messenger with an unread dot so customers spot new releases without leaving your product.",
    visual: <MessengerChangelogFragment />,
  },
  {
    label: "Email",
    title: "One clean email when you ship.",
    body: "Readers subscribe from your changelog page and confirm by email. Each release emails subscribers once in your brand.",
    visual: <EmailFragment />,
  },
  {
    label: "Feedback",
    title: "Hear what customers think right away.",
    body: "Readers leave quick reactions or open the messenger from any update to ask your team a question.",
    visual: <EntryFooterFragment />,
  },
];

const more = [
  { icon: Tag, title: "Custom labels", body: "Starts with New, Improved, and Fixed. Add your own labels and colors." },
  { icon: CalendarCheck, title: "Scheduled launches", body: "Write release notes ahead of time and let them go live on cue." },
  { icon: ImageSquare, title: "Cover images", body: "Add a crisp screenshot or header image on the page and in email." },
  { icon: UsersThree, title: "Named authors", body: "Credit everyone on the team who helped ship the release." },
  {
    icon: ChartBar,
    title: "Release strip",
    body: "Show a full year of shipping momentum, week by week, at the top.",
  },
  { icon: RssSimple, title: "RSS feeds", body: "Every label gets an RSS feed for readers who skip email." },
  {
    icon: ChatCircleText,
    title: "Ask about an update",
    body: "One button on every post opens the messenger with context attached.",
  },
  { icon: MarkdownLogo, title: "Markdown copies", body: "Every release has a plain Markdown URL ready for AI tools." },
  { icon: Code, title: "API and MCP", body: "Draft, label, and publish updates from CI scripts or coding agents." },
];

const questions = [
  {
    question: "Do I have to set up another subdomain?",
    answer:
      "Not at all. Your changelog lives right on your help center at /changelog, sharing the same domain, header, and brand.",
  },
  {
    question: "How do people subscribe and unsubscribe?",
    answer:
      "They enter their email on the changelog page and confirm via a link. Every email includes a one-click unsubscribe link, and RSS feeds are built in.",
  },
  {
    question: "Will fixing a typo email subscribers a second time?",
    answer:
      "Never. Edits save as a draft until you publish them, and subscribers only get emailed once per release.",
  },
  {
    question: "Can I use the changelog without a help center?",
    answer:
      "The changelog lives inside a help center and shares its domain and header. If you only want release notes, you still turn on a help center first.",
  },
  {
    question: "Can our deploy pipeline draft changelog posts?",
    answer:
      "Yes. Both the REST API and MCP server can create, label, and publish entries straight from your release scripts or AI client.",
  },
];

export function ChangelogPage() {
  return (
    <>
      <ProductHero
        eyebrow="Changelog"
        title="Shipped something good? Make sure customers see it."
        body="Publish updates once and reach people on your changelog, inside the messenger, and in their inbox."
        visual={<ChangelogPageFragment />}
      />
      <Steps title="Write once, reach customers in three places." items={steps} />
      <Rows title="Updates people read instead of missing." rows={rows} />
      <FeatureGrid title="All the details you expect from a changelog." items={more} />
      <Questions body="What teams ask before publishing their first update." items={questions} />
      <FinalCta />
    </>
  );
}
