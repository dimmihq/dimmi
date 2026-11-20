/*
 * The self-host install, played as a terminal: the one command, then the
 * installer's real stages (prepare, configure, start, check) ticking off.
 */
import { motion } from "motion/react";

import { useScene } from "./use-scene";

const lines = [
  { kind: "cmd", text: "curl -fsSL …/install.sh | bash" },
  { kind: "ok", text: "Prepared the server: Docker, firewall, swap, security updates" },
  { kind: "ok", text: "Wrote settings to /opt/dimmi/.env" },
  { kind: "ok", text: "Pulled images, migrated the database, started dimmi" },
  { kind: "ok", text: "HTTPS ready for support.acme.dev" },
  { kind: "done", text: "dimmi is running at https://support.acme.dev" },
] as const;

export function TerminalScene() {
  const { ref, step } = useScene(lines.length, { interval: [1400, 900, 900, 1300, 900], hold: 4200 });

  return (
    <div
      ref={ref}
      className="w-full overflow-hidden rounded-t-xl border border-b-0 border-neutral-800 bg-neutral-950 shadow-[0_24px_60px_-28px_rgb(0_0_0/0.45)]"
    >
      <div className="flex items-center gap-1.5 border-b border-neutral-800 px-4 py-3">
        <span className="size-2.5 rounded-full bg-neutral-700" />
        <span className="size-2.5 rounded-full bg-neutral-700" />
        <span className="size-2.5 rounded-full bg-neutral-700" />
        <span className="ms-3 font-mono text-caption-1-regular text-neutral-500">root@support-1</span>
      </div>
      <div className="flex h-[312px] flex-col gap-2 px-5 py-5 font-mono text-[12.5px] leading-6">
        {lines.slice(0, step + 1).map((line) => (
          <motion.div
            key={line.text}
            initial={{ opacity: 0, x: -4 }}
            animate={{ opacity: 1, x: 0 }}
            transition={{ duration: 0.3 }}
            className={
              line.kind === "cmd" ? "text-neutral-100" : line.kind === "done" ? "text-accent-400" : "text-neutral-400"
            }
          >
            <span className="me-2 select-none text-neutral-600">
              {line.kind === "cmd" ? "$" : line.kind === "ok" ? "✓" : "→"}
            </span>
            {line.text}
          </motion.div>
        ))}
        {step < lines.length - 1 ? <span className="h-4 w-2 animate-pulse bg-neutral-500" /> : null}
      </div>
    </div>
  );
}
