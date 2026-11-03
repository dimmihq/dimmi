/*
 * Setting up what no page can create yet, straight in the end-to-end
 * server's own database, the way an operator would: a teammate joining
 * a workspace before invitations exist. Values travel as psql variables,
 * never pasted into the statement, which psql reads from its input.
 */
import { execFileSync } from "node:child_process";

const database = "dimmi_e2e";

function psql(sql: string, variables: Record<string, string>) {
  const args = ["compose", "exec", "-T", "postgres", "psql", "-U", "postgres", "-d", database, "-v", "ON_ERROR_STOP=1"];
  for (const [name, value] of Object.entries(variables)) {
    args.push("-v", `${name}=${value}`);
  }

  execFileSync("docker", [...args, "-q"], { input: sql, stdio: ["pipe", "pipe", "pipe"] });
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
