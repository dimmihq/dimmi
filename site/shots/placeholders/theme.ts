import { readFileSync } from "node:fs";
import { resolve } from "node:path";

export type Scheme = "light" | "dark";

export type SceneSpec = {
  name: string;
  width: number;
  height: number;
  kind: "still" | "video";
  /** Duration in seconds for video scenes. */
  duration?: number;
  /** Timestamp in seconds to capture the static WebP poster for video scenes. */
  posterTime?: number;
  /** Returns full HTML document for the given color scheme. */
  html: (scheme: Scheme) => string;
};

const rootDir = resolve(process.cwd());
const interWoff2 = readFileSync(
  resolve(rootDir, "node_modules/@fontsource-variable/inter/files/inter-latin-wght-normal.woff2"),
).toString("base64");
const monoWoff2 = readFileSync(
  resolve(rootDir, "node_modules/@fontsource-variable/jetbrains-mono/files/jetbrains-mono-latin-wght-normal.woff2"),
).toString("base64");

const FONT_CSS = `
@font-face {
  font-family: "Inter Variable";
  src: url("data:font/woff2;base64,${interWoff2}") format("woff2");
  font-weight: 100 900;
  font-style: normal;
  font-display: block;
}
@font-face {
  font-family: "JetBrains Mono Variable";
  src: url("data:font/woff2;base64,${monoWoff2}") format("woff2");
  font-weight: 100 800;
  font-style: normal;
  font-display: block;
}
`;

