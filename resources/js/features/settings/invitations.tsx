/*
 * Inviting people into the workspace, for those allowed to change its
 * members: a dialog that takes a batch of addresses and a role and says
 * what happened to each, and the list of invitations still waiting for
 * an answer, each of which can be sent again, handed over as a fresh
 * link, or withdrawn. Every new invitation comes back with its link, so
 * the admin can also pass it on themselves, and can join teams at once.
 */
import { RiFileCopyLine, RiMailLine, RiMore2Line, RiUserAddLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { Dropdown, DropdownItem, DropdownPopover, DropdownTrigger } from "@/components/base/dropdown/dropdown";
import { Input } from "@/components/base/input/input";
import { AlertDialog, Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { Select, SelectItem } from "@/components/base/select/select";
import { Textarea } from "@/components/base/textarea/textarea";
import { ApiError } from "@/lib/api/client";
import {
  type Invitation,
  type InvitationResult,
  type InvitationRole,
  invitationsQuery,
  resendInvitation,
  revokeInvitation,
  sendInvitations,
} from "@/lib/api/invitations";
import { teamsQuery } from "@/lib/api/teams";
import { cx } from "@/utils/cx";

import { expiryLabel, maxInvitations, outcome, splitEmails } from "./invitation-text";
import { Glyph, Item, Section } from "./layout";

const roles: InvitationRole[] = ["member", "admin"];

const hint = `Separate addresses with commas, spaces or new lines. Up to ${maxInvitations} at a time.`;

function roleLabel(role: InvitationRole): string {
  return role === "admin" ? "Admin" : "Member";
}

/*
 * problem returns what the server said went wrong, preferring the
 * message about a field.
 */
function problem(error: unknown): string {
  if (error instanceof ApiError) {
    return Object.values(error.fields)[0] ?? error.message;
  }

  return error instanceof Error ? error.message : "Something went wrong. Try again.";
}

/*
 * resendProblem explains a refused resend, putting the daily limit in
 * plain words.
 */
function resendProblem(error: unknown): string {
  if (error instanceof ApiError && error.code === "too_many_attempts") {
    return "This invitation was already sent several times today. Try again tomorrow.";
  }

  return problem(error);
}

function CopyLink({ email, url }: { email: string; url: string }) {
  const [copied, setCopied] = useState(false);

  return (
    <div className="flex items-center gap-2">
      <Input aria-label={`Invitation link for ${email}`} size="small" value={url} isReadOnly className="flex-1" />
      <Button
        variant="secondary"
        size="small"
        leadingIcon={RiFileCopyLine}
        onClick={() => navigator.clipboard.writeText(url).then(() => setCopied(true))}
      >
        {copied ? "Copied" : "Copy"}
      </Button>
    </div>
  );
}

function Results({ results }: { results: InvitationResult[] }) {
  return (
    <ul aria-label="Invitations" className="flex flex-col gap-3">
      {results.map((result) => {
        const said = outcome(result);
        const link = result.invitation?.accept_url;

        return (
          <li key={result.email} className="flex flex-col gap-1.5">
            <div className="flex items-center justify-between gap-3">
              <span className="truncate text-body-regular text-text-primary">{result.email}</span>
              <span
                className={cx(
                  "shrink-0 text-body-2-regular",
                  said.tone === "failed" ? "text-text-error-primary" : "text-text-secondary",
                )}
              >
                {said.text}
              </span>
            </div>
            {link ? <CopyLink email={result.email} url={link} /> : null}
          </li>
        );
      })}
    </ul>
  );
}

/*
 * InviteDialog asks for the addresses and the role, then shows what
 * happened to each address until it is closed.
 */
function InviteDialog({ workspace, isOpen, onClose }: { workspace: string; isOpen: boolean; onClose: () => void }) {
  const queryClient = useQueryClient();
  const [text, setText] = useState("");
  const [role, setRole] = useState<InvitationRole>("member");
  const [teamIds, setTeamIds] = useState<string[]>([]);
  const { data: teams = [] } = useQuery({ ...teamsQuery(workspace), enabled: isOpen });
  const emails = splitEmails(text);
  const tooMany = emails.length > maxInvitations;

  const send = useMutation({
    mutationFn: () => sendInvitations(workspace, emails, role, teamIds),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: invitationsQuery(workspace).queryKey }),
  });

  const close = () => {
    setText("");
    setRole("member");
    setTeamIds([]);
    send.reset();
    onClose();
  };

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    send.mutate();
  };

  const error = tooMany ? `Invite up to ${maxInvitations} people at a time.` : send.error ? problem(send.error) : null;

  return (
    <Modal isOpen={isOpen} onOpenChange={(open) => (open ? null : close())}>
      {send.data ? (
        <>
          <ModalHeader title="Invitations" description="Each address and what happened to it." />
          <ModalBody>
            <Results results={send.data} />
          </ModalBody>
          <ModalFooter className="mt-6">
            <Button onClick={close}>Done</Button>
          </ModalFooter>
        </>
      ) : (
        <form onSubmit={submit}>
          <ModalHeader
            title="Invite people"
            description="They get a mail with a link to join. The link works for 7 days."
          />
          <ModalBody className="flex flex-col gap-4">
            <Textarea
              label="Email addresses"
              placeholder="ada@company.com, grace@company.com"
              rows={3}
              autoFocus
              value={text}
              onChange={(typed) => {
                setText(typed);
                send.reset();
              }}
              isInvalid={error !== null}
              hint={error ?? hint}
            />
            <Select
              label="Role"
              selectedKey={role}
              onSelectionChange={(key) => setRole(key === "admin" ? "admin" : "member")}
            >
              {roles.map((option) => (
                <SelectItem key={option} id={option} textValue={roleLabel(option)}>
                  {roleLabel(option)}
                </SelectItem>
              ))}
            </Select>
            {teams.length > 0 ? (
              <fieldset className="flex flex-col gap-2">
                <legend className="mb-2 text-body-2-medium text-text-secondary">Teams to join</legend>
                {teams.map((team) => (
                  <Checkbox
                    key={team.id}
                    isSelected={teamIds.includes(team.id)}
                    onChange={(on) =>
                      setTeamIds((ids) => (on ? [...ids, team.id] : ids.filter((id) => id !== team.id)))
                    }
                  >
                    {team.name}
                  </Checkbox>
                ))}
              </fieldset>
            ) : null}
          </ModalBody>
          <ModalFooter className="mt-6">
            <Button variant="secondary" onClick={close}>
              Cancel
            </Button>
            <Button type="submit" disabled={send.isPending || emails.length === 0 || tooMany}>
              {send.isPending ? "Sending…" : "Send invitations"}
            </Button>
          </ModalFooter>
        </form>
      )}
    </Modal>
  );
}

