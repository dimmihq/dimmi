import { addStep, blankSpec, pathsOf, removeStep, setSettings, summary, targetOf, templateSpec } from "./model";

describe("series model", () => {
  it("hangs a new step off a path and keeps what the path led to after it", () => {
    let [spec, wait] = addStep(blankSpec(), "entry", "next", "wait");
    const [withEmail, email] = addStep(spec, "entry", "next", "email");
    spec = withEmail;

    expect(targetOf(spec, "entry", "next")).toBe(email);
    expect(targetOf(spec, email, "next")).toBe(wait);
  });

  it("removing a step joins the step before it to the one after it and drops what no longer connects", () => {
    const spec = templateSpec("onboarding");
    const removed = removeStep(spec, "wait-2d");

    expect(targetOf(removed, "welcome", "next")).toBe("nudge");
    expect(removed.nodes.map((n) => n.id)).toEqual(["entry", "welcome", "nudge"]);
  });

  it("drops the paths a check no longer has when its settings change", () => {
    const [spec, check] = addStep(blankSpec(), "entry", "next", "condition");
    const node = spec.nodes.find((n) => n.id === check);
    const path = node?.settings?.paths?.[0]?.id ?? "";
    const [withTag] = addStep(spec, check, path, "tag");

    const changed = setSettings(withTag, check, { paths: [] });

    expect(targetOf(changed, check, path)).toBeUndefined();
    expect(changed.nodes.some((n) => n.kind === "tag")).toBe(false);
  });

  it("names every path of a split with its share", () => {
    const [spec, split] = addStep(blankSpec(), "entry", "next", "split");
    const node = spec.nodes.find((n) => n.id === split);

    expect(node && pathsOf(node).map((p) => p.label)).toEqual(["A · 50%", "B · 50%"]);
    expect(node && summary(node, spec)).toBe("Split A 50% · B 50%");
  });
});
