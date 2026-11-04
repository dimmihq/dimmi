/*
 * The workspace's sending domains on the email settings page: each with
 * its state and a way into its records, and, for whoever may manage
 * email, a field to add another. A new domain opens straight on its
 * records, since publishing them is the next step.
 */
import { RiGlobalLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Failure } from "@/features/email/setup/failure";
import { Empty, Glyph, Item, Row, Section } from "@/features/settings/layout";
import { useAccount } from "@/features/settings/use-account";
import { createEmailDomain, domainsKey, emailDomainsQuery } from "@/lib/api/email-domains";

import { DomainStatusChip } from "./domain-status";

/*
 * AddDomain adds a domain and opens it.
 */
function AddDomain() {
  const me = useAccount();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [domain, setDomain] = useState("");
  const add = useMutation({
    mutationFn: () => createEmailDomain(me.workspace.id, domain.trim()),
    onSuccess: (created) => {
      void queryClient.invalidateQueries({ queryKey: domainsKey(me.workspace.id) });
      return navigate({ to: "/d/settings/email/domains/$domain", params: { domain: created.id } });
    },
  });

  return (
    <>
      <Row label="Add a domain" description="Reply from your own address, such as support@yourcompany.com.">
        <Input
          aria-label="New sending domain"
          size="small"
          placeholder="yourcompany.com"
          value={domain}
          onChange={setDomain}
          className="w-56"
        />
        <Button size="small" disabled={!domain.trim() || add.isPending} onClick={() => add.mutate()}>
          Add domain
        </Button>
      </Row>
      <Failure error={add.error} />
    </>
  );
}

/*
 * DomainList lists the sending domains.
 */
export function DomainList({ manage }: { manage: boolean }) {
  const me = useAccount();
  const { data: domains, error } = useQuery(emailDomainsQuery(me.workspace.id));

  return (
    <Section title="Sending domains" description="Prove you own a domain so replies come from it.">
      {domains && domains.length === 0 && !manage ? <Empty>No domain is set up yet.</Empty> : null}
      {domains?.map((domain) => (
        <Item
          key={domain.id}
          mark={<Glyph icon={RiGlobalLine} />}
          title={
            <Link
              to="/d/settings/email/domains/$domain"
              params={{ domain: domain.id }}
              className="rounded-md outline-none hover:underline focus-visible:ring-2 focus-visible:ring-border-focus-ring"
            >
              {domain.domain}
            </Link>
          }
          detail={domain.failure_message ?? `${domain.records.length} records`}
        >
          <DomainStatusChip status={domain.status} />
        </Item>
      ))}
      {manage ? <AddDomain /> : null}
      <Failure error={error} />
    </Section>
  );
}
