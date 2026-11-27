/*
 * Machine-readable catalog of every dimmi SDK, derived at build time from
 * each package's committed manifest in sdk/ (never hardcoded versions:
 * registries and mirrors own released versions; see AGENTS.md).
 */
import angularPkgRaw from "../../sdk/angular/package.json?raw";
import apiPkgRaw from "../../sdk/api/package.json?raw";
import browserPkgRaw from "../../sdk/browser/package.json?raw";
import goModRaw from "../../sdk/go/go.mod?raw";
import laravelComposerRaw from "../../sdk/laravel/composer.json?raw";
import nextjsPkgRaw from "../../sdk/nextjs/package.json?raw";
import phpComposerRaw from "../../sdk/php/composer.json?raw";
import pythonPyprojectRaw from "../../sdk/python/pyproject.toml?raw";
import reactPkgRaw from "../../sdk/react/package.json?raw";
import serverPkgRaw from "../../sdk/server/package.json?raw";
import sveltePkgRaw from "../../sdk/svelte/package.json?raw";
import vuePkgRaw from "../../sdk/vue/package.json?raw";
import { SDK_PACKAGES } from "../../site/src/sdks";
import { API_VERSION } from "./api/spec";
import { withBase } from "./base";
import { GITHUB_URL } from "./nav";

type NpmManifest = {
  name: string;
  description?: string;
  engines?: Record<string, string>;
  peerDependencies?: Record<string, string>;
};

type ComposerManifest = {
  name: string;
  description?: string;
  require?: Record<string, string>;
};

export type SdkEntry = {
  id: string;
  name: string;
  kind: "messenger" | "backend";
  ecosystem: "npm" | "go" | "packagist" | "pypi";
  install: string;
  runtime: string;
  description: string;
  docsPath: string;
  sourceUrl: string;
};

function parseNpm(raw: string): NpmManifest {
  return JSON.parse(raw) as NpmManifest;
}

function parseComposer(raw: string): ComposerManifest {
  return JSON.parse(raw) as ComposerManifest;
}

function parseGoMod(raw: string): { module: string; go: string } {
  const moduleMatch = /^module\s+(\S+)/m.exec(raw);
  const goMatch = /^go\s+(\S+)/m.exec(raw);
  return {
    module: moduleMatch?.[1] ?? "github.com/dimmihq/dimmi-go",
    go: goMatch?.[1] ?? "1.22",
  };
}

function parsePyproject(raw: string): { name: string; description: string; requiresPython: string } {
  const nameMatch = /^name\s*=\s*"([^"]+)"/m.exec(raw);
  const descMatch = /^description\s*=\s*"([^"]+)"/m.exec(raw);
  const pyMatch = /^requires-python\s*=\s*"([^"]+)"/m.exec(raw);
  return {
    name: nameMatch?.[1] ?? "dimmi-sdk",
    description: descMatch?.[1] ?? "Python backend SDK for dimmi.",
    requiresPython: pyMatch?.[1] ?? ">=3.10",
  };
}

function formatNpmRuntime(pkg: NpmManifest, fallback: string): string {
  const parts: string[] = [];
  if (pkg.peerDependencies) {
    for (const [dep, range] of Object.entries(pkg.peerDependencies)) {
      if (dep === "react" || dep === "next" || dep === "vue" || dep === "@angular/core" || dep === "svelte") {
        parts.push(`${dep} ${range}`);
      }
    }
  }
  if (pkg.engines?.node) {
    parts.push(`Node.js ${pkg.engines.node}`);
  }
  return parts.length > 0 ? parts.join(", ") : fallback;
}

const browserPkg = parseNpm(browserPkgRaw);
const reactPkg = parseNpm(reactPkgRaw);
const nextjsPkg = parseNpm(nextjsPkgRaw);
const vuePkg = parseNpm(vuePkgRaw);
const angularPkg = parseNpm(angularPkgRaw);
const sveltePkg = parseNpm(sveltePkgRaw);
const serverPkg = parseNpm(serverPkgRaw);
const apiPkg = parseNpm(apiPkgRaw);
const goMod = parseGoMod(goModRaw);
const phpComposer = parseComposer(phpComposerRaw);
const laravelComposer = parseComposer(laravelComposerRaw);
const pyproject = parsePyproject(pythonPyprojectRaw);

