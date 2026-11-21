import { ProductHero, Questions, Rows, Steps } from "../components/product-page";
import { ShotStill } from "../components/shot-still";
import { ExchangeFragment, KnowledgeFragment, ModelFragment, TestFragment } from "../fragments/agent-fragments";
import { AgentDemoCard, AgentStill } from "../scenes/agent-demo";
import { useAgentStories } from "../scenes/use-agent-stories";
import { FinalCta } from "../sections/final-cta";

const steps = [
  {
    title: "Already in your inbox",
    body: "Turn it on per channel across chat, email, Slack, and API threads. No second tool to check.",
    visual: <ExchangeFragment />,
  },
  {
    title: "Learns from your docs",
    body: "Connect your help center, site, files, Notion, or GitHub. Every answer links to the page it used.",
    visual: <KnowledgeFragment />,
  },
  {
    title: "Test on past chats first",
    body: "Run safe tests on real threads and check every source and reply before going live.",
    visual: <TestFragment />,
  },
];

const rows = [
  {
    label: "Knowledge",
    title: "Every answer links its source.",
    body: "Replies link to the exact help article or doc the agent read. When your docs do not cover a question, it hands over instead of guessing.",
    visual: <AgentStill story={0} />,
  },
  {
    label: "Procedures",
    title: "Follows your steps, asks on risky ones.",
    body: "Write playbooks like refunding a duplicate charge in plain English. The agent runs your steps and drops an approval card before anything you cannot undo.",
    visual: <ShotStill name="procedure" alt="A procedure in the editor: refunding a duplicate charge, step by step" />,
  },
  {
    label: "Handover",
    title: "Steps aside with a quick summary.",
    body: "When a customer asks for a human or hits a rule you set, the agent routes the thread with a private note and stays quiet.",
    visual: <AgentStill story={2} />,
  },
  {
    label: "Voice & safety",
    title: "Sounds like your team, follows your rules.",
    body: "Pick the agent's name, tone, and rules in plain English. dimmi blocks unknown links, images, and raw HTML before any reply reaches a customer.",
    visual: <ShotStill name="agent-voice" alt="The agent's name, tone and guidance in settings" />,
  },
  {
    label: "Your model keys",
    title: "Use your own keys, save up to 83%.",
    body: "Plug in your OpenAI or Anthropic keys and pay your provider yourself. AI threads drop to the human rate, cutting dimmi fees by 50% to 83%.",
    visual: <ModelFragment />,
  },
];

const questions = [
  {
    question: "Where can the agent reply?",
    answer:
      "Across every channel in your inbox: messenger, email, Slack, Teams, Discord, WhatsApp, SMS, phone, and API. You pick which channels and hours it covers.",
  },
  {
    question: "Does it read the whole thread?",
    answer:
      "Yes. It reads the full chat and customer details before it replies. Internal team notes stay hidden from the model.",
  },
  {
    question: "What if a teammate jumps in?",
    answer:
      "By default, the agent steps back while an assigned teammate is online or replied in the last 24 hours. You can also set it to stop once anyone takes the thread.",
  },
  {
    question: "Can it answer the phone?",
    answer:
      "Yes. It picks up using the same docs and playbooks, then transfers the call with a summary when a human should take over.",
  },
  {
    question: "Does it speak my customers' languages?",
    answer:
      "Out of the box. Write your help center in your language and the agent replies in whatever language the customer used.",
  },
  {
    question: "Will it ever make things up?",
    answer:
      "Any AI model can miss an edge case, and vendors claiming 100% accuracy are lying. That is why dimmi requires real sources, blocks unknown links, pauses risky actions for approval, and gives you dry runs. Keep humans on topics where one mistake is too costly.",
  },
  {
    question: "Can I test it without messaging real customers?",
    answer:
      "Yes. Run test chats inside dimmi using your real docs and playbooks. You see the exact draft reply and sources while zero messages go out.",
  },
  {
    question: "Do you charge $0.99 per AI resolution like Intercom?",
    answer:
      "Never. AI is included in our conversation rates (10¢ down to 6¢ as you grow), with 100 free threads each month. Add your own OpenAI or Anthropic keys and AI drops to the human rate, from 5¢ down to 1¢.",
  },
];

function HeroDemo() {
  const stories = useAgentStories();
  return <AgentDemoCard stories={stories} className="max-w-[640px]" />;
}

export function AgentPage() {
  return (
    <>
      <ProductHero
        eyebrow="AI agent"
        title="An AI agent that cites its sources and knows when to stop."
        body="It works in your shared inbox, follows your written steps, and asks before risky actions. Bring your own AI keys and save up to 83%."
        visual={<HeroDemo />}
      />
      <Steps title="Go live without risking a single chat." items={steps} />
      <Rows title="Built for teams who want control over AI." rows={rows} />
      <Questions body="What smart teams ask before turning an AI agent on." items={questions} />
      <FinalCta />
    </>
  );
}
