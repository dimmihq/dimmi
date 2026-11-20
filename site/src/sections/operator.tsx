import { Panel, Points, Section, SplitHead } from "../components/container";
import { ShotFlow } from "../scenes/shot-flow";

const points = [
  { title: "Briefs you in seconds", body: "The thread and the customer, summed up before you read a word." },
  { title: "Drafts in your voice", body: "Replies written from your knowledge and past answers. You edit, you send." },
  {
    title: "Answers from your data",
    body: "Ask about the customer, their history or your docs, in the thread or in Slack.",
  },
];

export function Operator() {
  return (
    <Section anchor="operator" frameClassName="flex flex-col gap-12">
      <SplitHead
        tag="Operator"
        title="An AI teammate that stays backstage."
        body="Operator works beside each conversation. Only your team sees it, and nothing reaches a customer until you press send."
        link={{ href: "/operator", label: "See Operator" }}
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
