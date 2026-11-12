/*
 * The messenger's words for requests, from its texts, English when an
 * older config lacks them.
 */
const english: Record<string, string> = {
  tickets_get_help: "Get help with",
  tickets_open_only: "Open requests",
  tickets_all: "All",
  ticket_submitted: "Submitted",
  ticket_on_it: "We're on it",
  ticket_resolved: "Resolved",
  ticket_subject: "Subject",
  ticket_details: "Details",
  ticket_optional: "optional",
  ticket_send: "Send request",
  ticket_choose: "Choose…",
  ticket_attach: "Attach files",
  ticket_required: "Please fill this in",
  ticket_failed: "We couldn't send this. Try again.",
  status_title: "Service status",
  ticket_yes: "Yes",
};

export type Words = object;

/*
 * word reads one of the messenger's words for requests.
 */
export function word(texts: Words, key: string): string {
  return (texts as Record<string, string | undefined>)[key] || english[key] || key;
}