export const BASE_CSS = `
${FONT_CSS}
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
:root {
  --bg: #ffffff;
  --bg-shell: #f5f5f5;
  --bg-sub: #f7f7f7;
  --bg-muted: #efefef;
  --bg-hover: #f0f0f0;
  --bg-bubble-in: #f4f4f5;
  --bg-bubble-out: #111827;
  --fg-bubble-out: #ffffff;
  --bg-note: #fefce8;
  --border-note: #fde047;
  --fg-note: #713f12;
  --border: #e5e5e5;
  --border-strong: #d4d4d4;
  --fg: #0a0a0a;
  --fg-2: #525252;
  --fg-3: #8a8a8a;
  --accent: #059669;
  --accent-bright: #10b981;
  --accent-soft: #ecfdf5;
  --accent-border: #a7f3d0;
  --accent-fg: #047857;
  --blue-soft: #eff6ff;
  --blue-fg: #1d4ed8;
  --blue-border: #bfdbfe;
  --purple-soft: #faf5ff;
  --purple-fg: #7e22ce;
  --purple-border: #e9d5ff;
  --orange-soft: #fff7ed;
  --orange-fg: #c2410c;
  --orange-border: #fed7aa;
  --lime-soft: #ecfccb;
  --lime-fg: #3f6212;
  --red-soft: #fef2f2;
  --red-fg: #b91c1c;
  --red-border: #fecaca;
  --amber-soft: #fffbeb;
  --amber-fg: #b45309;
  --amber-border: #fde68a;
  --shadow-panel: 0 1px 2px rgb(0 0 0 / 0.04), 0 8px 24px -12px rgb(0 0 0 / 0.08);
  --shadow-card: 0 1px 2px rgb(0 0 0 / 0.05);
  --shadow-float: 0 16px 40px -12px rgb(0 0 0 / 0.16), 0 0 0 1px rgb(0 0 0 / 0.06);
  --code-bg: #f8fafc;
  --code-fg: #0f172a;
}
html.dark {
  --bg: #121212;
  --bg-shell: #0a0a0a;
  --bg-sub: #1a1a1a;
  --bg-muted: #242424;
  --bg-hover: #262626;
  --bg-bubble-in: #1f1f22;
  --bg-bubble-out: #059669;
  --fg-bubble-out: #ffffff;
  --bg-note: rgba(113, 63, 18, 0.22);
  --border-note: rgba(234, 179, 8, 0.45);
  --fg-note: #fef08a;
  --border: #262626;
  --border-strong: #363636;
  --fg: #fafafa;
  --fg-2: #a3a3a3;
  --fg-3: #737373;
  --accent: #10b981;
  --accent-bright: #34d399;
  --accent-soft: rgba(16, 185, 129, 0.14);
  --accent-border: rgba(16, 185, 129, 0.34);
  --accent-fg: #6ee7b7;
  --blue-soft: rgba(59, 130, 246, 0.15);
  --blue-fg: #93c5fd;
  --blue-border: rgba(59, 130, 246, 0.32);
  --purple-soft: rgba(168, 85, 247, 0.16);
  --purple-fg: #d8b4fe;
  --purple-border: rgba(168, 85, 247, 0.34);
  --orange-soft: rgba(249, 115, 22, 0.16);
  --orange-fg: #fdba74;
  --orange-border: rgba(249, 115, 22, 0.34);
  --lime-soft: rgba(132, 204, 22, 0.16);
  --lime-fg: #bef264;
  --red-soft: rgba(239, 68, 68, 0.14);
  --red-fg: #fca5a5;
  --red-border: rgba(239, 68, 68, 0.32);
  --amber-soft: rgba(245, 158, 11, 0.15);
  --amber-fg: #fcd34d;
  --amber-border: rgba(245, 158, 11, 0.35);
  --shadow-panel: 0 1px 2px rgb(0 0 0 / 0.35), 0 10px 28px -10px rgb(0 0 0 / 0.65);
  --shadow-card: 0 1px 2px rgb(0 0 0 / 0.35);
  --shadow-float: 0 20px 48px -12px rgb(0 0 0 / 0.72), 0 0 0 1px rgb(255 255 255 / 0.08);
  --code-bg: #17171a;
  --code-fg: #e5e7eb;
}
html, body {
  width: 100%;
  height: 100%;
  overflow: hidden;
  background: var(--bg);
  color: var(--fg);
  font-family: "Inter Variable", -apple-system, BlinkMacSystemFont, sans-serif;
  -webkit-font-smoothing: antialiased;
  font-feature-settings: "cv02", "cv03", "cv04", "cv11";
}
.mono {
  font-family: "JetBrains Mono Variable", ui-monospace, monospace;
}
.app-shell {
  display: flex;
  width: 100%;
  height: 100%;
  background: var(--bg-shell);
  padding: 10px;
  gap: 10px;
}
.rail {
  width: 52px;
  flex-shrink: 0;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 12px 0;
  gap: 8px;
}
.rail-logo {
  width: 32px;
  height: 32px;
  border-radius: 9px;
  background: var(--bg-muted);
  border: 1px solid var(--border);
  color: var(--fg);
  font-weight: 700;
  font-size: 13px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 6px;
}
.rail-item {
  width: 34px;
  height: 34px;
  border-radius: 9px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--fg-3);
}
.rail-item.active {
  background: var(--accent);
  color: #ffffff;
  box-shadow: 0 2px 8px -2px rgba(16, 185, 129, 0.6);
}
.rail-spacer { flex: 1; }
.pane {
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  min-width: 0;
}
.pane-header {
  height: 54px;
  padding: 0 18px;
  border-bottom: 1px solid var(--border);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-shrink: 0;
}
.pill {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 2px 8px;
  border-radius: 999px;
  font-size: 11.5px;
  font-weight: 500;
  line-height: 18px;
  white-space: nowrap;
}
.pill-accent { background: var(--accent-soft); color: var(--accent-fg); border: 1px solid var(--accent-border); }
.pill-muted { background: var(--bg-sub); color: var(--fg-2); border: 1px solid var(--border); }
.pill-blue { background: var(--blue-soft); color: var(--blue-fg); border: 1px solid var(--blue-border); }
.pill-purple { background: var(--purple-soft); color: var(--purple-fg); border: 1px solid var(--purple-border); }
.pill-orange { background: var(--orange-soft); color: var(--orange-fg); border: 1px solid var(--orange-border); }
.pill-red { background: var(--red-soft); color: var(--red-fg); border: 1px solid var(--red-border); }
.pill-amber { background: var(--amber-soft); color: var(--amber-fg); border: 1px solid var(--amber-border); }
.pill-lime { background: var(--lime-soft); color: var(--lime-fg); }
.chip-ai {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 1px 6px;
  border-radius: 5px;
  font-size: 10.5px;
  font-weight: 600;
  letter-spacing: 0.02em;
  background: var(--accent-soft);
  color: var(--accent-fg);
  border: 1px solid var(--accent-border);
}
.avatar {
  width: 28px;
  height: 28px;
  border-radius: 999px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 11.5px;
  font-weight: 600;
  flex-shrink: 0;
}
.avatar-sm { width: 22px; height: 22px; font-size: 10px; }
.avatar-lg { width: 34px; height: 34px; font-size: 13px; }
.av-emerald { background: #d1fae5; color: #065f46; }
.av-sky { background: #e0f2fe; color: #075985; }
.av-amber { background: #fef3c7; color: #92400e; }
.av-rose { background: #ffe4e6; color: #9f1239; }
.av-violet { background: #ede9fe; color: #5b21b6; }
.av-lime { background: #ecfccb; color: #3f6212; }
.av-slate { background: #e2e8f0; color: #334155; }
html.dark .av-emerald { background: #064e3b; color: #6ee7b7; }
html.dark .av-sky { background: #0c4a6e; color: #7dd3fc; }
html.dark .av-amber { background: #78350f; color: #fde68a; }
html.dark .av-rose { background: #881337; color: #fda4af; }
html.dark .av-violet { background: #4c1d95; color: #ddd6fe; }
html.dark .av-lime { background: #365314; color: #bef264; }
html.dark .av-slate { background: #334155; color: #e2e8f0; }

.btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 7px 13px;
  border-radius: 9px;
  font-size: 13px;
  font-weight: 500;
  border: 1px solid var(--border);
  background: var(--bg);
  color: var(--fg);
  box-shadow: var(--shadow-card);
  white-space: nowrap;
}
.btn-primary {
  background: linear-gradient(180deg, #10b981 0%, #059669 100%);
  border-color: #059669;
  color: #ffffff;
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 0.2), 0 1px 2px rgb(0 0 0 / 0.12);
}
.btn-sm { padding: 5px 10px; font-size: 12px; border-radius: 7px; }
.switch {
  width: 34px;
  height: 20px;
  border-radius: 999px;
  background: var(--border-strong);
  position: relative;
  flex-shrink: 0;
}
.switch.on { background: var(--accent); }
.switch::after {
  content: "";
  position: absolute;
  top: 2px;
  left: 2px;
  width: 16px;
  height: 16px;
  border-radius: 999px;
  background: #ffffff;
  box-shadow: 0 1px 2px rgb(0 0 0 / 0.2);
}
.switch.on::after { left: 16px; }
.card {
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 14px 16px;
}
.card-sub {
  background: var(--bg-sub);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 14px 16px;
}
.dot { width: 7px; height: 7px; border-radius: 999px; display: inline-block; flex-shrink: 0; }
.dot-green { background: #10b981; }
.dot-amber { background: #f59e0b; }
.dot-red { background: #ef4444; }
.dot-blue { background: #3b82f6; }

/* Frame-level deterministic helpers */
.anim-pop {
  opacity: 0;
  transform: translateY(10px) scale(0.985);
}
.anim-pop.show {
  opacity: 1;
  transform: translateY(0) scale(1);
}
.anim-fade { opacity: 0; }
.anim-fade.show { opacity: 1; }
`;

