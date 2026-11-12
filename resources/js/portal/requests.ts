/*
 * A request form's fields that show only once another field has a value,
 * one of a listed few when the field lists any. Hidden fields are left
 * out of what the form sends, so a value typed before the condition
 * changed never reaches the team.
 */

function valuesOf(form: HTMLFormElement, key: string): string[] {
  const data = new FormData(form);
  return data
    .getAll(`field.${key}`)
    .map(String)
    .filter((value) => value !== "");
}

export function showWhen(root: ParentNode): void {
  for (const form of root.querySelectorAll<HTMLFormElement>("[data-request-form]")) {
    for (const field of form.querySelectorAll<HTMLElement>("[data-show-field]")) {
      const other = field.dataset.showField ?? "";
      if (!other) {
        continue;
      }
      const equals = (field.dataset.showEquals ?? "").split("\n").filter(Boolean);
      const values = valuesOf(form, other);
      const shown = values.length > 0 && (equals.length === 0 || values.some((value) => equals.includes(value)));
      field.hidden = !shown;
      for (const input of field.querySelectorAll<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>(
        "input, select, textarea",
      )) {
        input.disabled = !shown;
      }
    }
  }
}

document.addEventListener("input", (event) => {
  const form = (event.target as Element | null)?.closest?.("[data-request-form]");
  if (form) {
    showWhen(form.parentNode ?? document);
  }
});
