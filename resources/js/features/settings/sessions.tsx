/*
 * Where the person is signed in, from the server: every browser and app
 * session, the one in use marked, with a way to sign any other one out
 * or all of them at once. Signing all others out asks for the password
 * again when the last sign-in is a while ago.
 */
import { RiComputerLine, RiSmartphoneLine } from "@remixicon/react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { AlertDialog } from "@/components/base/modal/modal";
import { endOtherSessions, endSession, type Session, sessionsQuery } from "@/lib/api/sessions";

import { Action } from "./controls";
import { Glyph, Item, Row, RowsSkeleton, Section } from "./layout";
import { useRecentAuth } from "./recent-auth";
import { sessionDetail } from "./session-detail";

export function SessionsSection({ hasPassword }: { hasPassword: boolean }) {
  const queryClient = useQueryClient();
  const { data: sessions = [], isPending } = useQuery(sessionsQuery);
  const { run, prompt } = useRecentAuth(hasPassword);
  const [ending, setEnding] = useState<Session | null>(null);
  const [endingOthers, setEndingOthers] = useState(false);
  const [failure, setFailure] = useState<string | null>(null);
  const others = sessions.filter((session) => !session.current);

  const attempt = async (action: () => Promise<unknown>) => {
    setFailure(null);

    try {
      await run(action);
    } catch (error) {
      setFailure(error instanceof Error ? error.message : "Something went wrong. Try again.");
    }

    await queryClient.invalidateQueries({ queryKey: sessionsQuery.queryKey });
  };

  return (
    <Section title="Sessions" description="Browsers and apps signed in to your account.">
      {isPending ? <RowsSkeleton rows={2} /> : null}
      {sessions.map((session) => (
        <Item
          key={session.id}
          mark={<Glyph icon={session.kind === "app" ? RiSmartphoneLine : RiComputerLine} />}
          title={session.device_label ?? session.client ?? "Unknown device"}
          detail={sessionDetail(session)}
        >
          {session.current ? null : <Action onPress={() => setEnding(session)}>Revoke</Action>}
        </Item>
      ))}
      {others.length > 0 ? (
        <Row label="Sign out other sessions">
          <Action onPress={() => setEndingOthers(true)}>Sign out others</Action>
        </Row>
      ) : null}
      {failure ? (
        <p role="alert" className="py-2.5 pe-3 text-body-2-regular text-text-error-primary">
          {failure}
        </p>
      ) : null}
      <AlertDialog
        isOpen={ending !== null}
        onOpenChange={(open) => !open && setEnding(null)}
        title="Sign this session out?"
        description="That browser or app has to sign in again."
        confirmLabel="Revoke"
        onConfirm={() => {
          if (ending) {
            const id = ending.id;
            void attempt(() => endSession(id));
          }
        }}
      />
      <AlertDialog
        isOpen={endingOthers}
        onOpenChange={setEndingOthers}
        title="Sign out every other session?"
        description="Every browser and app but this one has to sign in again."
        confirmLabel="Sign out others"
        onConfirm={() => void attempt(endOtherSessions)}
      />
      {prompt}
    </Section>
  );
}
