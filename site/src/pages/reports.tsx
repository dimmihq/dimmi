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
  { icon: ChartBar, title: "Overview", body: "New and closed chats, busiest hours, and your open backlog." },
  { icon: Gauge, title: "Speed", body: "First reply and next reply times as median, p90, and average." },
  {
    icon: CheckCircle,
    title: "Resolution",
    body: "Time to resolve, solves on the first reply, and how often chats reopen.",
  },
  { icon: UsersThree, title: "Team and teammates", body: "Replies, closes, response times, and CSAT for each person." },
  { icon: Pulse, title: "Live queue", body: "See what is open, unassigned, and waiting right now in real time." },
  { icon: Smiley, title: "Satisfaction", body: "CSAT scores, rating spread, and who earned every review." },
  { icon: Sparkle, title: "AI agent", body: "How often the agent replies, solves, and hands off, plus why." },
  { icon: Article, title: "Help center", body: "Top articles and searches that returned zero results." },
  { icon: Ticket, title: "Tickets and SLAs", body: "Tickets by type, time in each state, and SLA targets met." },
];

const rows = [
  {
    label: "AI agent",
    title: "See what AI solved and where it handed off.",
    body: "Track how often the agent joins and resolves chats, plus the exact questions it could not answer so you know which help article to write next.",
    visual: <AiFragment />,
  },
  {
    label: "Satisfaction",
    title: "Collect CSAT right when a chat closes.",
    body: "Customers rate support in one click inside the messenger or email. CSAT counts fours and fives out of five, credited to whoever helped.",
    visual: <SatisfactionFragment />,
  },
  {
    label: "Team",
    title: "Fair numbers that respect office hours.",
    body: "Track replies, closes, speed, and CSAT per teammate or team. Count office hours only so nights and weekends never skew response times.",
    visual: <TeamFragment />,
  },
  {
    label: "Drill in",
    title: "Every chart opens the real chats behind it.",
    body: "Click any number or bar to see the exact conversations inside it, and open any thread in your inbox to check what happened.",
    visual: <DrillFragment />,
  },
  {
    label: "Digests and exports",
    title: "Send summaries to your inbox, or export raw data.",
    body: "Get daily or weekly email digests compared against the prior period, or export one CSV row per conversation for your own tools.",
    visual: <DigestFragment />,
  },
];

const questions = [
  {
    question: "Can I slice reports by team, channel, or tag?",
    answer:
      "Yes. Filter any chart by date range, inbox, team, teammate, channel, tag, priority, or AI involvement, and compare against the previous period.",
  },
  {
    question: "Can we build custom dashboards?",
    answer:
      "Start from any standard report or a blank page. Pick a metric, split it by team, tag, or customer field, and save it to a shared dashboard.",
  },
  {
    question: "Is this meant to replace a BI tool?",
    answer:
      "Keep your BI tool for SQL queries and data warehouse joins. dimmi reports answer support questions fast; export CSVs or call the REST API when you want warehouse joins.",
  },
  {
    question: "How does dimmi calculate CSAT?",
    answer:
      "CSAT is the share of 4-star and 5-star ratings out of 5. Every chart shows the exact rating count behind the score so small samples never mislead you.",
  },
  {
    question: "Can I pull report data into our own tools?",
    answer:
      "Yes. Export one CSV row per conversation in the background, or query every report through the REST API.",
  },
  {
    question: "Can everyone see everyone else's metrics?",
    answer:
      "Reports follow your workspace roles. Each teammate only sees numbers for the inboxes and conversations they are allowed to access.",
  },
];

export function ReportsPage() {
  return (
    <>
      <ProductHero
        eyebrow="Reports"
        title="Support numbers you can click through and check."
        body="Track volume, reply times, CSAT, and AI results by team or channel. Click any bar to see the real chats behind it."
        visual={<ReportsHeroFragment />}
      />
      <FeatureGrid
        title="Ready from day one, custom when you need more."
        body="Open every core support report out of the box, or build shared dashboards for your team."
        items={reports}
      />
      <Rows title="Every number backed by real chats." rows={rows} />
      <Questions body="How the numbers work under the hood." items={questions} />
      <FinalCta />
    </>
  );
}
