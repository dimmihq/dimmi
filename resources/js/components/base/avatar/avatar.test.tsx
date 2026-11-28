import { fireEvent, render } from "@testing-library/react";

import { Avatar } from "./avatar";

describe("Avatar", () => {
  it("shows the initials when its photo fails to load", () => {
    const { container } = render(<Avatar src="https://example.test/gone.jpg" initials="H" />);
    const photo = container.querySelector("img");
    expect(photo).not.toBeNull();

    fireEvent.error(photo as HTMLImageElement);

    expect(container.querySelector("img")).toBeNull();
    expect(container.textContent).toBe("H");
  });

  it("tries again when given another photo", () => {
    const { container, rerender } = render(<Avatar src="https://example.test/gone.jpg" initials="H" />);
    fireEvent.error(container.querySelector("img") as HTMLImageElement);

    rerender(<Avatar src="https://example.test/new.jpg" initials="H" />);

    expect(container.querySelector("img")?.getAttribute("src")).toBe("https://example.test/new.jpg");
  });
});