export const SDK_CATALOG: SdkEntry[] = [
  {
    id: "browser",
    name: browserPkg.name,
    kind: "messenger",
    ecosystem: "npm",
    install: `npm install ${browserPkg.name}`,
    runtime: "Any modern browser",
    description: browserPkg.description ?? "Browser messenger loader and typed JS API.",
    docsPath: "/developers/messenger/install.md",
    sourceUrl: `${GITHUB_URL}/tree/main/sdk/browser`,
  },
  {
    id: "react",
    name: reactPkg.name,
    kind: "messenger",
    ecosystem: "npm",
    install: `npm install ${reactPkg.name}`,
    runtime: formatNpmRuntime(reactPkg, "React 18+"),
    description: reactPkg.description ?? "React provider, hooks, and button for the dimmi messenger.",
    docsPath: "/developers/messenger/install.md",
    sourceUrl: `${GITHUB_URL}/tree/main/sdk/react`,
  },
  {
    id: "nextjs",
    name: nextjsPkg.name,
    kind: "messenger",
    ecosystem: "npm",
    install: `npm install ${nextjsPkg.name}`,
    runtime: formatNpmRuntime(nextjsPkg, "Next.js 14+, React 18+, Node.js 20+"),
    description: nextjsPkg.description ?? "Next.js App Router messenger and server-action identity helpers.",
    docsPath: "/developers/messenger/install.md",
    sourceUrl: `${GITHUB_URL}/tree/main/sdk/nextjs`,
  },
  {
    id: "vue",
    name: vuePkg.name,
    kind: "messenger",
    ecosystem: "npm",
    install: `npm install ${vuePkg.name}`,
    runtime: formatNpmRuntime(vuePkg, "Vue 3.3+"),
    description: vuePkg.description ?? "Vue 3 and Nuxt 3 plugin and composables for the dimmi messenger.",
    docsPath: "/developers/messenger/install.md",
    sourceUrl: `${GITHUB_URL}/tree/main/sdk/vue`,
  },
  {
    id: "angular",
    name: angularPkg.name,
    kind: "messenger",
    ecosystem: "npm",
    install: `npm install ${angularPkg.name}`,
    runtime: formatNpmRuntime(angularPkg, "Angular 17+"),
    description: angularPkg.description ?? "Angular provider and service for the dimmi messenger.",
    docsPath: "/developers/messenger/install.md",
    sourceUrl: `${GITHUB_URL}/tree/main/sdk/angular`,
  },
  {
    id: "svelte",
    name: sveltePkg.name,
    kind: "messenger",
    ecosystem: "npm",
    install: `npm install ${sveltePkg.name}`,
    runtime: formatNpmRuntime(sveltePkg, "Svelte 4+"),
    description: sveltePkg.description ?? "Svelte and SvelteKit store and action for the dimmi messenger.",
    docsPath: "/developers/messenger/install.md",
    sourceUrl: `${GITHUB_URL}/tree/main/sdk/svelte`,
  },
  {
    id: "server",
    name: serverPkg.name,
    kind: "backend",
    ecosystem: "npm",
    install: `npm install ${serverPkg.name}`,
    runtime: formatNpmRuntime(serverPkg, "Node.js 20+"),
    description: serverPkg.description ?? "Sign dimmi messenger identity tokens on Node.js, Bun, Deno, or workers.",
    docsPath: "/developers/messenger/identity-verification.md",
    sourceUrl: `${GITHUB_URL}/tree/main/sdk/server`,
  },
  {
    id: "api",
    name: apiPkg.name,
    kind: "backend",
    ecosystem: "npm",
    install: `npm install ${apiPkg.name}`,
    runtime: formatNpmRuntime(apiPkg, "Node.js 20+"),
    description: apiPkg.description ?? "Typed TypeScript client for the dimmi REST API.",
    docsPath: "/developers/sdks/typescript.md",
    sourceUrl: `${GITHUB_URL}/tree/main/sdk/api`,
  },
  {
    id: "go",
    name: goMod.module,
    kind: "backend",
    ecosystem: "go",
    install: `go get ${goMod.module}`,
    runtime: `Go >=${goMod.go}`,
    description: "Go backend SDK for dimmi: REST API client, identity tokens, and webhook verification.",
    docsPath: "/developers/sdks/go.md",
    sourceUrl: `${GITHUB_URL}/tree/main/sdk/go`,
  },
  {
    id: "php",
    name: phpComposer.name,
    kind: "backend",
    ecosystem: "packagist",
    install: `composer require ${phpComposer.name}`,
    runtime: `PHP ${phpComposer.require?.php ?? ">=8.2"}`,
    description:
      phpComposer.description ?? "PHP backend SDK for dimmi: REST API client, identity tokens, and webhooks.",
    docsPath: "/developers/sdks/php.md",
    sourceUrl: `${GITHUB_URL}/tree/main/sdk/php`,
  },
  {
    id: "laravel",
    name: laravelComposer.name,
    kind: "backend",
    ecosystem: "packagist",
    install: `composer require ${laravelComposer.name}`,
    runtime: `PHP ${laravelComposer.require?.php ?? ">=8.2"}, Laravel ${laravelComposer.require?.["illuminate/support"] ?? "10-12"}`,
    description:
      laravelComposer.description ??
      "Laravel integration for dimmi: facade, Blade messenger directive, and webhook routes.",
    docsPath: "/developers/sdks/laravel.md",
    sourceUrl: `${GITHUB_URL}/tree/main/sdk/laravel`,
  },
  {
    id: "python",
    name: pyproject.name,
    kind: "backend",
    ecosystem: "pypi",
    install: `pip install ${pyproject.name}`,
    runtime: `Python ${pyproject.requiresPython}`,
    description: pyproject.description,
    docsPath: "/developers/sdks/python.md",
    sourceUrl: `${GITHUB_URL}/tree/main/sdk/python`,
  },
];

