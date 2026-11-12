/*
 * The fixed choices of the channel and priority filters.
 */
export interface Option {
  id: string;
  label: string;
}

export const channelOptions: Option[] = [
  { id: "widget", label: "Messenger" },
  { id: "email", label: "Email" },
  { id: "slack", label: "Slack" },
  { id: "api", label: "API" },
];

export const priorityOptions: Option[] = [
  { id: "urgent", label: "Urgent" },
  { id: "high", label: "High" },
  { id: "medium", label: "Medium" },
  { id: "low", label: "Low" },
];
