/*
 * The browser's side of a passkey: asking the device to make one from the
 * options the server handed out, and to sign the server's challenge with
 * one. The server keeps every challenge; the browser only carries the
 * device's answer back. A person closing the device's prompt is not an
 * error worth a stack trace, so it reads as a plain sentence.
 */
import {
  type AuthenticationResponseJSON,
  browserSupportsWebAuthn,
  type PublicKeyCredentialCreationOptionsJSON,
  type PublicKeyCredentialRequestOptionsJSON,
  type RegistrationResponseJSON,
  startAuthentication,
  startRegistration,
} from "@simplewebauthn/browser";

export type CreationOptions = PublicKeyCredentialCreationOptionsJSON;
export type RequestOptions = PublicKeyCredentialRequestOptionsJSON;

/*
 * PasskeyError is a passkey prompt that did not end in an answer, with
 * what to tell the person.
 */
export class PasskeyError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "PasskeyError";
  }
}

/*
 * passkeysSupported reports whether this browser can use passkeys at all.
 */
export function passkeysSupported(): boolean {
  return browserSupportsWebAuthn();
}

/*
 * createPasskey asks the device to make a passkey with the options.
 */
export async function createPasskey(options: CreationOptions): Promise<RegistrationResponseJSON> {
  try {
    return await startRegistration({ optionsJSON: options });
  } catch (error) {
    throw explain(error, "This device already has a passkey for your account.");
  }
}

/*
 * signWithPasskey asks the device to sign the challenge with one of the
 * passkeys the options allow.
 */
export async function signWithPasskey(options: RequestOptions): Promise<AuthenticationResponseJSON> {
  try {
    return await startAuthentication({ optionsJSON: options });
  } catch (error) {
    throw explain(error, "This device has no passkey for your account.");
  }
}

/*
 * explain turns a failed prompt into a sentence: the device already or
 * never held the passkey, or the prompt was closed or timed out.
 */
function explain(error: unknown, invalidState: string): PasskeyError {
  const name = error instanceof Error ? error.name : "";
  const code = typeof error === "object" && error !== null && "code" in error ? String(error.code) : "";

  if (name === "InvalidStateError" || code === "ERROR_AUTHENTICATOR_PREVIOUSLY_REGISTERED") {
    return new PasskeyError(invalidState);
  }

  return new PasskeyError("The passkey prompt was closed or timed out. Try again.");
}
