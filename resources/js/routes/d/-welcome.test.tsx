import { fireEvent, render, screen } from "@testing-library/react";

import { Welcome } from "./-welcome";

describe("Welcome", () => {
  it("counts presses of its button", () => {
    render(<Welcome />);

    fireEvent.click(screen.getByRole("button", { name: "Clicked 0 times" }));

    expect(screen.getByRole("button", { name: "Clicked 1 times" })).toBeTruthy();
  });
});
