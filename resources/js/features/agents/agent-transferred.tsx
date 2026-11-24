/*
 * The line the team sees where one AI agent handed the chat to another,
 * with the note it left. The customer never sees it.
 */
export function Transferred({ transfer }: { transfer: { name: string; from_name: string; summary: string } }) {
  return (
    <span className="flex items-center gap-2 self-stretch text-center text-body-2-regular text-text-tertiary">
      <span className="h-px min-w-4 flex-1 bg-separator-border" />
      {transfer.from_name} transferred to {transfer.name}
      {transfer.summary ? `: ${transfer.summary}` : ""}
      <span className="h-px min-w-4 flex-1 bg-separator-border" />
    </span>
  );
}
