/*
 * What went wrong when a provider sent the browser back with an error
 * code, in words. The codes come from the server's provider callback.
 */
const messages: Record<string, string> = {
  oauth_state: "That sign-in attempt expired or was started in another browser. Try again.",
  oauth_denied: "You cancelled the sign-in at the provider.",
  oauth_unavailable: "The provider could not be reached. Try again, or sign in another way.",
  oauth_email_unverified: "The provider has not verified your email address. Verify it there, or sign in another way.",
  signup_closed: "Signing up is by invitation on this server.",
  sso_required: "Sign in with your organization's single sign-on.",
  identity_taken: "That account is already linked to another user.",
  provider_already_linked: "You already linked an account of this provider.",
  reauth_user_mismatch: "Sign in as the user of this session.",
  two_factor_challenge_expired: "Your sign-in timed out before the second step. Sign in again.",
  auth_busy: "The server is busy. Try again in a moment.",
};

/*
 * oauthError returns the message for the code, a general one for a code
 * it does not know, and null for no code.
 */
export function oauthError(code: string | undefined): string | null {
  if (!code) {
    return null;
  }

  return messages[code] ?? "Signing in did not work. Try again.";
}
