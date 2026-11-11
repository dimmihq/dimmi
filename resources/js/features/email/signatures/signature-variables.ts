/*
 * The details a signature can name: the teammate's name and job title,
 * the address's display name and the company's name. The server fills
 * them in when a reply is written and leaves out a line whose detail is
 * empty, so they carry no fallback. A signature keeps them as
 * {{member_name}} in its Markdown, and every line break of the stored
 * signature is a line break in the reply. The writer works in paragraphs
 * and breaks instead, so the text is turned into the writer's Markdown
 * on the way in and back on the way out.
 */

/*
 * SignatureVariable is one detail: its key in the stored text, the name
 * the chip shows and what it is filled in with.
 */
export interface SignatureVariable {
  key: string;
  label: string;
  description: string;
}

export const signatureVariables: SignatureVariable[] = [
  { key: "member_name", label: "Teammate name", description: "The name of the teammate who replies." },
  { key: "member_title", label: "Job title", description: "The job title of the teammate who replies." },
  { key: "address_name", label: "Address name", description: "The display name of the address replied from." },
  { key: "company_name", label: "Company name", description: "Your company's name." },
];

export const signatureVariablePattern = /^\{\{\s*(member_name|member_title|address_name|company_name)\s*\}\}/;

export function signatureVariableOf(key: string): SignatureVariable | undefined {
  return signatureVariables.find((variable) => variable.key === key);
}

/*
 * findSignatureVariables narrows the details to those whose name or key
 * holds what was typed.
 */
export function findSignatureVariables(query: string): SignatureVariable[] {
  const needle = query.trim().toLowerCase();

  return signatureVariables.filter(
    (variable) => needle === "" || `${variable.label} ${variable.key}`.toLowerCase().includes(needle),
  );
}

/*
 * toWriter turns a stored signature into the writer's Markdown: a lone
 * line break becomes a hard break, which the writer keeps as one.
 */
export function toWriter(body: string): string {
  return body
    .replaceAll("\r\n", "\n")
    .split(/\n{2,}/)
    .map((paragraph) =>
      paragraph
        .split("\n")
        .map((line) => line.trimEnd())
        .join("\\\n"),
    )
    .join("\n\n");
}

/*
 * fromWriter turns the writer's Markdown back into the stored form: a
 * hard break becomes a lone line break, trailing spaces go, and what
 * the writer encodes only for safety reads as typed: "&" and angle
 * brackets, and an underscore inside a word, which never marks italic.
 */
export function fromWriter(markdown: string): string {
  return markdown
    .replaceAll("&nbsp;", " ")
    .replaceAll("&lt;", "<")
    .replaceAll("&gt;", ">")
    .replaceAll("&amp;", "&")
    .replace(/(?<=[\p{L}\p{N}])\\_(?=[\p{L}\p{N}])/gu, "_")
    .replace(/(?:\\| {2,}|<br\s*\/?>)\n/g, "\n")
    .split("\n")
    .map((line) => line.trimEnd())
    .join("\n")
    .replace(/\n{3,}/g, "\n\n")
    .trim();
}
