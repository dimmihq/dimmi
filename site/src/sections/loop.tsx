import { Panel, Points, Section, SplitHead } from "../components/container";
import { LoopScene } from "../scenes/loop-scene";

const points = [
  {
    title: "Collect requests anywhere",
    body: "Let customers vote on a public board, or link ideas straight from chat and email.",
  },
  {
    title: "A roadmap that stays fresh",
    body: "Pick which statuses customers see. Move a request inside dimmi and the board updates too.",
  },
  {
    title: "Email every voter at launch",
    body: "Link shipped items to a changelog post and notify everyone who asked in one click.",
  },
];

export function Loop() {
  return (
    <Section anchor="feedback" frameClassName="flex flex-col gap-12">
      <SplitHead
        tag="Feedback & changelog"
        title="Close the loop when you ship what customers asked for."
        body="Feedback boards, your roadmap, and your changelog share one home on your domain. Link a chat to a request, and voters hear when it ships."
        link={{ href: "/feedback", label: "Explore feedback" }}
      />
      <div className="grid grid-cols-1 items-center gap-x-16 gap-y-10 lg:grid-cols-[1fr_minmax(0,340px)] [&>*]:min-w-0">
        <Panel className="px-6 py-10 sm:px-10">
          <LoopScene />
        </Panel>
        <Points items={points} className="max-lg:order-first" />
      </div>
    </Section>
  );
}
