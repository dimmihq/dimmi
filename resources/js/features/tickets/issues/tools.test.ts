import { describe, expect, it } from "vitest";

import { providerLook } from "@/features/settings/integrations";

import { IncidentIOMark, JiraMark, RootlyMark } from "./tool-logos";
import { toolByLabel, toolLook } from "./tools";

describe("tool marks", () => {
  it("draws incident.io and Rootly with their own marks and colours", () => {
    expect(providerLook("incident_io")).toMatchObject({ icon: IncidentIOMark, brand: "#F25533" });
    expect(providerLook("rootly")).toMatchObject({ icon: RootlyMark, brand: "#8D6FDE" });
    expect(providerLook("jira")).toMatchObject({ icon: JiraMark, brand: "#0052CC" });
  });

  it("names the tool a synced note came from", () => {
    expect(toolByLabel("incident.io")).toBe("incident_io");
    expect(toolByLabel("Rootly")).toBe("rootly");
    expect(toolByLabel("dimmi")).toBeUndefined();
  });

  it("holds incidents, not issues", () => {
    expect(toolLook("incident_io").kind).toBe("incidents");
    expect(toolLook("rootly").kind).toBe("incidents");
  });
});
