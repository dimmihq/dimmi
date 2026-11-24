/*
 * Agreeing to the legal terms before an account exists. A server that
 * publishes terms asks the person to tick a box naming each document,
 * linked so it can be read first. A server without legal pages asks nothing. The
 * sentence is the box's label, so clicking its words ticks the box; a link
 * in it opens the document and leaves the box as it was. A server that
 * refuses the account for want of the agreement says so under the box.
 */
import { LinkButton } from "@/components/base/buttons/link-button";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { HintText } from "@/components/base/input/hint-text";

import type { Agreement } from "./use-agreement";

function Document({ href, children }: { href: string; children: string }) {
  return (
    <LinkButton href={href} target="_blank" rel="noreferrer">
      {children}
    </LinkButton>
  );
}

export interface TermsAgreementProps {
  agreement: Agreement;
  /** Someone joining a workspace another person owns agrees to the terms and the privacy policy only. */
  joining?: boolean;
  /** What the server said when it refused the account because the terms were not accepted. */
  error?: string;
}

export function TermsAgreement({ agreement, joining = false, error }: TermsAgreementProps) {
  const { terms, accepted, missing, setAccepted } = agreement;
  if (!terms) {
    return null;
  }

  return (
    <div className="flex flex-col gap-2">
      <Checkbox
        name="terms_accepted"
        isSelected={accepted}
        onChange={setAccepted}
        isInvalid={missing || Boolean(error)}
      >
        <span className="text-body-regular text-text-primary">
          I agree to the <Document href={terms.terms_url}>Terms of Service</Document>
          {joining ? " and " : ", "}
          <Document href={terms.privacy_url}>Privacy Policy</Document>
          {joining ? null : (
            <>
              {" and "}
              <Document href={terms.data_processing_url}>Data Processing Agreement</Document>
            </>
          )}
          .
        </span>
      </Checkbox>
      {missing ? (
        <HintText isInvalid role="alert">
          Tick the box to agree before you create your account.
        </HintText>
      ) : error ? (
        <HintText isInvalid role="alert">
          {error}
        </HintText>
      ) : null}
    </div>
  );
}