const catalogIds = SDK_CATALOG.map((sdk) => sdk.id);
if (catalogIds.length !== SDK_PACKAGES.length || catalogIds.some((id, i) => id !== SDK_PACKAGES[i])) {
  throw new Error(
    `docs/src/sdks.ts SDK_CATALOG (${catalogIds.join(", ")}) does not match site/src/sdks.ts SDK_PACKAGES (${SDK_PACKAGES.join(", ")})`,
  );
}

export function sdksJson(origin: string) {
  return {
    api_version: API_VERSION,
    openapi: `${origin}/api/v1/openapi.json`,
    mcp: {
      url: `${origin}/mcp`,
      transport: "streamable-http",
      oauth_metadata: `${origin}/.well-known/oauth-authorization-server`,
      server_card: `${origin}${withBase("/mcp/server-card.json")}`,
    },
    install_skill: {
      name: "dimmi-install",
      docs: `${origin}${withBase("/developers/ai-coding-agent.md")}`,
      skill_md: "https://raw.githubusercontent.com/dimmihq/dimmi/main/sdk/skills/dimmi-install/SKILL.md",
      agents_snippet: "https://raw.githubusercontent.com/dimmihq/dimmi/main/sdk/skills/dimmi-install/AGENTS.snippet.md",
    },
    sdks: SDK_CATALOG.map((sdk) => ({
      id: sdk.id,
      package: sdk.name,
      kind: sdk.kind,
      ecosystem: sdk.ecosystem,
      install: sdk.install,
      runtime: sdk.runtime,
      description: sdk.description,
      docs: `${origin}${withBase(sdk.docsPath)}`,
      source: sdk.sourceUrl,
    })),
  };
}