export function doc(scheme: Scheme, body: string, extraCss = "", script = ""): string {
  return `<!doctype html>
<html lang="en" class="${scheme === "dark" ? "dark" : ""}">
<head>
<meta charset="utf-8" />
<style>${BASE_CSS}\n${extraCss}</style>
</head>
<body>
${body}
${script ? `<script>${script}</script>` : ""}
</body>
</html>`;
}

export const ICONS = {
  inbox: `<svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 16 12 14 15 10 15 8 12 2 12"/><path d="M5.45 5.11 2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z"/></svg>`,
  sparkle: `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m12 3-1.9 5.8a2 2 0 0 1-1.3 1.3L3 12l5.8 1.9a2 2 0 0 1 1.3 1.3L12 21l1.9-5.8a2 2 0 0 1 1.3-1.3L21 12l-5.8-1.9a2 2 0 0 1-1.3-1.3Z"/></svg>`,
  clock: `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="9"/><polyline points="12 7 12 12 15 14"/></svg>`,
  book: `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 19.5v-15A2.5 2.5 0 0 1 6.5 2H20v20H6.5a2.5 2.5 0 0 1 0-5H20"/></svg>`,
  users: `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>`,
  settings: `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9c.26.6.85 1 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>`,
  search: `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/></svg>`,
  workflow: `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="6" height="6" rx="1"/><rect x="15" y="15" width="6" height="6" rx="1"/><path d="M9 6h4a2 2 0 0 1 2 2v7"/></svg>`,
  check: `<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>`,
  lock: `<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="11" width="18" height="11" rx="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>`,
  mail: `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="4" width="20" height="16" rx="2"/><path d="m22 7-8.97 5.7a1.94 1.94 0 0 1-2.06 0L2 7"/></svg>`,
  chat: `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>`,
  gmail: `<svg width="22" height="22" viewBox="0 0 28 28" aria-hidden="true"><path fill="#4285F4" d="M3.5 22.25h4.2V12.5L2 8.2v12.55c0 .83.67 1.5 1.5 1.5Z"/><path fill="#34A853" d="M20.3 22.25h4.2c.83 0 1.5-.67 1.5-1.5V8.2l-5.7 4.3v9.75Z"/><path fill="#FBBC04" d="M20.3 7.25v5.25L26 8.2V6.7c0-1.85-2.12-2.9-3.6-1.8l-2.1 2.35Z"/><path fill="#EA4335" d="M7.7 12.5V7.25L14 12l6.3-4.75v5.25L14 17.25 7.7 12.5Z"/><path fill="#C5221F" d="M2 6.7v1.5l5.7 4.3V7.25L5.6 4.9C4.12 3.8 2 4.85 2 6.7Z"/></svg>`,
  slack: `<svg width="16" height="16" viewBox="0 0 122.8 122.8" aria-hidden="true"><path fill="#E01E5A" d="M25.8 77.6c0 7.1-5.8 12.9-12.9 12.9S0 84.7 0 77.6s5.8-12.9 12.9-12.9h12.9v12.9zm6.5 0c0-7.1 5.8-12.9 12.9-12.9s12.9 5.8 12.9 12.9v32.3c0 7.1-5.8 12.9-12.9 12.9s-12.9-5.8-12.9-12.9V77.6z"/><path fill="#36C5F0" d="M45.2 25.8c-7.1 0-12.9-5.8-12.9-12.9S38.1 0 45.2 0s12.9 5.8 12.9 12.9v12.9H45.2zm0 6.5c7.1 0 12.9 5.8 12.9 12.9s-5.8 12.9-12.9 12.9H12.9C5.8 58.1 0 52.3 0 45.2s5.8-12.9 12.9-12.9h32.3z"/><path fill="#2EB67D" d="M97 45.2c0-7.1 5.8-12.9 12.9-12.9s12.9 5.8 12.9 12.9-5.8 12.9-12.9 12.9H97V45.2zm-6.5 0c0 7.1-5.8 12.9-12.9 12.9s-12.9-5.8-12.9-12.9V12.9C64.7 5.8 70.5 0 77.6 0s12.9 5.8 12.9 12.9v32.3z"/><path fill="#ECB22E" d="M77.6 97c7.1 0 12.9 5.8 12.9 12.9s-5.8 12.9-12.9 12.9-12.9-5.8-12.9-12.9V97h12.9zm0-6.5c-7.1 0-12.9-5.8-12.9-12.9s5.8-12.9 12.9-12.9h32.3c7.1 0 12.9 5.8 12.9 12.9s-5.8 12.9-12.9 12.9H77.6z"/></svg>`,
  whatsapp: `<svg width="16" height="16" viewBox="0 0 28 28" aria-hidden="true"><path fill="#25D366" d="M14.04 2C7.42 2 2.04 7.36 2.04 13.96c0 2.11.55 4.17 1.6 5.99L2 26l6.23-1.63a11.96 11.96 0 0 0 5.81 1.48h.01c6.61 0 11.99-5.36 11.99-11.96A11.98 11.98 0 0 0 14.04 2Z"/><path fill="#ffffff" d="M19.86 16.92c-.3-.15-1.78-.88-2.06-.98-.28-.1-.48-.15-.68.15-.2.3-.78.98-.95 1.18-.18.2-.35.23-.65.08-.3-.15-1.27-.47-2.42-1.49-.9-.8-1.5-1.79-1.68-2.09-.18-.3-.02-.46.13-.61.14-.14.3-.35.45-.53.15-.18.2-.3.3-.5.1-.2.05-.38-.03-.53-.08-.15-.68-1.63-.93-2.23-.24-.59-.49-.5-.68-.51h-.58c-.2 0-.53.08-.8.38-.28.3-1.05 1.03-1.05 2.51s1.08 2.91 1.23 3.11c.15.2 2.12 3.24 5.14 4.54.72.31 1.28.5 1.72.64.72.23 1.38.2 1.9.12.58-.09 1.78-.73 2.03-1.43.25-.7.25-1.3.18-1.43-.08-.13-.28-.2-.58-.35Z"/></svg>`,
  instagram: `<svg width="16" height="16" viewBox="0 0 28 28" aria-hidden="true"><defs><radialGradient id="ig-g" cx="28%" cy="105%" r="130%"><stop offset="0%" stop-color="#FEDA75"/><stop offset="25%" stop-color="#FA7E1E"/><stop offset="55%" stop-color="#D62976"/><stop offset="80%" stop-color="#962FBF"/><stop offset="100%" stop-color="#4F5BD5"/></radialGradient></defs><rect x="2" y="2" width="24" height="24" rx="6.5" fill="url(#ig-g)"/><rect x="6.5" y="6.5" width="15" height="15" rx="4.2" fill="none" stroke="#ffffff" stroke-width="1.9"/><circle cx="14" cy="14" r="3.6" fill="none" stroke="#ffffff" stroke-width="1.9"/><circle cx="18.3" cy="9.7" r="1.15" fill="#ffffff"/></svg>`,
  messenger: `<svg width="16" height="16" viewBox="0 0 28 28" aria-hidden="true"><defs><linearGradient id="fbm-g" x1="50%" y1="0%" x2="50%" y2="100%"><stop offset="0%" stop-color="#00B2FF"/><stop offset="100%" stop-color="#006AFF"/></linearGradient></defs><path fill="url(#fbm-g)" d="M14 2C7.24 2 2 6.96 2 13.64c0 3.49 1.43 6.5 3.76 8.58.2.17.32.42.33.68l.07 2.13c.02.68.72 1.12 1.35.84l2.38-1.05c.2-.09.43-.11.65-.05 1.09.3 2.25.46 3.46.46 6.76 0 12-4.96 12-11.6C26 6.96 20.76 2 14 2Z"/><path fill="#ffffff" d="m21.2 10.67-3.52 5.58a1.8 1.8 0 0 1-2.6.48l-2.81-2.11a.72.72 0 0 0-.87 0l-3.8 2.88c-.51.39-1.17-.22-.83-.76l3.52-5.58a1.8 1.8 0 0 1 2.6-.48l2.81 2.11a.72.72 0 0 0 .87 0l3.8-2.88c.51-.39 1.17.22.83.76Z"/></svg>`,
  code: `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/></svg>`,
  gitPr: `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="18" cy="18" r="3"/><circle cx="6" cy="6" r="3"/><path d="M13 6h3a2 2 0 0 1 2 2v7"/><line x1="6" y1="9" x2="6" y2="21"/></svg>`,
  shield: `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><path d="m9 12 2 2 4-4"/></svg>`,
  paperclip: `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m21.44 11.05-9.19 9.19a6 6 0 0 1-8.49-8.49l8.57-8.57A4 4 0 1 1 18 8.84l-8.59 8.57a2 2 0 0 1-2.83-2.83l8.49-8.48"/></svg>`,
  send: `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m22 2-7 20-4-9-9-4Z"/><path d="M22 2 11 13"/></svg>`,
  copy: `<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>`,
};

