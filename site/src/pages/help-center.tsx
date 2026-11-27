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
import { ThemeShowcase } from "../sections/theme-showcase";

const steps = [
  {
    title: "Write in a clean editor",
    body: "Use slash commands for headings, callouts, images, and tables, or paste Markdown straight in.",
    visual: <WriterFragment />,
  },
  {
    title: "Group into collections",
    body: "Nest collections as deep as you need. Readers browse in place without losing their spot.",
    visual: <CollectionsFragment />,
  },
  {
    title: "Publish on your domain",
    body: "Add two DNS records. dimmi checks them and sets up HTTPS for you.",
    visual: <DomainFragment />,
  },
];

const how = [
  {
    title: "Readers ask in one box",
    body: "With the AI agent on, the box answers their question. With it off, it searches your articles.",
  },
  {
    title: "Answers come from your docs",
    body: "Every reply links the exact article it used and respects private audience rules.",
  },
  {
    title: "Your team picks up the chat",
    body: "When a person should help, the chat opens in your inbox with a quick summary.",
  },
];

const more = [
  { icon: Globe, title: "Your own domain", body: "Host at help.yourcompany.com with automatic HTTPS." },
  {
    icon: PaintBrush,
    title: "Your brand, or a whole new look",
    body: "Set your logo, colors, and greeting, or redesign every page to match your own site.",
  },
  {
    icon: LockKey,
    title: "Private articles",
    body: "Keep private articles visible only to chosen audiences or the AI agent.",
  },
  {
    icon: DownloadSimple,
    title: "Import from old tools",
    body: "Bring articles over from Zendesk, Intercom, Notion, or your site with automatic redirects.",
  },
  {
    icon: Smiley,
    title: "Article feedback",
    body: "Readers vote if an article helped, and tell you what was missing when it did not.",
  },
  {
    icon: ChartLineUp,
    title: "Search insights",
    body: "See daily views, top articles, and the questions people searched that found nothing.",
  },
  {
    icon: ClockCounterClockwise,
    title: "Version history",
    body: "Every published version is saved so you can roll back in one click.",
  },
  {
    icon: MarkdownLogo,
    title: "Ready for AI tools",
    body: "Every article has a clean Markdown URL plus a menu to open it in ChatGPT or Claude.",
  },
  {
    icon: Translate,
    title: "Multi-language docs",
    body: "Draft translations beside each article and get flagged whenever the original changes.",
  },
];

const questions = [
  {
    question: "Can it run on our own domain?",
    answer:
      "Yes. Point a CNAME and TXT record at dimmi and we handle the SSL certificate on your domain. Your old help center address keeps working too.",
  },
  {
    question: "Does the AI agent read our help center automatically?",
    answer:
      "Every published article feeds the agent and gets cited in its replies. Restricted articles are only used for readers who belong to that audience.",
  },
  {
    question: "Do we have to copy and paste everything from Zendesk or Intercom?",
    answer:
      "Never. Connect Zendesk, Intercom, Notion, or an existing website and import articles into matching collections, with redirects from the old URLs.",
  },
  {
    question: "How do translations work when an article changes?",
    answer:
      "Add your languages and dimmi drafts translations beside the original for a teammate to review. Whenever you edit the main article, translations get flagged as stale.",
  },
  {
    question: "Can we change more than the colors?",
    answer:
      "All of it. Every page, from articles to feature requests, can get a completely new design. You see it with your real articles before customers do.",
  },
  {
    question: "Can I host our developer API docs here?",
    answer:
      "Use a dedicated docs tool for that. dimmi is built for customer help centers, not OpenAPI reference generators or versioned developer docs.",
  },
  {
    question: "Can we run separate help centers for different brands?",
    answer: "Yes. Create one help center per product or brand, each with its own custom domain, articles, and styling.",
  },
  {
    question: "Will Google index our help articles?",
    answer: "Yes, unless you switch indexing off. Pages render as fast HTML with canonical links on your own domain.",
  },
];

export function HelpCenterPage() {
  return (
    <>
      <ProductHero
        eyebrow="Help center"
        title="One help center for customers and your AI agent."
        body="Publish articles on your own domain. Customers find answers fast, and your AI agent reads from the exact same docs."
        visual={<HelpHomeFragment />}
      />
      <Steps title="From draft to live on your domain." items={steps} />
      <HowItWorks
        label="Ask, not search"
        title="One box that answers real questions."
        body="Readers ask what they need instead of guessing your keywords. A teammate is always one click away."
        steps={how}
        visual={(step) => <AskFlowFragment step={step} />}
      />
      <ThemeShowcase />
      <FeatureGrid
        title="Everything you need to run docs customers read."
        body="Match your brand, import old articles, and see which questions still need answers."
        items={more}
      />
      <Questions body="What teams ask before publishing their first article." items={questions} />
      <FinalCta />
    </>
  );
}
