/*
 * The install snippet for each kind of site. Every one loads the same
 * script with the messenger's app id; they differ only in where the tag
 * goes and, on Next.js, in using the framework's own Script component so
 * the messenger loads once the page is interactive.
 */
import { installSnippet } from "@/lib/api/widget-settings";

export const frameworks = ["Next.js", "React", "Plain HTML", "Vue", "Other"];

/*
 * snippetFor returns the code to paste for the framework and where it
 * goes.
 */
export function snippetFor(framework: string, origin: string, widget: string): { code: string; hint: string } {
  const tag = installSnippet(origin, widget);
  if (framework === "Next.js") {
    return {
      code: [
        'import Script from "next/script";',
        "",
        `<Script src="${origin}/widget/v1/loader.js" data-app-id="${widget}" strategy="afterInteractive" />`,
      ].join("\n"),
      hint: "Add it inside <body> in app/layout.tsx, or in pages/_app.tsx.",
    };
  }
  if (framework === "React" || framework === "Vue") {
    return { code: tag, hint: "Paste it before the closing body tag in index.html." };
  }

  return { code: tag, hint: "Paste it before the closing body tag of every page." };
}