export function renderRail(active: "inbox" | "agents" | "routines" | "help" | "settings"): string {
  const items: { id: string; icon: string }[] = [
    { id: "search", icon: ICONS.search },
    { id: "inbox", icon: ICONS.inbox },
    { id: "agents", icon: ICONS.sparkle },
    { id: "workflow", icon: ICONS.workflow },
    { id: "routines", icon: ICONS.clock },
    { id: "users", icon: ICONS.users },
    { id: "help", icon: ICONS.book },
  ];
  return `<aside class="rail">
    <div class="rail-logo">S</div>
    ${items.map((item) => `<div class="rail-item ${item.id === active ? "active" : ""}">${item.icon}</div>`).join("")}
    <div class="rail-spacer"></div>
    <div class="rail-item ${active === "settings" ? "active" : ""}">${ICONS.settings}</div>
    <div class="avatar avatar-sm av-sky">M</div>
  </aside>`;
}

export function renderDetailsSidebar(opts: {
  assignee: string;
  assigneeAv: string;
  assigneeInitial: string;
  inbox: string;
  channel: string;
  sla?: string;
  customer: string;
  customerRole: string;
  customerEmail: string;
  customerAv: string;
  customerInitial: string;
  attrs: [string, string][];
  past?: { channel: string; title: string; time: string }[];
}): string {
  return `<aside class="pane" style="width: 318px; flex-shrink: 0;">
    <div class="pane-header">
      <span style="font-size: 14px; font-weight: 600;">Details</span>
      <span class="btn btn-sm">Actions ▾</span>
    </div>
    <div style="padding: 16px 18px; display: flex; flex-direction: column; gap: 14px; font-size: 13px;">
      <div style="display: flex; flex-direction: column; gap: 10px;">
        <div style="display: flex; align-items: center; justify-content: space-between;">
          <span style="color: var(--fg-3);">Assignee</span>
          <span class="pill pill-muted"><span class="avatar avatar-sm ${opts.assigneeAv}" style="width:16px;height:16px;font-size:9px;">${opts.assigneeInitial}</span>${opts.assignee}</span>
        </div>
        <div style="display: flex; align-items: center; justify-content: space-between;">
          <span style="color: var(--fg-3);">Status</span>
          <span class="pill pill-accent"><span class="dot dot-green"></span>Open</span>
        </div>
        <div style="display: flex; align-items: center; justify-content: space-between;">
          <span style="color: var(--fg-3);">Inbox</span>
          <span style="font-weight: 500;">${opts.inbox}</span>
        </div>
        <div style="display: flex; align-items: center; justify-content: space-between;">
          <span style="color: var(--fg-3);">Channel</span>
          <span class="mono" style="font-size: 11.5px; color: var(--fg-2);">${opts.channel}</span>
        </div>
        ${
          opts.sla
            ? `<div style="display: flex; align-items: center; justify-content: space-between;">
          <span style="color: var(--fg-3);">SLA target</span>
          <span class="pill pill-amber">${ICONS.clock} ${opts.sla}</span>
        </div>`
            : ""
        }
      </div>
      <div style="height: 1px; background: var(--border);"></div>
      <div style="display: flex; flex-direction: column; gap: 10px;">
        <div style="font-size: 12px; font-weight: 600; color: var(--fg-2);">Customer</div>
        <div style="display: flex; align-items: center; gap: 10px;">
          <span class="avatar avatar-lg ${opts.customerAv}">${opts.customerInitial}</span>
          <div style="min-width: 0;">
            <div style="font-weight: 600; font-size: 13.5px;">${opts.customer}</div>
            <div style="font-size: 12px; color: var(--fg-3);">${opts.customerRole}</div>
          </div>
        </div>
        <div style="display: flex; align-items: center; justify-content: space-between; font-size: 12.5px;">
          <span style="color: var(--fg-3);">Email</span>
          <span style="color: var(--fg-2); font-weight: 500;">${opts.customerEmail}</span>
        </div>
        ${opts.attrs
          .map(
            ([k, v]) =>
              `<div style="display: flex; align-items: center; justify-content: space-between; font-size: 12.5px;"><span style="color: var(--fg-3);">${k}</span><span class="mono" style="font-size: 11.5px; color: var(--fg);">${v}</span></div>`,
          )
          .join("")}
      </div>
      ${
        opts.past && opts.past.length > 0
          ? `<div style="height: 1px; background: var(--border);"></div>
      <div style="display: flex; flex-direction: column; gap: 8px;">
        <div style="font-size: 12px; font-weight: 600; color: var(--fg-2);">Past conversations</div>
        ${opts.past
          .map(
            (
              p,
            ) => `<div style="padding: 8px 10px; border-radius: 9px; background: var(--bg-sub); border: 1px solid var(--border); display: flex; flex-direction: column; gap: 2px;">
            <div style="display: flex; justify-content: space-between; align-items: center; gap: 8px;">
              <span style="font-size: 12px; font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">${p.title}</span>
              <span style="font-size: 10.5px; color: var(--fg-3); flex-shrink: 0;">${p.time}</span>
            </div>
            <span style="font-size: 11px; color: var(--fg-3);">${p.channel}</span>
          </div>`,
          )
          .join("")}
      </div>`
          : ""
      }
    </div>
  </aside>`;
}

