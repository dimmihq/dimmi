/*
 * The kinds of callout, as the help center names them in Markdown,
 * lowercased.
 */
export const calloutKinds = ["note", "tip", "important", "warning", "caution"] as const;

export type CalloutKind = (typeof calloutKinds)[number];
