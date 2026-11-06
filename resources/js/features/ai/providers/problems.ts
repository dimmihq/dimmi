/*
 * What is wrong with a provider form, field by field: what is missing
 * before anything is sent, and what the server refused.
 */
import { ApiError } from "@/lib/api/client";

type FieldName = "name" | "api_key" | "base_url";

export type Problems = Partial<Record<FieldName | "form", string>>;

/*
 * codeFields says which field a refusal the server names by code
 * belongs to.
 */
const codeFields: Record<string, FieldName> = {
  name_required: "name",
  key_required: "api_key",
  base_url_required: "base_url",
};

/*
 * problemsOf turns a refusal into the messages each field shows.
 */
export function problemsOf(error: unknown): Problems {
  if (!(error instanceof ApiError)) {
    return { form: "dimmi could not reach the server. Check your connection and try again." };
  }
  const problems: Problems = {};
  for (const [field, message] of Object.entries(error.fields)) {
    if (field === "name" || field === "api_key" || field === "base_url") {
      problems[field] = message;
    }
  }
  const field = codeFields[error.code];
  if (field) {
    problems[field] = error.message;
  }
  if (Object.keys(problems).length === 0) {
    problems.form = error.status === 403 ? "Only admins can add providers." : error.message;
  }

  return problems;
}

/*
 * checkFields finds what is missing before anything is sent.
 */
export function checkFields(provider: string, fields: { name: string; apiKey: string; baseUrl: string }): Problems {
  const problems: Problems = {};
  if (fields.name.trim() === "") {
    problems.name = "Give the connection a name.";
  }
  if (provider === "openai_compatible") {
    if (!/^https?:\/\/\S+$/.test(fields.baseUrl.trim())) {
      problems.base_url = "Enter the server's address, starting with https://.";
    }
  } else if (fields.apiKey.trim() === "") {
    problems.api_key = "Paste the API key.";
  }

  return problems;
}
