/*
 * The tabs of Operator's page, each at its own address, and how one is
 * read from the address.
 */
export const operatorTabs = ["overview", "suggestions", "routines", "activity", "settings"] as const;
export type OperatorTab = (typeof operatorTabs)[number];

/*
 * operatorTab reads a tab from the address, the overview when it names
 * none or one that does not exist.
 */
export function operatorTab(value: unknown): OperatorTab {
  return operatorTabs.find((tab) => tab === value) ?? "overview";
}

/*
 * operatorAddress is where a tab lives: the overview at /d/operator, any
 * other under it.
 */
export function operatorAddress(tab: OperatorTab) {
  return tab === "overview" ? ({ to: "/d/operator" } as const) : ({ to: "/d/operator/$tab", params: { tab } } as const);
}
