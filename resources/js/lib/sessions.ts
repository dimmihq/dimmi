/*
 * Session recordings of the dashboard. The server's page names a kimmi
 * recorder and its project's public key; a page that names none records
 * nothing, as in development, tests and any server whose operator named no
 * recorder. The recorder loads from its own host only once the dashboard
 * knows who is signed in and in which workspace. It names the teammate by
 * workspace and user id, as "ws_1:usr_42", so each workspace's recordings
 * stand apart, and learns their name and the workspace's: never their email,
 * never anything about customers. Its project's rules mask every character
 * of text and every input in the browser before anything is sent. Signing
 * out stops it.
 */

export type Recorder = { host: string; key: string };

/** The signed-in teammate and the workspace they are in. */
export type Teammate = {
  id: string;
  name: string;
  workspace: { id: string; name: string };
};

type Traits = { name?: string; [key: string]: unknown };

type Kimmi = { identify(id: string, traits?: Traits): void; stop(): void };

declare global {
  interface Window {
    kimmi?: Kimmi;
  }
}

/** The recorder the page names, or none. */
export function readRecorder(doc: Document = document): Recorder | null {
  const meta = (name: string) =>
    doc.querySelector<HTMLMetaElement>(`meta[name="dimmi-sessions-${name}"]`)?.content ?? "";
  const host = meta("host");
  const key = meta("key");
  if (!host || !key) {
    return null;
  }

  try {
    const url = new URL(host);

    return url.protocol === "https:" || url.protocol === "http:" ? { host: url.origin, key } : null;
  } catch {
    return null;
  }
}

/** The member the recorder names once it has loaded. */
let member: Teammate | null = null;

/** Who the running recorder last named, so a repeat names no one twice. */
let named = "";

function identify(kimmi: Kimmi, teammate: Teammate): void {
  const { id, name, workspace } = teammate;
  const key = JSON.stringify([id, name, workspace.id, workspace.name]);
  if (key === named) {
    return;
  }
  named = key;
  kimmi.identify(`${workspace.id}:${id}`, { name, workspaceId: workspace.id, workspaceName: workspace.name });
}

/**
 * Records the signed-in member's sessions when the page names a recorder,
 * loading it the first time and naming the member, again whenever they or
 * their workspace change.
 */
export function recordSession(teammate: Teammate, doc: Document = document, win: Window = window): void {
  const recorder = readRecorder(doc);
  if (!recorder || !teammate.id || !teammate.workspace.id) {
    return;
  }

  member = teammate;
  if (win.kimmi) {
    identify(win.kimmi, teammate);
    return;
  }
  if (doc.querySelector("script[data-dimmi-sessions]")) {
    return;
  }

  const script = doc.createElement("script");
  script.src = `${recorder.host}/kimmi.js`;
  script.async = true;
  script.dataset.key = recorder.key;
  script.dataset.host = recorder.host;
  script.dataset.dimmiSessions = "";
  script.addEventListener(
    "load",
    () => {
      if (!win.kimmi) {
        return;
      }
      if (member) {
        identify(win.kimmi, member);
      } else {
        win.kimmi.stop();
      }
    },
    { once: true },
  );
  doc.head.append(script);
}

/** Stops recording, as when the member signs out. */
export function stopRecording(win: Window = window): void {
  member = null;
  named = "";
  win.kimmi?.stop();
}