/*
 * InviteButton opens the invitation dialog, for the members page's
 * heading.
 */
export function InviteButton({ workspace }: { workspace: string }) {
  const [open, setOpen] = useState(false);

  return (
    <>
      <Button size="small" leadingIcon={RiUserAddLine} onClick={() => setOpen(true)}>
        Invite
      </Button>
      <InviteDialog workspace={workspace} isOpen={open} onClose={() => setOpen(false)} />
    </>
  );
}

function Waiting({
  invitation,
  onResend,
  onLink,
  onRevoke,
}: {
  invitation: Invitation;
  onResend: () => void;
  onLink: () => void;
  onRevoke: () => void;
}) {
  const label = `Manage invitation for ${invitation.email}`;
  const [menuOpen, setMenuOpen] = useState(false);
  const pick = (action: () => void) => () => {
    setMenuOpen(false);
    action();
  };

  return (
    <Item mark={<Glyph icon={RiMailLine} />} title={invitation.email} detail={expiryLabel(invitation.expires_at)}>
      <Chip variant="caption" color="neutral">
        {roleLabel(invitation.role)}
      </Chip>
      <Dropdown isOpen={menuOpen} onOpenChange={setMenuOpen}>
        <DropdownTrigger
          aria-label={label}
          className="flex size-8 items-center justify-center rounded-lg outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
        >
          <RiMore2Line aria-hidden className="size-4 text-foreground-icon-secondary" />
        </DropdownTrigger>
        <DropdownPopover aria-label={label} placement="bottom end">
          <DropdownItem onSelect={pick(onResend)}>
            <span className="text-body-2-regular text-text-primary">Resend</span>
          </DropdownItem>
          <DropdownItem onSelect={pick(onLink)}>
            <span className="text-body-2-regular text-text-primary">Copy a new link</span>
          </DropdownItem>
          <DropdownItem onSelect={pick(onRevoke)}>
            <span className="text-body-2-regular text-text-error-primary">Revoke</span>
          </DropdownItem>
        </DropdownPopover>
      </Dropdown>
    </Item>
  );
}

