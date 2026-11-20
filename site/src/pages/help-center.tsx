import {
  ChartLineUp,
  ClockCounterClockwise,
  DownloadSimple,
  Globe,
  LockKey,
  MarkdownLogo,
  PaintBrush,
  Smiley,
  Translate,
} from "@phosphor-icons/react";

import { FeatureGrid, HowItWorks, ProductHero, Questions, Steps } from "../components/product-page";
import {
  AskFlowFragment,
  CollectionsFragment,
  DomainFragment,
  HelpHomeFragment,
  WriterFragment,
} from "../fragments/help-center-fragments";
import { FinalCta } from "../sections/final-cta";

const steps = [
  {
    title: "Write in a plain editor",
    body: "Type a slash for headings, callouts, images, and tables. Markdown works too, if that's your habit.",
    visual: <WriterFragment />,
  },
  {
    title: "Sort into collections",
    body: "Nest collections as deep as you like. Readers open them in place, without leaving the page.",
    visual: <CollectionsFragment />,
  },
  {
    title: "Publish on your domain",
    body: "Add two DNS records. dimmi checks them and issues the certificate for you.",
    visual: <DomainFragment />,
  },
];

const how = [
  {
    title: "Readers ask in one box",
    body: "With the agent on, the box asks the agent. With it off, the box searches your articles.",
  },
  {
    title: "The agent answers from your articles",
    body: "Every answer links the article it came from. It never uses an article that reader can't see.",
  },
  {
    title: "Your team picks up the same thread",
    body: "When a person should take over, the chat lands in your inbox with a summary.",
  },
];

const more = [
  { icon: Globe, title: "Your own domain", body: "help.yourcompany.com. We handle the certificate." },
  { icon: PaintBrush, title: "Your brand", body: "Your logo, color, and greeting, in light and dark." },
  {
    icon: LockKey,
    title: "Private articles",
    body: "Keep an article to signed-in readers in one audience, or to the agent alone.",
  },
  {
    icon: DownloadSimple,
    title: "Bring your articles",
    body: "Import from Zendesk, Intercom, Notion, or a website. Old links redirect to the new ones.",
  },
  {
    icon: Smiley,
    title: "Reader feedback",
    body: "Readers say if an article helped. A no asks what was missing.",
  },
  {
    icon: ChartLineUp,
    title: "Analytics",
    body: "Views by day, the most read articles, and the searches that found nothing.",
  },
  {
    icon: ClockCounterClockwise,
    title: "Version history",
    body: "Every published version is kept. Restore one in a click.",
  },
  {
    icon: MarkdownLogo,
    title: "Ready for AI tools",
    body: "Every page has a Markdown copy, and a menu to open it in ChatGPT or Claude.",
  },
  {
    icon: Translate,
    title: "Every language you support",
    body: "One article, many languages. dimmi drafts the translation and flags it when the original changes.",
  },
];

const questions = [
  {
    question: "Can it run on our domain, or are we stuck on yours?",
    answer:
      "Yours. Add a CNAME and a TXT record. Once dimmi verifies them, the help center answers there with its own certificate. The old address keeps working.",
  },
  {
    question: "Does the AI agent actually read our articles?",
    answer:
      "Yes. It answers from your articles and links the one it used. Articles kept to an audience are only used for readers in that audience.",
  },
  {
    question: "Do we have to retype everything from our old tool?",
    answer:
      "No. Connect Zendesk, Intercom, Notion, or a website, then turn what it synced into articles, filed in the same collections. The old addresses redirect.",
  },
  {
    question: "Can one article exist in several languages?",
    answer:
      "Yes. Add a language and each article gets a translation beside the original. dimmi drafts it, a teammate checks it, and readers get their own language. When the original changes, the translation is flagged as stale.",
  },
  {
    question: "Can I host our API docs on it?",
    answer:
      "No. There's no API reference generator and no versioned docs. It's a help center for customers. Developer docs deserve a docs tool.",
  },
  {
    question: "Can we run more than one?",
    answer: "Yes. Run one per product or brand, each with its own address, articles, and look.",
  },
  {
    question: "Will Google index it?",
    answer:
      "Yes, unless you turn indexing off. Pages are plain, fast HTML with a canonical address on your own domain.",
  },
];

export function HelpCenterPage() {
  return (
    <>
      <ProductHero
        eyebrow="Help center"
        title="Answers customers find without you."
        body="Write articles, publish them on your own domain, and let the AI agent answer from them. Your team gets what the articles can't cover."
        visual={<HelpHomeFragment />}
      />
      <Steps title="Write it, sort it, publish it." items={steps} />
      <HowItWorks
        label="Ask, don't search"
        title="One box that answers."
        body="Readers type a question instead of guessing your keywords. The agent replies from your articles. A person is one step away."
        steps={how}
        visual={(step) => <AskFlowFragment step={step} />}
      />
      <FeatureGrid
        title="The rest of the help center."
        body="The parts that make it yours, and the ones that tell you what to write next."
        items={more}
      />
      <Questions body="What people ask before they publish their first article." items={questions} />
      <FinalCta />
    </>
  );
}