/** Deterministic animation helper injected into video scenes. */
export const TIMELINE_SCRIPT_HELPER = `
function clamp01(x) { return x < 0 ? 0 : x > 1 ? 1 : x; }
function easeOut(t) { t = clamp01(t); return 1 - Math.pow(1 - t, 3); }
function showPop(id, t, start, dur) {
  const el = document.getElementById(id);
  if (!el) return;
  const p = easeOut((t - start) / (dur || 0.35));
  el.style.opacity = String(p);
  el.style.transform = 'translateY(' + ((1 - p) * 10).toFixed(2) + 'px) scale(' + (0.985 + p * 0.015).toFixed(4) + ')';
}
function showFade(id, t, start, dur) {
  const el = document.getElementById(id);
  if (!el) return;
  const p = easeOut((t - start) / (dur || 0.28));
  el.style.opacity = String(p);
}
function typeText(id, fullText, t, start, end) {
  const el = document.getElementById(id);
  if (!el) return;
  if (t < start) { el.textContent = ''; return; }
  const p = clamp01((t - start) / (end - start));
  const count = Math.round(p * fullText.length);
  el.textContent = fullText.slice(0, count);
}
function setDisplay(id, visible) {
  const el = document.getElementById(id);
  if (!el) return;
  el.style.display = visible ? '' : 'none';
}
`;
