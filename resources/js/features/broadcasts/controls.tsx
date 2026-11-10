/*
 * The controls the composer's rows use: a pick of one stored thing by
 * its id, and a switch, both held by the draft rather than by
 * themselves, so a value the server sent back shows as it is.
 */
import { Select, SelectItem } from "@/components/base/select/select";
import { Switch } from "@/components/base/switch/switch";

/*
 * Option is one choice of a Pick: the id it saves and the name people
 * read.
 */
export interface Option {
  id: string;
  name: string;
}

/*
 * none is the id of the choice that clears a pick.
 */
const none = "none";

/*
 * Pick chooses one option by id, telling onChange the id picked, or
 * null for the clearing choice. placeholder names the clearing choice
 * when there is one.
 */
export function Pick({
  label,
  options,
  value,
  placeholder,
  disabled = false,
  onChange,
}: {
  label: string;
  options: Option[];
  value: string | null;
  placeholder?: string;
  disabled?: boolean;
  onChange: (id: string | null) => void;
}) {
  const all = placeholder ? [{ id: none, name: placeholder }, ...options] : options;

  return (
    <Select
      aria-label={label}
      size="sm"
      isDisabled={disabled}
      selectedKey={value ?? (placeholder ? none : null)}
      onSelectionChange={(key) => onChange(key === null || key === none ? null : String(key))}
      popoverClassName="w-64"
    >
      {all.map((option) => (
        <SelectItem key={option.id} id={option.id} textValue={option.name}>
          {option.name}
        </SelectItem>
      ))}
    </Select>
  );
}

/*
 * Flag switches a setting the draft holds.
 */
export function Flag({ label, on, onChange }: { label: string; on: boolean; onChange: (on: boolean) => void }) {
  return <Switch aria-label={label} size="sm" isSelected={on} onChange={onChange} />;
}
