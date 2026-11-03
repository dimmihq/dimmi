/*
 * Every failure of a form shows under the field it concerns. The server
 * names the field when it can; a failure that names none, such as too
 * many attempts or a server that cannot be reached, shows under the
 * form's main field, so a form never needs a separate alert.
 */
import { ApiError } from "@/lib/api/client";

const unreachable = "The server could not be reached. Check your connection and try again.";

/*
 * fieldError returns the message to show under the field, or undefined.
 * The main field also shows failures that name no field of the form.
 */
export function fieldError(error: unknown, field: string, fields: string[], main: string): string | undefined {
  if (!error) {
    return undefined;
  }

  if (!(error instanceof ApiError)) {
    return field === main ? unreachable : undefined;
  }

  if (error.fields[field]) {
    return error.fields[field];
  }

  const namesFormField = fields.some((name) => error.fields[name]);

  return field === main && !namesFormField ? error.message : undefined;
}
