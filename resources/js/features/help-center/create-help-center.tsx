/*
 * Adding a help center: its name, the language its pages speak and its
 * address. The address follows the name until the team edits it; left
 * as suggested, the server keeps it unique with a short suffix when the
 * name's address is taken. The new help center becomes the one the
 * dashboard works on.
 */
import { RiBookOpenLine } from "@remixicon/react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { Notification } from "@/components/base/notification/notification";
import { Select, SelectItem } from "@/components/base/select/select";
import { createHelpCenter } from "@/lib/api/help-center-sites";
import { helpCentersQuery } from "@/lib/api/help-centers";

import { pickHelpCenter } from "./use-help-center-pick";

/*
 * languages are the languages a help center's pages can speak.
 */
const languages = [
  { id: "en", name: "English" },
  { id: "fr", name: "French" },
  { id: "de", name: "German" },
  { id: "es", name: "Spanish" },
  { id: "pt-BR", name: "Portuguese (Brazil)" },
  { id: "it", name: "Italian" },
  { id: "nl", name: "Dutch" },
  { id: "ja", name: "Japanese" },
];

/*
 * suggest turns a name into the address it suggests: lower case letters,
 * digits and single dashes, at most 40 characters.
 */
function suggest(name: string): string {
  return name
    .normalize("NFKD")
    .replace(/[̀-ͯ]/g, "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 40)
    .replace(/-+$/g, "");
}

export function CreateHelpCenterDialog({ workspace, onClose }: { workspace: string; onClose: () => void }) {
  const queryClient = useQueryClient();
  const [name, setName] = useState("");
  const [subdomain, setSubdomain] = useState<string | null>(null);
  const [locale, setLocale] = useState("en");
  const address = subdomain ?? suggest(name);
  const create = useMutation({
    mutationFn: () => createHelpCenter(workspace, { name: name.trim(), locale, ...(subdomain ? { subdomain } : {}) }),
    onSuccess: async (made) => {
      await queryClient.invalidateQueries({ queryKey: helpCentersQuery(workspace).queryKey });
      pickHelpCenter(workspace, made.id);
      onClose();
    },
  });
  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    create.mutate();
  };

  return (
    <Modal isOpen size="md" onOpenChange={(open) => !open && onClose()}>
      <form onSubmit={submit} className="flex min-h-0 flex-col gap-4">
        <ModalHeader
          icon={RiBookOpenLine}
          title="New help center"
          description="Another set of docs at its own address, such as one per product or language."
        />
        <ModalBody className="flex flex-col gap-4">
          {create.error ? (
            <Notification status="error" title={create.error.message} dismissible={false} role="alert" />
          ) : null}
          <Input label="Name" value={name} onChange={setName} isRequired maxLength={80} autoFocus />
          <Input
            label="Address"
            value={address}
            onChange={(next) => setSubdomain(next)}
            maxLength={40}
            inputDir="ltr"
            hint="Lowercase letters, digits and dashes. You can add your own domain later."
          />
          <Select
            label="Language"
            selectedKey={locale}
            onSelectionChange={(key) => (key === null ? undefined : setLocale(String(key)))}
          >
            {languages.map((l) => (
              <SelectItem key={l.id} id={l.id} textValue={l.name}>
                {l.name}
              </SelectItem>
            ))}
          </Select>
        </ModalBody>
        <ModalFooter>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" loading={create.isPending} disabled={name.trim() === ""}>
            Create help center
          </Button>
        </ModalFooter>
      </form>
    </Modal>
  );
}
