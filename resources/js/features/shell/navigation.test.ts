import { appSections, selectedKey, settingsLink, settingsSections } from "./navigation";

describe("selectedKey", () => {
  it("picks the row whose page holds the path, preferring the longest address", () => {
    const sections = settingsSections();

    expect(selectedKey(sections, [], "/d/settings/members")).toBe("settings-members");
    expect(selectedKey(appSections(), [settingsLink], "/d/settings/profile")).toBe("settings");
    expect(selectedKey(appSections(), [settingsLink], "/d/inbox/all")).toBe("inbox");
    expect(selectedKey(appSections(), [settingsLink], "/d/inbox/team_a/conv_1")).toBe("inbox");
  });

  it("selects nothing for a path no row holds", () => {
    expect(selectedKey(appSections(), [settingsLink], "/d/unknown")).toBe("");
  });

  it("never treats an address as holding another that only starts with its letters", () => {
    const sections = [{ key: "s", items: [{ key: "doc", label: "Doc", icon: () => null, href: "/d/doc" }] }];

    expect(selectedKey(sections, [], "/d/help-center")).toBe("");
  });
});

describe("settingsSections", () => {
  it("lists no page that only shows an example", () => {
    const labels = settingsSections().flatMap((section) => section.items.map((item) => item.label));

    for (const label of ["Billing", "Import & export", "Connections", "GitHub"]) {
      expect(labels).not.toContain(label);
    }
  });
});
