import { Panel, Points, Section, SplitHead } from "../components/container";
import { WorkflowScene } from "../scenes/workflow-scene";

const points = [
  {
    title: "Start from real events",
    body: "Trigger on a new message, a tag, an app event, an SLA alert, or a timer.",
  },
  {
    title: "Templates or plain English",
    body: "Start from a template, build from scratch, or describe what you want and let dimmi draft it.",
  },
  {
    title: "Call your own API",
    body: "Send requests to your backend, open tickets, or hand threads straight to the dimmi agent.",
  },
];

export function Automate() {
  return (
    <Section anchor="workflows" frameClassName="flex flex-col gap-12">
      <SplitHead
        tag="Workflows"
        title="Automate routing, SLAs, and follow-ups on every plan."
        body="Build triggers and actions on a visual canvas included for everyone. Check the run log anytime to see every step that ran at 3am."
        link={{ href: "/workflows", label: "Explore workflows" }}
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
