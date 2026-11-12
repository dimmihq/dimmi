/*
 * A chart's own data as a spreadsheet, built in the browser: a table's
 * rows by its columns, or every point of the axis by series.
 */
import type { ReportChart } from "@/lib/api/reports";

import { csvValue } from "./format";

function cell(value: string): string {
  return /[",\n\r]/.test(value) ? `"${value.replaceAll('"', '""')}"` : value;
}

function tableLines(chart: ReportChart): string[][] {
  return [
    ["", ...chart.columns.map((c) => c.label)],
    ...chart.rows.map((row) => [
      row.label,
      ...row.values.map((v, i) => csvValue(chart.columns[i]?.unit ?? chart.unit, v)),
    ]),
  ];
}

function seriesLines(chart: ReportChart): string[][] {
  const xs = [...new Set(chart.series.flatMap((s) => s.points.map((p) => p.x)))];
  const values = chart.series.map((s) => new Map(s.points.map((p) => [p.x, p.y])));

  return [
    [chart.axis, ...chart.series.map((s) => s.label)],
    ...xs.map((x) => [x, ...values.map((m) => csvValue(chart.unit, m.get(x)))]),
  ];
}

export function chartCsv(chart: ReportChart): string {
  const table = chart.kind === "table" || (chart.series.length === 0 && chart.rows.length > 0);
  const lines = table ? tableLines(chart) : seriesLines(chart);

  return `${lines.map((line) => line.map(cell).join(",")).join("\n")}\n`;
}

/*
 * saveFile hands the browser a file under its own name.
 */
export function saveFile(contents: string, filename: string, type = "text/csv") {
  const url = URL.createObjectURL(new Blob([contents], { type }));
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  link.click();
  URL.revokeObjectURL(url);
}
