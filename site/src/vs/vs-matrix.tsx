/*
 * Reusable capability comparison matrix extracted from `compare.tsx`.
 * Renders both 2-column dimmi-vs-rival tables and multi-column comparisons.
 */
import { RiCheckLine, RiCloseLine, RiExternalLinkLine, RiSubtractLine } from "@remixicon/react";
import { Fragment } from "react";

import { cx } from "@/utils/cx";

import { Intro, Section } from "../components/container";
import { HELP_URL } from "../links";

const mono = "font-mono text-[12px] leading-5 font-medium tracking-[0.04em] uppercase";

export type CellStatus = "yes" | "partial" | "no";
export type MatrixCell = { status: CellStatus; label?: string };

export type MatrixColumn<K extends string = string> = {
  key: K;
  label: string;
  highlight?: boolean;
};

export type GenericMatrixRow<K extends string = string> = {
  feature: string;
  requestTitle?: string;
  cells: Record<K, MatrixCell>;
};

export type GenericMatrixGroup<K extends string = string> = {
  title: string;
  rows: GenericMatrixRow<K>[];
};

function requestUrl(title: string): string {
  return `${HELP_URL}/feedback/new?${new URLSearchParams({ title }).toString()}`;
}

export function StatusMark({ status }: { status: CellStatus }) {
  if (status === "yes") {
    return (
      <span className="inline-flex size-5 shrink-0 items-center justify-center rounded-full bg-accent-500/15 text-accent-600 dark:text-accent-400">
        <RiCheckLine className="size-3.5" aria-hidden />
      </span>
    );
  }
  if (status === "partial") {
    return (
      <span className="inline-flex size-5 shrink-0 items-center justify-center rounded-full bg-background-secondary-default text-text-tertiary">
        <RiSubtractLine className="size-3.5" aria-hidden />
      </span>
    );
  }
  return (
    <span className="inline-flex size-5 shrink-0 items-center justify-center rounded-full bg-background-secondary-default text-text-tertiary/65">
      <RiCloseLine className="size-3.5" aria-hidden />
    </span>
  );
}

function MatrixDataCell({
  cell,
  highlight = false,
  requestTitle,
}: {
  cell: MatrixCell;
  highlight?: boolean;
  requestTitle?: string;
}) {
  const showRequest = highlight && requestTitle ? requestTitle : null;
  const labelClass = highlight && cell.status === "yes" ? "font-medium text-text-primary" : "text-text-secondary";

  return (
    <td
      className={cx("whitespace-nowrap px-4 py-3.5", highlight && "border-x border-accent-500/15 bg-accent-500/[0.03]")}
    >
      <span className="inline-flex items-center gap-2">
        <StatusMark status={cell.status} />
        {showRequest ? (
          <a
            href={requestUrl(showRequest)}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1 text-text-tertiary underline-offset-4 hover:text-text-primary hover:underline"
          >
            Request
            <RiExternalLinkLine className="size-3" aria-hidden />
          </a>
        ) : cell.label ? (
          <span className={cx("tabular-nums", labelClass)}>{cell.label}</span>
        ) : null}
      </span>
    </td>
  );
}

export function CapabilityMatrix<K extends string>({
  title,
  body,
  columns,
  groups,
  minWidthClass = "min-w-[640px]",
}: {
  title: string;
  body: string;
  columns: readonly MatrixColumn<K>[];
  groups: GenericMatrixGroup<K>[];
  minWidthClass?: string;
}) {
  const colSpan = columns.length + 1;

  return (
    <Section frameClassName="flex flex-col gap-12">
      <Intro title={title} body={body} />
      <div className="overflow-x-auto rounded-2xl border border-border-button-default bg-background-full">
        <table className={cx("w-full border-collapse text-start", minWidthClass)}>
          <thead>
            <tr className="border-b border-separator-border">
              <th className={`px-6 py-4 text-start text-text-tertiary ${mono}`}>Capability</th>
              {columns.map((col) => (
                <th
                  key={col.key}
                  className={cx(
                    "whitespace-nowrap px-4 py-4 text-start text-[14px] font-medium",
                    col.highlight
                      ? "border-x border-accent-500/20 bg-accent-500/[0.08] text-accent-700 dark:text-accent-300"
                      : "text-text-primary",
                  )}
                >
                  {col.label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-separator-border text-[14px] leading-5">
            {groups.map((group) => (
              <Fragment key={group.title}>
                <tr className="bg-background-secondary-default/60">
                  <th
                    colSpan={colSpan}
                    scope="colgroup"
                    className={`px-6 py-2.5 text-start text-text-tertiary ${mono}`}
                  >
                    {group.title}
                  </th>
                </tr>
                {group.rows.map((row) => (
                  <tr key={row.feature} className="transition-colors hover:bg-background-secondary-default/40">
                    <th scope="row" className="px-6 py-3.5 text-start align-top font-medium text-text-primary">
                      {row.feature}
                    </th>
                    {columns.map((col) => (
                      <MatrixDataCell
                        key={col.key}
                        cell={row.cells[col.key]}
                        highlight={col.highlight}
                        requestTitle={row.requestTitle}
                      />
                    ))}
                  </tr>
                ))}
              </Fragment>
            ))}
          </tbody>
        </table>
      </div>
    </Section>
  );
}

export type VsMatrixRow = {
  feature: string;
  dimmi: MatrixCell;
  rival: MatrixCell;
  requestTitle?: string;
};

export type VsMatrixGroup = {
  title: string;
  rows: VsMatrixRow[];
};

export function VsMatrix({
  title,
  body,
  rivalName,
  groups,
}: {
  title: string;
  body: string;
  rivalName: string;
  groups: VsMatrixGroup[];
}) {
  return (
    <CapabilityMatrix
      title={title}
      body={body}
      columns={[
        { key: "dimmi", label: "dimmi", highlight: true },
        { key: "rival", label: rivalName, highlight: false },
      ]}
      groups={groups.map((group) => ({
        title: group.title,
        rows: group.rows.map((row) => ({
          feature: row.feature,
          requestTitle: row.requestTitle,
          cells: { dimmi: row.dimmi, rival: row.rival },
        })),
      }))}
    />
  );
}