/*
 * Notice is the last thing done to an invitation, or why it failed.
 */
interface Notice {
  tone: "status" | "alert";
  text: string;
}

/*
 * PendingInvitations lists the invitations still waiting for an answer,
 * and draws nothing when there are none.
 */
export function PendingInvitations({ workspace }: { workspace: string }) {
  const queryClient = useQueryClient();
  const { data: invitations = [] } = useQuery(invitationsQuery(workspace));
  const [revoking, setRevoking] = useState<Invitation | null>(null);
  const [linking, setLinking] = useState<Invitation | null>(null);
  const [link, setLink] = useState<{ email: string; url: string } | null>(null);
  const [notice, setNotice] = useState<Notice | null>(null);
  const refresh = () => queryClient.invalidateQueries({ queryKey: invitationsQuery(workspace).queryKey });

  const resend = useMutation({
    mutationFn: (invitation: Invitation) => resendInvitation(workspace, invitation.id),
    onSuccess: (_, invitation) => setNotice({ tone: "status", text: `Sent again to ${invitation.email}.` }),
    onError: (error) => setNotice({ tone: "alert", text: resendProblem(error) }),
    onSettled: refresh,
  });
  const relink = useMutation({
    mutationFn: (invitation: Invitation) => resendInvitation(workspace, invitation.id, "link"),
    onSuccess: (sent, invitation) => {
      setNotice(null);
      setLink(sent.invitation.accept_url ? { email: invitation.email, url: sent.invitation.accept_url } : null);
    },
    onError: (error) => setNotice({ tone: "alert", text: resendProblem(error) }),
    onSettled: refresh,
  });
  const revoke = useMutation({
    mutationFn: (invitation: Invitation) => revokeInvitation(workspace, invitation.id),
    onSuccess: (_, invitation) =>
      setNotice({ tone: "status", text: `Revoked the invitation for ${invitation.email}.` }),
    onError: (error) => setNotice({ tone: "alert", text: problem(error) }),
    onSettled: refresh,
  });

  if (invitations.length === 0 && notice === null && link === null) {
    return null;
  }

  return (
    <>
      {invitations.length > 0 ? (
        <Section title="Pending invitations" description={`${invitations.length} waiting for an answer`}>
          {invitations.map((invitation) => (
            <Waiting
              key={invitation.id}
              invitation={invitation}
              onResend={() => resend.mutate(invitation)}
              onLink={() => setLinking(invitation)}
              onRevoke={() => setRevoking(invitation)}
            />
          ))}
        </Section>
      ) : null}
      {notice ? (
        <p
          role={notice.tone}
          className={cx(
            "px-3 text-body-2-regular",
            notice.tone === "alert" ? "text-text-error-primary" : "text-text-secondary",
          )}
        >
          {notice.text}
        </p>
      ) : null}
      {link ? (
        <div className="flex flex-col gap-1 px-3">
          <p role="status" className="text-body-2-regular text-text-secondary">
            {`A new link for ${link.email}. Pass it on yourself: the mailed one no longer works.`}
          </p>
          <CopyLink email={link.email} url={link.url} />
        </div>
      ) : null}
      <AlertDialog
        isOpen={linking !== null}
        onOpenChange={(open) => (open ? null : setLinking(null))}
        title={`Copy a new link for ${linking?.email ?? ""}?`}
        description="You get a fresh link to pass on yourself. The link in the mail stops working."
        confirmLabel="Make a new link"
        variant="default"
        onConfirm={() => (linking ? relink.mutateAsync(linking).catch(() => undefined) : undefined)}
      />
      <AlertDialog
        isOpen={revoking !== null}
        onOpenChange={(open) => (open ? null : setRevoking(null))}
        title={`Revoke the invitation for ${revoking?.email ?? ""}?`}
        description="The link in the mail stops working straight away."
        confirmLabel="Revoke invitation"
        onConfirm={() => (revoking ? revoke.mutateAsync(revoking).catch(() => undefined) : undefined)}
      />
    </>
  );
}
