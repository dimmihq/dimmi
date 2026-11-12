/*
 * How i18next-cli reads the browser app: which files hold text, where each
 * language's messages live, and which keys are built at run time and must
 * be kept although no call names them, such as an audit event's label.
 * English is the source; the pseudo locale is generated from it by
 * scripts/pseudo-locale.ts and never checked in. BoardUI's own folders
 * and the tests hold no product text.
 */
import { defineConfig } from "i18next-cli";

export default defineConfig({
  locales: ["en"],
  extract: {
    input: ["resources/js/**/*.{ts,tsx}"],
    ignore: [
      "resources/js/components/**",
      "resources/js/styles/**",
      "resources/js/utils/**",
      "resources/js/**/*.test.{ts,tsx}",
      "resources/js/**/test-utils.tsx",
      "resources/js/route-tree.gen.ts",
    ],
    output: "resources/js/lang/{{language}}/{{namespace}}.json",
    primaryLanguage: "en",
    defaultNS: "common",
    nsSeparator: ":",
    keySeparator: ".",
    functions: ["t", "*.t"],
    preservePatterns: [
      "settings:audit.*",
      "settings:security.*",
      "settings:channels.*",
      "trackers:message.step.*",
      "trackers:results.*",
      "trackers:reasons.*",
      "trackers:reports.sla.targets.*",
      "trackers:incident.severityHint.*",
    ],
    sort: true,
    indentation: 2,
  },
});
