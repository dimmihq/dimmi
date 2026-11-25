import type { APIRoute } from "astro";

import { sdksJson } from "~docs/sdks";

// Machine-readable catalog of every dimmi SDK, OpenAPI contract, MCP endpoint,
// and coding-agent skill (linked from llms.txt).
export const GET: APIRoute = ({ site }) => {
  const origin = site?.origin ?? "https://trydimmi.com";
  return new Response(`${JSON.stringify(sdksJson(origin), null, 2)}\n`, {
    headers: { "Content-Type": "application/json; charset=utf-8" },
  });
};
