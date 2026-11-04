/*
 * Setting up what no page can create yet, straight in the end-to-end
 * server's own database, the way an operator would: a teammate joining
 * a workspace before invitations exist. Values travel as psql variables,
 * never pasted into the statement, which psql reads from its input.
 */
import { execFileSync } from "node:child_process";

const database = process.env.E2E_DATABASE ?? "dimmi_e2e";

function psql(sql: string, variables: Record<string, string>): string {
  const args = ["compose", "exec", "-T", "postgres", "psql", "-U", "postgres", "-d", database, "-v", "ON_ERROR_STOP=1"];
  for (const [name, value] of Object.entries(variables)) {
    args.push("-v", `${name}=${value}`);
  }

  return execFileSync("docker", [...args, "-q", "-t", "-A"], {
    input: sql,
    stdio: ["pipe", "pipe", "pipe"],
  }).toString();
}

const crockford = "0123456789abcdefghjkmnpqrstvwxyz";

/*
 * publicId turns a row's UUID into the public id clients see: the
 * prefix and the UUID as 26 Crockford base32 characters.
 */
export function publicId(prefix: string, uuid: string): string {
  let value = BigInt(`0x${uuid.replaceAll("-", "")}`);
  const chars: string[] = [];
  for (let i = 0; i < 26; i++) {
    chars.unshift(crockford[Number(value & 31n)] ?? "0");
    value >>= 5n;
  }
  return `${prefix}_${chars.join("")}`;
}

/*
 * startConversation starts a conversation on a messenger widget in the
 * workspace the owner with ownerEmail created, as a customer writing in
 * would, and returns its public id.
 */
export function startConversation(ownerEmail: string): string {
  const id = psql(
    `WITH ws AS (
       SELECT m.workspace_id FROM workspace_members m JOIN users u ON u.id = m.user_id
       WHERE u.email = :'owner' AND m.role = 'owner'
     ), inbox AS (
       INSERT INTO inboxes (workspace_id, name) SELECT workspace_id, 'Realtime' FROM ws RETURNING id, workspace_id
     ), channel AS (
       INSERT INTO channel_connections (workspace_id, inbox_id, type, name, status, config)
       SELECT workspace_id, id, 'widget', 'Website', 'active', '{}' FROM inbox RETURNING id, workspace_id, inbox_id
     ), contact AS (
       INSERT INTO contacts (workspace_id, kind, name) SELECT workspace_id, 'user', 'Bob Lee' FROM ws RETURNING id
     ), number AS (
       UPDATE workspaces SET next_conversation_number = next_conversation_number + 1
       WHERE id = (SELECT workspace_id FROM ws) RETURNING next_conversation_number - 1 AS n
     )
     INSERT INTO conversations (workspace_id, number, inbox_id, channel_connection_id, contact_id, subject, status, source)
     SELECT channel.workspace_id, number.n, channel.inbox_id, channel.id, contact.id, 'Card declined', 'open', '{}'
     FROM channel, contact, number RETURNING id`,
    { owner: ownerEmail },
  );

  return publicId("conv", id.trim().split("\n")[0] ?? "");
}

/*
 * addTeammate makes the person with the email a verified member, with
 * the role, of the workspace the owner with ownerEmail created.
 */
export function addTeammate(ownerEmail: string, email: string, role: "admin" | "member") {
  psql(
    `UPDATE users SET email_verified_at = now() WHERE email = :'email';
     INSERT INTO workspace_members (workspace_id, user_id, role, joined_via)
     SELECT m.workspace_id, u.id, :'role', 'invitation'
     FROM workspace_members m JOIN users o ON o.id = m.user_id, users u
     WHERE o.email = :'owner' AND m.role = 'owner' AND u.email = :'email'`,
    { owner: ownerEmail, email, role },
  );
}

/*
 * verifyEmail marks the person's address as proven, as following the
 * mailed link would.
 */
export function verifyEmail(email: string) {
  psql("UPDATE users SET email_verified_at = now() WHERE email = :'email'", { email });
}

/*
 * stopMail stops all mail to the address in the workspace the owner with
 * ownerEmail created, as a hard bounce would.
 */
export function stopMail(ownerEmail: string, email: string) {
  psql(
    `INSERT INTO email_suppressions (workspace_id, email, reason, scope, source, details, created_at, updated_at)
     SELECT m.workspace_id, :'email', 'hard_bounce', 'all', 'dsn', '{}', now(), now()
     FROM workspace_members m JOIN users u ON u.id = m.user_id
     WHERE u.email = :'owner' AND m.role = 'owner'`,
    { owner: ownerEmail, email },
  );
}

/*
 * resumeMail lets mail reach the address again.
 */
export function resumeMail(email: string) {
  psql("DELETE FROM email_suppressions WHERE email = :'email'", { email });
}
