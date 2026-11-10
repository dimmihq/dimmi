/*
 * The page an unsubscribe link in a broadcast email opens, with no
 * sign-in: the link's token names the person. It says which team's
 * email this is and to which address, lists the topics they can hear
 * about, and lets them leave everything at once. Leaving everything
 * still lets notices about their own account through, which the page
 * says plainly. A link that no longer works says so calmly instead of
 * showing an error.
 */
import { RiCheckLine, RiLinkUnlinkM } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { Spinner } from "@/components/base/spinner/spinner";
import { Switch } from "@/components/base/switch/switch";
import { AuthCard, AuthStatus } from "@/features/auth/auth-frame";
import { type EmailPreferences, emailPreferencesQuery, saveEmailPreferences } from "@/lib/api/broadcasts";
import { ApiError } from "@/lib/api/client";

function Gone() {
  return (
    <AuthStatus
      icon={RiLinkUnlinkM}
      title="This link no longer works"
      description="Open the latest email you got and use the link at its foot to change what you receive."
    />
  );
}

function Form({ token, preferences }: { token: string; preferences: EmailPreferences }) {
  const queryClient = useQueryClient();
  const topics = preferences.topics ?? [];
  const [kept, setKept] = useState(() => new Set(topics.filter((t) => t.subscribed).map((t) => t.id)));
  const [all, setAll] = useState(preferences.unsubscribed_all);
  const [saved, setSaved] = useState(false);
  const save = useMutation({
    mutationFn: () => saveEmailPreferences(token, { topics: [...kept], unsubscribe_all: all }),
    onSuccess: (next) => {
      queryClient.setQueryData(emailPreferencesQuery(token).queryKey, next);
      setSaved(true);
    },
  });
  const touch = () => setSaved(false);
  const toggle = (id: string, on: boolean) => {
    const next = new Set(kept);
    if (on) {
      next.add(id);
    } else {
      next.delete(id);
    }
    setKept(next);
    touch();
  };

  return (
    <AuthCard>
      <div className="flex flex-col gap-1.5">
        <h1 className="text-title-2-medium text-text-primary">Email from {preferences.workspace}</h1>
        <p className="text-body-regular text-text-secondary">
          Choose what {preferences.workspace} sends to <bdi dir="ltr">{preferences.email}</bdi>.
        </p>
      </div>
      {topics.length ? (
        <ul className="mt-6 flex flex-col gap-4">
          {topics.map((topic) => (
            <li key={topic.id}>
              <Checkbox
                isSelected={!all && kept.has(topic.id)}
                isDisabled={all}
                onChange={(on) => toggle(topic.id, on)}
              >
                <span className="flex flex-col">
                  <span className="text-body-medium text-text-primary">{topic.name}</span>
                  {topic.description ? (
                    <span className="text-body-2-regular text-text-secondary">{topic.description}</span>
                  ) : null}
                </span>
              </Checkbox>
            </li>
          ))}
        </ul>
      ) : null}
      <div className="mt-6 flex items-start justify-between gap-4 border-t border-separator-border pt-5">
        <span className="flex flex-col">
          <span className="text-body-medium text-text-primary">Unsubscribe from all</span>
          <span className="text-body-2-regular text-text-secondary">
            Notices about your account or service still arrive.
          </span>
        </span>
        <Switch
          aria-label="Unsubscribe from all"
          isSelected={all}
          onChange={(on) => {
            setAll(on);
            touch();
          }}
        />
      </div>
      {save.isError ? (
        <p role="alert" className="mt-4 text-body-2-regular text-text-error-primary">
          {save.error.message}
        </p>
      ) : null}
      <div className="mt-6 flex items-center gap-3">
        <Button onClick={() => save.mutate()} disabled={save.isPending}>
          Save
        </Button>
        {saved ? (
          <span role="status" className="flex items-center gap-1 text-body-2-regular text-text-secondary">
            <RiCheckLine aria-hidden className="size-4 text-foreground-icon-secondary" />
            Saved
          </span>
        ) : null}
      </div>
    </AuthCard>
  );
}

export function EmailPreferencesPage({ token }: { token: string }) {
  const { data, isPending, error } = useQuery({ ...emailPreferencesQuery(token), enabled: token !== "" });
  if (!token || (error instanceof ApiError && error.status === 404)) {
    return <Gone />;
  }
  if (error) {
    return <AuthStatus icon={RiLinkUnlinkM} tone="error" title="Something went wrong" description={error.message} />;
  }
  if (isPending || !data) {
    return (
      <AuthCard>
        <Spinner size="sm" aria-label="Loading" />
      </AuthCard>
    );
  }

  return <Form token={token} preferences={data} />;
}
