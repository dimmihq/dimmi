/*
 * How AI conversation attribute values read on screen and in report
 * filters: yes or no for a yes-or-no attribute, a number in the
 * reader's locale, a date as a short date, and text as it is. Reports
 * split by the choice and yes-or-no attributes only, since their values
 * are a short, known list; with none picked, the first active one.
 */
import type { AIAttribute } from "@/lib/api/ai-attributes";

export interface ValueOption {
  id: string;
  label: string;
}

/*
 * aiValueLabel writes a value the AI found, or null when it found none.
 */
export function aiValueLabel(attribute: Pick<AIAttribute, "data_type">, value: unknown): string | null {
  if (value === null || value === undefined || value === "") {
    return null;
  }

  switch (attribute.data_type) {
    case "boolean":
      return value === true || value === "true" ? "Yes" : "No";
    case "number":
      return typeof value === "number" ? value.toLocaleString() : String(value);
    case "date": {
      const at = new Date(String(value));
      return Number.isNaN(at.getTime())
        ? String(value)
        : at.toLocaleDateString(undefined, { day: "numeric", month: "short", year: "numeric", timeZone: "UTC" });
    }
    default:
      return typeof value === "string" ? value : JSON.stringify(value);
  }
}

/*
 * aiValueOptions are the values a report filter offers for an
 * attribute: a choice's options, or yes and no. Other types offer none
 * and filter on whether a value was found.
 */
export function aiValueOptions(attribute: Pick<AIAttribute, "data_type" | "options">): ValueOption[] {
  if (attribute.data_type === "boolean") {
    return [
      { id: "true", label: "Yes" },
      { id: "false", label: "No" },
    ];
  }

  if (attribute.data_type === "choice") {
    return attribute.options.map((o) => ({ id: o, label: o }));
  }

  return [];
}

/*
 * splitsReports says whether a report can split by the attribute.
 */
export function splitsReports(attribute: Pick<AIAttribute, "data_type">): boolean {
  return attribute.data_type === "choice" || attribute.data_type === "boolean";
}

/*
 * defaultBreakdown is the attribute the AI report splits by when none is
 * picked: the first active choice or yes-or-no one, in the order the API
 * lists them, as the server picks it.
 */
export function defaultBreakdown(attributes: AIAttribute[]): AIAttribute | undefined {
  return attributes.find((a) => a.status === "active" && splitsReports(a));
}

/*
 * aiFilterSummary says what the AI attribute filter is set to, such as
 * "Reason: billing", or "Reason: found" with no value.
 */
export function aiFilterSummary(
  attributes: AIAttribute[],
  key: string,
  value: string | undefined,
): { attribute: string; value: string } {
  const attribute = attributes.find((a) => a.key === key);
  if (!value) {
    return { attribute: attribute?.label ?? key, value: "found" };
  }

  return {
    attribute: attribute?.label ?? key,
    value: (attribute && aiValueLabel(attribute, value)) ?? value,
  };
}
