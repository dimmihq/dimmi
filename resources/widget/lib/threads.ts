/*
 * The thread's bubbles, cards and their stylesheet are a chunk of their
 * own: a visitor who never opens a conversation never fetches them. The
 * messenger asks for the chunk as soon as the panel opens or a
 * conversation is shown, and draws a thread through it once it arrived;
 * until then the conversation shows its skeleton.
 */
export type Parts = typeof import("./thread_parts");

let parts: Parts | undefined;
let loading: Promise<Parts> | undefined;

/*
 * thread returns the loaded chunk, or nothing while it is on its way.
 */
export function thread(): Parts | undefined {
  return parts;
}

/*
 * loadThread fetches the chunk once and adopts its stylesheet into the
 * messenger's shadow root.
 */
export function loadThread(root: ShadowRoot): Promise<Parts> {
  loading ??= import("./thread_parts").then((loaded) => (parts = loaded));

  return loading.then((loaded) => {
    loaded.adopt(root);
    return loaded;
  });
}
