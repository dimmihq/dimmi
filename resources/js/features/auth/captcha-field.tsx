/*
 * The captcha a signed-out form asks for, when the server turned one on.
 * It hands the form a fresh answer once solved and takes it back when it
 * expires; an answer works once, so the form remounts the field after a
 * refused submit to get a new one.
 */
import { useEffect, useRef, useState } from "react";

import type { Captcha } from "@/lib/api/auth";

import { loadTurnstile } from "./turnstile";

export function CaptchaField({ captcha, onToken }: { captcha: Captcha; onToken: (token: string | null) => void }) {
  const box = useRef<HTMLFieldSetElement>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let id: string | null = null;
    let gone = false;

    loadTurnstile()
      .then((turnstile) => {
        if (gone || !box.current) {
          return;
        }
        id = turnstile.render(box.current, {
          sitekey: captcha.site_key,
          theme: "auto",
          callback: (token) => onToken(token),
          "expired-callback": () => onToken(null),
          "error-callback": () => onToken(null),
        });
      })
      .catch(() => setFailed(true));

    return () => {
      gone = true;
      if (id !== null) {
        window.turnstile?.remove(id);
      }
    };
  }, [captcha.site_key, onToken]);

  if (failed) {
    return (
      <p role="alert" className="text-body-2-regular text-text-error-primary">
        The check that you are not a robot did not load. Reload the page to try again.
      </p>
    );
  }

  return <fieldset ref={box} aria-label="Check that you are not a robot" className="min-h-16" />;
}
