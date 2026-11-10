/*
 * The variables a message can carry: a person's or their company's
 * details, filled in for each person when the message goes out. Each
 * one is kept in the Markdown in Liquid's form, with the fallback used
 * for people who don't have that detail:
 *
 *	{{ contact.first_name | default: "there" }}
 *
 * so the server reads one plain format, whatever channel it renders to.
 */

/*
 * Variable is one detail a message can name: its key in the Markdown,
 * the name people read, the fallback a new one starts with, and what it
 * reads as for the sample person the preview is written to.
 */
export interface Variable {
  key: string;
  label: string;
  fallback: string;
  sample: string;
}

export const variables: Variable[] = [
  { key: "contact.first_name", label: "First name", fallback: "there", sample: "Ada" },
  { key: "contact.name", label: "Full name", fallback: "there", sample: "Ada Okafor" },
  { key: "contact.email", label: "Email", fallback: "", sample: "ada@northwind.io" },
  { key: "company.name", label: "Company", fallback: "your team", sample: "Northwind" },
  { key: "contact.plan", label: "Plan", fallback: "your plan", sample: "Pro" },
  { key: "sender.first_name", label: "Your first name", fallback: "", sample: "Wren" },
];

export const variablePattern = /^\{\{\s*([a-z_]+(?:\.[a-z_]+)*)\s*(?:\|\s*default:\s*"([^"]*)"\s*)?\}\}/;

const anywhere = new RegExp(variablePattern.source.slice(1), "g");

export function variableOf(key: string): Variable | undefined {
  return variables.find((variable) => variable.key === key);
}

/*
 * variableMarkdown writes a variable the way the Markdown keeps it.
 */
export function variableMarkdown(key: string, fallback: string): string {
  return fallback === "" ? `{{ ${key} }}` : `{{ ${key} | default: "${fallback.replaceAll('"', "")}" }}`;
}

/*
 * findVariables narrows the variables to those whose name or key holds
 * what was typed.
 */
export function findVariables(query: string): Variable[] {
  const needle = query.trim().toLowerCase();

  return variables.filter(
    (variable) => needle === "" || `${variable.label} ${variable.key}`.toLowerCase().includes(needle),
  );
}

/*
 * filled writes every variable in the Markdown as the sample person's
 * value, or its fallback when the sample has none.
 */
export function filled(markdown: string): string {
  return markdown.replace(anywhere, (_, key: string, fallback?: string) => {
    return variableOf(key)?.sample || fallback || "";
  });
}

/*
 * plainText writes every variable as its fallback, for showing a subject
 * outside any one person's message.
 */
export function plainText(markdown: string): string {
  return markdown.replace(anywhere, (_, _key: string, fallback?: string) => fallback ?? "…");
}
