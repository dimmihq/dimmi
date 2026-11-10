/*
 * One number on an agent's page, on a card: what it counts, the value,
 * and an optional note under it, such as the cap the spend is held to.
 */
import type { ReactNode } from "react";

export function Stat({ label, value, note }: { label: string; value: ReactNode; note?: string }) {
  return (
    <div className="flex flex-col gap-1 rounded-2xl bg-background-secondary-default p-4">
      <span className="text-body-2-regular text-text-secondary">{label}</span>
      <span className="text-title-2-medium text-text-primary tabular-nums">{value}</span>
      {note ? <span className="text-caption-1-medium text-text-tertiary">{note}</span> : null}
    </div>
  );
}
