/*
 * Starting a connection to the company's identity provider. A SAML
 * connection takes the provider's entity id, sign-in address and signing
 * certificate, or its metadata address; an OpenID Connect one takes the
 * issuer, client id and secret. It begins as a draft that signs nobody
 * in, so a mistake here locks nobody out.
 */
import { type FormEvent, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { Textarea } from "@/components/base/textarea/textarea";
import { type ConnectionDraft, createConnection } from "@/lib/api/sso";

import { useSSOChange } from "./use-sso";

const protocols: { key: ConnectionDraft["protocol"]; label: string }[] = [
  { key: "saml", label: "SAML" },
  { key: "oidc", label: "OpenID Connect" },
];

interface Fields {
  name: string;
  entityID: string;
  ssoURL: string;
  certificate: string;
  metadataURL: string;
  issuer: string;
  clientID: string;
  clientSecret: string;
  emailAttribute: string;
  nameAttribute: string;
  groupsAttribute: string;
}

const empty: Fields = {
  name: "",
  entityID: "",
  ssoURL: "",
  certificate: "",
  metadataURL: "",
  issuer: "",
  clientID: "",
  clientSecret: "",
  emailAttribute: "email",
  nameAttribute: "",
  groupsAttribute: "",
};

/*
 * draftOf turns the form into the connection to create, leaving out what
 * was left blank.
 */
function draftOf(protocol: ConnectionDraft["protocol"], f: Fields): ConnectionDraft {
  if (protocol === "oidc") {
    return {
      protocol,
      name: f.name.trim(),
      config: { issuer: f.issuer.trim(), client_id: f.clientID.trim() },
      client_secret: f.clientSecret,
    };
  }

  const certificate = f.certificate.trim();

  return {
    protocol,
    name: f.name.trim(),
    config: {
      idp_entity_id: f.entityID.trim() || undefined,
      idp_sso_url: f.ssoURL.trim() || undefined,
      idp_certificates: certificate ? [certificate] : undefined,
      idp_metadata_url: f.metadataURL.trim() || undefined,
      attribute_map: {
        email: f.emailAttribute.trim() || undefined,
        display_name: f.nameAttribute.trim() || undefined,
        groups: f.groupsAttribute.trim() || undefined,
      },
    },
  };
}

function SAMLFields({ fields, set }: { fields: Fields; set: (change: Partial<Fields>) => void }) {
  return (
    <>
      <Input
        label="Metadata URL"
        hint="Or fill in the three fields below."
        value={fields.metadataURL}
        onChange={(metadataURL) => set({ metadataURL })}
      />
      <Input label="Identity provider entity ID" value={fields.entityID} onChange={(entityID) => set({ entityID })} />
      <Input label="Sign-in URL" value={fields.ssoURL} onChange={(ssoURL) => set({ ssoURL })} />
      <Textarea
        label="Signing certificate"
        rows={4}
        placeholder="-----BEGIN CERTIFICATE-----"
        value={fields.certificate}
        onChange={(certificate) => set({ certificate })}
      />
      <Input
        label="Email attribute"
        hint="Leave empty when the name id is the email address."
        value={fields.emailAttribute}
        onChange={(emailAttribute) => set({ emailAttribute })}
      />
      <Input label="Name attribute" value={fields.nameAttribute} onChange={(nameAttribute) => set({ nameAttribute })} />
      <Input
        label="Groups attribute"
        value={fields.groupsAttribute}
        onChange={(groupsAttribute) => set({ groupsAttribute })}
      />
    </>
  );
}

function OIDCFields({ fields, set }: { fields: Fields; set: (change: Partial<Fields>) => void }) {
  return (
    <>
      <Input label="Issuer" value={fields.issuer} onChange={(issuer) => set({ issuer })} />
      <Input label="Client ID" value={fields.clientID} onChange={(clientID) => set({ clientID })} />
      <Input
        label="Client secret"
        type="password"
        value={fields.clientSecret}
        onChange={(clientSecret) => set({ clientSecret })}
      />
    </>
  );
}

export function ConnectionDialog({ isOpen, onClose }: { isOpen: boolean; onClose: () => void }) {
  const { send, failure, prompt } = useSSOChange();
  const [protocol, setProtocol] = useState<ConnectionDraft["protocol"]>("saml");
  const [fields, setFields] = useState<Fields>(empty);
  const [pending, setPending] = useState(false);
  const set = (change: Partial<Fields>) => setFields((current) => ({ ...current, ...change }));

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setPending(true);
    const done = await send((workspace) => createConnection(workspace, draftOf(protocol, fields)));
    setPending(false);
    if (done) {
      setFields(empty);
      onClose();
    }
  };

  return (
    <>
      <Modal size="lg" isOpen={isOpen} onOpenChange={(open) => (open ? null : onClose())}>
        <form onSubmit={submit}>
          <ModalHeader
            title="New connection"
            description="It starts as a draft and signs nobody in until it is tested and turned on."
          />
          <ModalBody className="flex max-h-[60vh] flex-col gap-4 overflow-y-auto">
            <SegmentedControl
              aria-label="Protocol"
              selectedKeys={[protocol]}
              onSelectionChange={(keys) => setProtocol([...keys][0] === "oidc" ? "oidc" : "saml")}
            >
              {protocols.map((p) => (
                <SegmentedControlItem key={p.key} id={p.key}>
                  {p.label}
                </SegmentedControlItem>
              ))}
            </SegmentedControl>
            <Input label="Name" placeholder="Okta" value={fields.name} onChange={(name) => set({ name })} />
            {protocol === "saml" ? <SAMLFields fields={fields} set={set} /> : <OIDCFields fields={fields} set={set} />}
            {failure ? (
              <p role="alert" className="text-body-2-regular text-text-error-primary">
                {failure}
              </p>
            ) : null}
          </ModalBody>
          <ModalFooter className="mt-6">
            <Button variant="secondary" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={fields.name.trim() === "" || pending}>
              Create connection
            </Button>
          </ModalFooter>
        </form>
      </Modal>
      {prompt}
    </>
  );
}
