import { Panel, Points, Section, SplitHead } from "../components/container";
import { WorkflowScene } from "../scenes/workflow-scene";

const points = [
  { title: "Start from anything", body: "A message, a tag, a product event, an SLA at risk, or a schedule." },
  { title: "Build it your way", body: "Start from a template, or describe what you want and let dimmi draft it." },
  { title: "Reach into your systems", body: "Call your API, create tickets, and update customers as steps." },
];

export function Automate() {
  return (
    <Section anchor="workflows" frameClassName="flex flex-col gap-12">
      <SplitHead
        tag="Workflows"
        title="Automate the boring parts."
        body="Sort, route and follow up on a canvas you design. Every run is logged, step by step."
        link={{ href: "/workflows", label: "See workflows" }}
      />
      <div className="grid grid-cols-1 items-center gap-x-16 gap-y-10 lg:grid-cols-[minmax(0,340px)_1fr] [&>*]:min-w-0">
        <Points items={points} />
        <Panel className="px-6 py-14 sm:px-10">
          <WorkflowScene />
        </Panel>
      </div>
    </Section>
  );
}
