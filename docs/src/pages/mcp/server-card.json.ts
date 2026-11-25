import type { APIRoute } from "astro";

import { API_VERSION } from "~docs/api/spec";
import { withBase } from "~docs/base";

// Official MCP Server Card describing dimmi's remote Streamable HTTP MCP server.
export const GET: APIRoute = ({ site }) => {
  const origin = site?.origin ?? "https://trydimmi.com";
  const card = {
    $schema: "https://static.modelcontextprotocol.io/schemas/v1/server-card.schema.json",
    name: "com.trydimmi/workspace",
    title: "dimmi",
    description:
      "Connect an AI assistant to your dimmi workspace to search knowledge, manage conversations, contacts, tickets, help articles, and procedures, or propose reviewable Operator edits.",
    version: API_VERSION,
    websiteUrl: `${origin}${withBase("/developers/mcp/")}`,
    icons: [{ src: `${origin}/favicon.svg`, mimeType: "image/svg+xml", sizes: ["any"] }],
    remotes: [
      {
        type: "streamable-http",
        url: `${origin}/mcp`,
        authentication: {
          type: "oauth2",
          authorizationServerMetadataUrl: `${origin}/.well-known/oauth-authorization-server`,
        },
      },
    ],
  };

  return new Response(`${JSON.stringify(card, null, 2)}\n`, {
    headers: { "Content-Type": "application/json; charset=utf-8" },
  });
};
