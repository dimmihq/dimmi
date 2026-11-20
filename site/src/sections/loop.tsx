import { Panel, Points, Section, SplitHead } from "../components/container";
import { LoopScene } from "../scenes/loop-scene";

const points = [
  { title: "Collect requests", body: "On a public board, or straight from a conversation." },
  { title: "Show what's planned", body: "The roadmap moves as you change a status." },
  { title: "Tell everyone who asked", body: "Publishing the changelog emails every voter." },
];

export function Loop() {
  return (
    <Section anchor="feedback" frameClassName="flex flex-col gap-12">
      <SplitHead
        tag="Feedback"
        title="Tell customers when you ship what they asked for."
        body="The feedback board, roadmap, and changelog are linked. A request carries its voters all the way to release, so nobody has to ask for an update."
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
