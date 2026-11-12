import { createElement } from "react";
import { renderToString } from "react-dom/server";

import { DimmiButton, DimmiProvider } from "./index";

it("renders on a server without touching window", () => {
  const html = renderToString(
    createElement(DimmiProvider, { appId: "wgt_1", host: "https://d.test" }, createElement(DimmiButton, null, "Chat")),
  );
  expect(html).toBe('<button type="button">Chat</button>');
});
