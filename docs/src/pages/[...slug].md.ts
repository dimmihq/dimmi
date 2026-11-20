import { getCollection } from "astro:content";
import type { APIRoute, GetStaticPaths } from "astro";

import { operationMarkdown, operations } from "~docs/api/spec";
import { withBase } from "~docs/base";
import { docMarkdown } from "~docs/markdown";

// Each docs page and API endpoint as Markdown, at its path plus ".md".
export const getStaticPaths = (async () => {
  const docs = (await getCollection("docs")).filter((entry) => entry.id !== "404");
  return [
    ...docs.map((entry) => ({
      params: { slug: entry.id },
      props: { body: docMarkdown(entry, new URL(import.meta.env.SITE).origin) },
    })),
    ...operations.map((op) => ({ params: { slug: op.href.slice(1, -1) }, props: { body: operationMarkdown(op) } })),
  ];
}) satisfies GetStaticPaths;

// Each copy ends with a pointer to the index, so an agent that fetched one
// page can find the rest.
export const GET: APIRoute = ({ props, site }) =>
  new Response(
    `${props.body as string}\n---\n\nIndex of all dimmi docs: ${site?.origin ?? ""}${withBase("/llms.txt")}\n`,
    {
      headers: { "Content-Type": "text/markdown; charset=utf-8" },
    },
  );
