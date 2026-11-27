import { getCollection } from "astro:content";
import type { APIRoute, GetStaticPaths } from "astro";

import { apiCategories, categoryMarkdown, operationMarkdown, operations } from "~docs/api/spec";
import { docMarkdown } from "~docs/markdown";

// Each docs page, API category and API endpoint as Markdown, at its path plus ".md".
export const getStaticPaths = (async () => {
  const docs = (await getCollection("docs")).filter((entry) => entry.id !== "404");
  return [
    ...docs.map((entry) => ({
      params: { slug: entry.id },
      props: { body: docMarkdown(entry, new URL(import.meta.env.SITE).origin) },
    })),
    ...apiCategories.map((category) => ({
      params: { slug: category.href.slice(1, -1) },
      props: { body: categoryMarkdown(category) },
    })),
    ...operations.map((op) => ({ params: { slug: op.href.slice(1, -1) }, props: { body: operationMarkdown(op) } })),
  ];
}) satisfies GetStaticPaths;

// Each copy ends with a pointer to the root index, so an agent that fetched
// one page can discover MCP, SDKs, and the rest of the docs.
export const GET: APIRoute = ({ props, site }) => {
  const origin = site?.origin ?? "https://trydimmi.com";
  return new Response(
    `${props.body as string}\n---\n\nIndex of all dimmi docs: ${origin}/llms.txt (full text: ${origin}/llms-full.txt)\n`,
    {
      headers: { "Content-Type": "text/markdown; charset=utf-8" },
    },
  );
};
