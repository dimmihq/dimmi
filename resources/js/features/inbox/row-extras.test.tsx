import { render, screen } from "@testing-library/react";

import { ChannelBadge } from "./row-extras";

function badge(type: string) {
  render(<ChannelBadge channel={{ id: "ch_1", type, name: "" }} />);

  return screen.getByRole("img");
}

describe("ChannelBadge", () => {
  it.each([
    ["facebook", "Via Facebook Messenger", "rgb(8, 102, 255)"],
    ["instagram", "Via Instagram", "rgb(225, 48, 108)"],
    ["whatsapp", "Via WhatsApp", "rgb(37, 211, 102)"],
  ])("wears the brand colour of %s", (type, label, colour) => {
    const el = badge(type);

    expect(el.getAttribute("aria-label")).toBe(label);
    expect(el.style.backgroundColor).toBe(colour);
  });

  it.each(["email", "widget", "api", "slack", "something-new"])("keeps %s on the neutral disc", (type) => {
    expect(badge(type).style.backgroundColor).toBe("");
  });
});
