import { ProductHero, Questions, Rows, Steps } from "../components/product-page";
import { ShotStill } from "../components/shot-still";
import { ExchangeFragment, KnowledgeFragment, ModelFragment, TestFragment } from "../fragments/agent-fragments";
import { AgentDemoCard, AgentStill } from "../scenes/agent-demo";
import { useAgentStories } from "../scenes/use-agent-stories";
import { FinalCta } from "../sections/final-cta";

const steps = [
  {
    title: "Already in your inbox",
    body: "The agent works the same channels and threads your team does. No second tool to check.",
    visual: <ExchangeFragment />,
  },
  {
    title: "Add your docs",
    body: "Point it at your help center, website, and files. Every answer cites where it came from.",
    visual: <KnowledgeFragment />,
  },
  {
    title: "Test, then go live",
    body: "Run it on real conversations and read its reasoning. Nothing gets sent until you switch it on.",
    visual: <TestFragment />,
  },
];

const rows = [
  {
    label: "Knowledge",
    title: "Every answer shows its work.",
    body: "Replies link to the article they came from. Check an answer before it reaches a customer, or after.",
    visual: <AgentStill story={0} />,
  },
  {
    label: "Procedures",
    title: "It follows your steps and asks before the risky ones.",
    body: "Write a procedure once in plain words, like how to refund a duplicate charge. The agent follows it and waits for approval where you say so.",
    visual: <ShotStill name="procedure" alt="A procedure in the editor: refunding a duplicate charge, step by step" />,
  },
  {
    label: "Handover",
    title: "It knows when to stop.",
    body: "When a person should take over, it passes the conversation to the right team with a summary. Then it gets out of the way.",
    visual: <AgentStill story={2} />,
  },
  {
    label: "Voice",
    title: "It sounds like your team.",
    body: "Give it a name, a tone, and guidance in plain words. No prompt engineering degree needed.",
    visual: <ShotStill name="agent-voice" alt="The agent's name, tone and guidance in settings" />,
  },
  {
    label: "Your key",
    title: "Your model, your bill.",
    body: "Bring your own model key and pay your provider directly. AI conversations then cost the same as human ones.",
    visual: <ModelFragment />,
  },
];

const questions = [
  {
    question: "Where does it actually answer?",
    answer:
      "On every channel the inbox has: the messenger, email, Slack, Microsoft Teams, Discord, WhatsApp, SMS, and the phone. You pick which channels and which customers.",
  },
  {
    question: "Does it read the whole thread?",
    answer: "Yes, all of it. It also reads the customer's details and their history before it answers.",
  },
  {
    question: "Will it talk over my team?",
    answer: "No. It steps back the moment a teammate joins and leaves a summary, so nobody starts from zero.",
  },
  {
    question: "Can it answer the phone?",
    answer:
      "Yes. It picks up, answers from the same knowledge, and follows the same procedures. When a person should take over, it transfers the call with a summary.",
  },
  {
    question: "Does it speak my customers' language?",
    answer: "Yes. It replies in the language the customer wrote in, from articles written in yours.",
  },
  {
    question: "Will it ever get one wrong?",
    answer:
      "Yes, sometimes. It's a language model, not an oracle. That's why every answer links its source, risky steps wait for approval, and test runs exist. If one wrong answer would be a disaster, keep a person on that topic.",
  },
  {
    question: "Can I try it without it messaging a real customer?",
    answer: "Yes. Test runs show the reply it would send on real conversations. Nothing is sent.",
  },
  {
    question: "What does it cost?",
    answer:
      "Nothing per answer. AI is in the conversation price. On your own model key, an AI conversation costs the same as a human one.",
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
        title="An AI agent that answers, acts, and hands over."
        body="It works the same threads your team does. You pick what it knows, what it may do, and how it sounds before a customer ever meets it."
        visual={<HeroDemo />}
      />
      <Steps title="Set it up in three steps." items={steps} />
      <Rows title="It does the work. You set the rules." rows={rows} />
      <Questions body="What you'd ask before letting it near customers." items={questions} />
      <FinalCta />
    </>
  );
}
