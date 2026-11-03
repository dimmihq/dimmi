import { render, screen } from "@testing-library/react";

import { DimmiIcon, DimmiIconSmall, DimmiLogo, DimmiWordmark } from "./logo";

describe("logo", () => {
  it.each([
    ["icon", DimmiIcon, "0 0 40 40", 3],
    ["small icon", DimmiIconSmall, "0 0 40 40", 2],
    ["wordmark", DimmiWordmark, "0 0 2104 652", 5],
    ["logo", DimmiLogo, "0 0 2880.632 656.632", 8],
  ])("draws the %s in the current text color", (_, Component, viewBox, paths) => {
    render(<Component className="h-8" />);

    const svg = screen.getByRole("img", { name: "Dimmi" });
    expect(svg.getAttribute("viewBox")).toBe(viewBox);
    expect(svg.getAttribute("fill")).toBe("currentColor");
    expect(svg.getAttribute("class")).toBe("h-8");
    expect(svg.querySelectorAll("path")).toHaveLength(paths);
  });

  it("takes a custom accessible name", () => {
    render(<DimmiLogo title="Dimmi home" />);

    expect(screen.getByRole("img", { name: "Dimmi home" })).toBeTruthy();
  });
});
