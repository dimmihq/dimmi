/*
 * The emoji the composer's picker offers: a short built-in set, so the
 * messenger carries no emoji library. It is a chunk of its own, fetched
 * the first time a visitor opens the picker.
 */

const emoji =
  "😀 😃 😄 😁 😅 😂 🙂 😉 😊 😍 🥰 😘 😎 🤔 😐 😕 🙁 😢 😭 😡 😮 😴 🥳 🤯 👍 👎 👏 🙌 🙏 👋 💪 👀 ❤️ 🔥 ✨ 🎉 ✅ ❌ ⚠️ 💯";

/*
 * buttons is the picker's markup: one button per emoji.
 */
export function buttons(): string {
  return emoji
    .split(" ")
    .map(
      (each) =>
        `<button type="button" data-action="click->pick" data-emoji="${each}" aria-label="${each}">${each}</button>`,
    )
    .join("");
}

/*
 * insert puts the emoji in place of what is selected in the text box, or
 * at the cursor, and leaves the cursor after it.
 */
export function insert(input: HTMLTextAreaElement, value: string) {
  const start = input.selectionStart ?? input.value.length;
  const end = input.selectionEnd ?? start;
  input.value = input.value.slice(0, start) + value + input.value.slice(end);
  input.focus();
  input.setSelectionRange(start + value.length, start + value.length);
}
