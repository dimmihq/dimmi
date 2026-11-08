import { freshName } from "./fresh-name";

describe("freshName", () => {
  it("names a new workflow untitled, numbered past the names taken", () => {
    expect(freshName([])).toBe("Untitled workflow");
    expect(freshName(["untitled workflow", "Untitled workflow 2"])).toBe("Untitled workflow 3");
  });
});
