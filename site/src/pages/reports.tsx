import {
  Article,
  ChartBar,
  CheckCircle,
  Gauge,
  Pulse,
  Smiley,
  Sparkle,
  Ticket,
  UsersThree,
} from "@phosphor-icons/react";

import { FeatureGrid, ProductHero, Questions, Rows } from "../components/product-page";
import {
  AiFragment,
  DigestFragment,
  DrillFragment,
  ReportsHeroFragment,
  SatisfactionFragment,
  TeamFragment,
} from "../fragments/reports-fragments";
import { FinalCta } from "../sections/final-cta";

const reports = [
  { icon: ChartBar, title: "Overview", body: "New and closed conversations, the busiest hours, and the backlog." },
  { icon: Gauge, title: "Responsiveness", body: "First response and reply time as median, 90th percentile, and mean." },
  {
    icon: CheckCircle,
    title: "Resolution",
    body: "Time to resolve, first contact resolution, and how often conversations reopen.",
  },
  { icon: UsersThree, title: "Team and teammates", body: "Replies, closes, reply times, and CSAT for each person." },
  { icon: Pulse, title: "Live workload", body: "What's open, unassigned, and waiting right now. It refreshes itself." },
  { icon: Smiley, title: "Satisfaction", body: "CSAT, the spread of ratings, and who earned them." },
  { icon: Sparkle, title: "dimmi AI", body: "How often the agent joins, resolves, and hands off. And why." },
  { icon: Article, title: "Help center", body: "The most read articles, and the searches that found nothing." },
  { icon: Ticket, title: "Tickets and SLAs", body: "Tickets by type, time in each state, and service levels kept." },
];

const rows = [
  {
    label: "AI agent",
    title: "What the agent resolved, and what it didn't.",
    body: "See how often the AI joins a conversation and how often it resolves one. You also get the reasons it handed over and the questions it couldn't answer.",
    visual: <AiFragment />,
  },
  {
    label: "Satisfaction",
    title: "Ask customers how it went.",
    body: "Customers rate a closed conversation in one click, from the messenger or email. CSAT is the share of fours and fives, credited to whoever helped.",
    visual: <SatisfactionFragment />,
  },
  {
    label: "Team",
    title: "Fair numbers for every teammate.",
    body: "Replies, closes, response times, and ratings per person and per team. Count business hours only, and nights and weekends stop skewing the times.",
    visual: <TeamFragment />,
  },
  {
    label: "Drill in",
    title: "Every number opens its conversations.",
    body: "Click a number or a bar to see the conversations behind it. Open any of them in the inbox. There's no number you can't check.",
    visual: <DrillFragment />,
  },
  {
    label: "Digests and exports",
    title: "The numbers come to you.",
    body: "Get the main numbers by email, daily or weekly, set against the period before. Export one row per conversation as a spreadsheet for your own tools.",
    visual: <DigestFragment />,
  },
];

const questions = [
  {
    question: "Can I filter, or is it one big average?",
    answer:
      "Yes, filter away. Pick a date range, then narrow by inbox, team, teammate, channel, tag, priority, and whether the AI was involved. Turn on compare to see the period before.",
  },
  {
    question: "Can I build my own reports?",
    answer:
      "Yes. Start from one of the nine or from a blank page. Pick a number, split it by team, channel, tag, or any attribute, and save it to a dashboard your team shares.",
  },
  {
    question: "Is this a BI tool?",
    answer:
      "No. There's no SQL and no joining against your warehouse. If that's what you need, export the data or read it through the API, and use the BI tool you already have.",
  },
  {
    question: "How do you work out CSAT?",
    answer:
      "Fours and fives. CSAT is the share of ratings that are four or five out of five. Each split shows how many ratings it rests on.",
  },
  {
    question: "Can I get the raw data out?",
    answer:
      "Yes. Export one row per conversation as a CSV, built in the background. Or read every report through the REST API.",
  },
  {
    question: "Can everyone see everyone's numbers?",
    answer: "No. Reports follow your roles. A teammate only sees numbers for the conversations they're allowed to see.",
  },
];

export function ReportsPage() {
  return (
    <>
      <ProductHero
        eyebrow="Reports"
        title="See how support is actually going."
        body="Volume, response times, ratings, and the AI agent's results, by team, channel, and teammate. Click any number to see the conversations behind it."
        visual={<ReportsHeroFragment />}
      />
      <FeatureGrid
        title="Nine reports before you build your own."
        body="They're there from day one, each with its own numbers and charts."
        items={reports}
      />
      <Rows title="Numbers you can check." rows={rows} />
      <Questions body="The fine print on how the numbers work." items={questions} />
      <FinalCta />
    </>
  );
}
