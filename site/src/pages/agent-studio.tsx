import {
  ChartLineUp,
  CheckSquareOffset,
  ClockCounterClockwise,
  Flask,
  ShieldCheck,
  Sliders,
} from "@phosphor-icons/react";

import { FeatureGrid, HowItWorks, ProductHero, Questions, Rows } from "../components/product-page";
import { ShotStill } from "../components/shot-still";
import { ShotVideo } from "../components/shot-video";
import { AgentStudioLoopFragment } from "../fragments/agent-studio-fragments";
import { FinalCta } from "../sections/final-cta";

const how = [
  {
    title: "Write guidance linked to real work",
    body: "Type @ inside your agent's instructions to link real procedures, help articles, actions, and teams, with a live checklist of anything to fix before publishing.",
  },
  {
    title: "Catch flaky replies with simulations",
    body: "Save real chats or turn last week's handovers into simulations. Agent Studio tracks recent runs as pass and fail dots so one lucky pass never hides a flaky reply.",
  },
  {
    title: "Compare Draft against Live side by side",
    body: "Run your draft and live agent on the exact same customer conversation simultaneously, with pinned checks under each column flagging every disagreement.",
  },
  {
    title: "Diagnose and fix failures with Operator",
    body: "Ask Operator why a simulation failed right beside the editor. Review its proposed draft fix, apply it in one click, and re-run to confirm.",
  },
];

const rows = [
  {
    label: "Build canvas",
    title: "Link real procedures, docs, and actions right inside your guidance.",
    body: "Everything your agent reads and follows sits on one page. Type @ to link a procedure, help article, action, or team, lock down its scopes, and see what still needs fixing before you publish.",
    visual: (
      <ShotStill
        name="agent-studio-guidance"
        alt="Agent Studio Build canvas with inline @ guidance chips and enforced Knowledge and Procedure scopes"
      />
    ),
  },
  {
    label: "Multi-run simulations",
    title: "One lucky pass proves nothing. Track flakiness and read why every check failed.",
    body: "AI replies vary from run to run, so Agent Studio scores each simulation across its latest runs as Passing, Flaky, or Failing, and explains in plain English why every check passed or failed.",
    visual: (
      <ShotStill
        name="agent-studio-simulations"
        alt="Agent Studio Simulations panel showing multi-run pass and fail dots, past runs, and plain-English check reasons"
        maxWidth={388}
      />
    ),
  },
  {
    label: "Draft vs. live",
    title: "Watch Draft and Live answer the exact same customer side by side.",
    body: "Click Compare to run your working draft and your live agent on the same scenario at the same time. Spot every reply and check where they disagree before a single customer sees your edit.",
    visual: (
      <ShotVideo
        name="agent-studio-compare"
        poster="agent-studio-compare"
        alt="High-resolution walkthrough comparing Draft and Live agent runs side by side in Agent Studio"
        maxWidth={560}
      />
    ),
  },
  {
    label: "Operator in the Studio",
    title: "Turn last week's handovers into tests, and let Operator propose the fix.",
    body: "Operator reads your agent's draft, simulations, and recent conversations right beside the editor. Ask it to turn recent handovers into tests or explain a failing run and draft the exact fix for you to apply.",
    visual: (
      <ShotVideo
        name="agent-studio-operator"
        poster="agent-studio-operator"
        alt="High-resolution walkthrough of Operator inside Agent Studio diagnosing a failing simulation and proposing a draft fix"
        maxWidth={385}
      />
    ),
  },
];

const more = [
  {
    icon: Flask,
    title: "Safe Preview with mock actions",
    body: "Chat with your draft as a real or made-up customer while actions return saved test data. Zero messages reach customers.",
  },
  {
    icon: ClockCounterClockwise,
    title: "Numbered releases & line diffs",
    body: "Every publish creates a numbered version. Compare guidance and scope changes line by line and roll back in one click.",
  },
  {
    icon: ChartLineUp,
    title: "30-day per-agent Analytics",
    body: "Inspect resolution rates, handovers, and ratings for each agent, or jump straight into Reports to compare agents side by side.",
  },
  {
    icon: CheckSquareOffset,
    title: "Pre-publish safety checks",
    body: "dimmi blocks publishing if a linked procedure, article, team, or action is missing or unpublished so broken rules never go live.",
  },
  {
    icon: Sliders,
    title: "Per-agent models & spend caps",
    body: "Pick the right model and optional monthly budget cap for each agent, or plug in your own keys to save 50% to 83% on AI fees.",
  },
  {
    icon: ShieldCheck,
    title: "Operator never publishes alone",
    body: "Every fix Operator proposes lands as a reviewable changeset with one-click Apply, Decline, and Undo. Publishing stays with your team.",
  },
];

const questions = [
  {
    question: "Why does Agent Studio track multiple runs instead of a single pass or fail?",
    answer:
      "AI answers vary slightly from one run to the next. A single pass can hide an unreliable instruction, so Agent Studio tracks the latest runs on your current copy and marks a test Passing, Flaky, or Failing before you publish.",
  },
  {
    question: "How does Draft vs. Live comparison work?",
    answer:
      "Every edit saves to a private draft while your live agent keeps answering customers. Click Compare on any simulation to run both versions on the exact same customer messages and inspect both replies and checks side by side.",
  },
  {
    question: "Can Operator change our live support agent on its own?",
    answer:
      "Operator only proposes edits to your working draft as reviewable changesets. Your team reviews the diff, applies or declines in one click, tests the draft, and decides when to publish.",
  },
  {
    question: "Does testing in Preview or Simulations trigger real refunds or emails?",
    answer:
      "Preview and Simulations never email customers or call your production APIs. Every connected action answers with its saved mock or last test response so your team can test high-stakes billing flows safely.",
  },
  {
    question: "What happens if an update lowers live resolution rates after we publish?",
    answer:
      "Open the History tab, compare the live version against any earlier release line by line, and publish the earlier version back in one click.",
  },
  {
    question: "Is Agent Studio an extra paid add-on?",
    answer:
      "Agent Studio, unlimited agents, simulations, Draft vs. Live comparison, and release history are included on every workspace and in the MIT-licensed self-hosted build.",
  },
];

export function AgentStudioPage() {
  return (
    <>
      <ProductHero
        eyebrow="Agent Studio"
        title="Build, test, and improve support agents without guessing."
        body="Write guidance linked to your real docs and policies, catch flaky replies with multi-run simulations, compare Draft against Live side by side, and let Operator diagnose failures before customers see a change."
        visual={
          <ShotVideo
            name="agent-studio-tour"
            poster="agent-studio-hero"
            alt="High-resolution tour of dimmi Agent Studio showing Build guidance, multi-run simulations, Draft vs. Live comparison, and Operator"
            bleed
          />
        }
      />
      <HowItWorks
        label="The quality loop"
        title="Edit in draft, catch flakiness early, ship with proof."
        body="Stop tweaking support prompts in the dark and hoping nothing broke. Agent Studio gives your team one workspace to write, test, diagnose, and release."
        steps={how}
        visual={(step) => <AgentStudioLoopFragment step={step} />}
      />
      <Rows title="Built for support teams who measure every change." rows={rows} />
      <FeatureGrid title="Everything you need to ship reliable agents, every week." items={more} />
      <Questions body="What support leads ask about testing, flakiness, and safe releases." items={questions} />
      <FinalCta />
    </>
  );
}
