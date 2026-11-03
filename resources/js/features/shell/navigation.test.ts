import { appSections, selectedKey, settingsLink, settingsSections } from "./navigation";

describe("selectedKey", () => {
  it("picks the row whose page holds the path, preferring the longest address", () => {
    const sections = settingsSections(false);

    expect(selectedKey(sections, [], "/d/settings/members")).toBe("settings-members");
    expect(selectedKey(appSections(), [settingsLink], "/d/settings/profile")).toBe("settings");
    expect(selectedKey(appSections(), [settingsLink], "/d/inbox/all")).toBe("inbox-all");
  });

  it("selects nothing for a path no row holds", () => {
    expect(selectedKey(appSections(), [settingsLink], "/d/unknown")).toBe("");
  });

  it("never treats an address as holding another that only starts with its letters", () => {
    const sections = [{ key: "s", items: [{ key: "doc", label: "Doc", icon: () => null, href: "/d/doc" }] }];

    expect(selectedKey(sections, [], "/d/docs")).toBe("");
  });
});

describe("settingsSections", () => {
  it("offers billing on the cloud only", () => {
    const labels = (cloud: boolean) =>
      settingsSections(cloud).flatMap((section) => section.items.map((item) => item.label));

    expect(labels(true)).toContain("Billing");
    expect(labels(false)).not.toContain("Billing");
  });
});
