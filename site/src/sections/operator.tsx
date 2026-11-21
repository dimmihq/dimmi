import { Panel, Points, Section, SplitHead } from "../components/container";
import { ShotFlow } from "../scenes/shot-flow";

const points = [
  {
    title: "Catch up in 5 seconds",
    body: "Reads a long thread and past tickets, then leaves a private summary right in the inbox.",
  },
  {
    title: "Drafts from your docs",
    body: "Writes a reply in your team's voice right in the box. You edit and send it.",
  },
  {
    title: "Changes you approve first",
    body: "Ask it to tag threads, pull reports, or edit a playbook. Every change waits for your OK.",
  },
];

export function Operator() {
  return (
    <Section anchor="operator" frameClassName="flex flex-col gap-12">
      <SplitHead
        tag="Operator"
        title="An internal AI teammate that never messages your customers."
        body="Operator sits beside every thread and in Slack. Ask about a customer, draft a reply, or review workspace edits you can undo."
        link={{ href: "/operator", label: "Explore Operator" }}
      />
      <div className="grid grid-cols-1 items-start gap-x-16 gap-y-10 lg:grid-cols-[minmax(0,1fr)_minmax(0,464px)] [&>*]:min-w-0">
        <Points items={points} />
        <Panel className="h-[540px] pt-12">
          <ShotFlow name="operator" alt="Asking Operator about the customer" maxHeight={488} maxWidth={400} />
        </Panel>
      </div>
    </Section>
  );
}
