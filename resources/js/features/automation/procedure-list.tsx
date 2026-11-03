/*
 * The procedures page: the step-by-step instructions Dimmi follows for
 * particular requests, when each applies, how often Dimmi used it and
 * whether it is live.
 */
import { RiAddLine, RiListOrdered2 } from "@remixicon/react";

import { Button } from "@/components/base/buttons/button";
import { Section } from "@/features/settings/layout";

import { KindMark, ListPage, OpenRow, StatusChip } from "./parts";
import { procedures } from "./procedures";

export function ProcedureList() {
  return (
    <ListPage
      title="Procedures"
      description="Teach Dimmi how your team handles a request, step by step, in plain words."
      action={
        <Button size="small" leadingIcon={RiAddLine}>
          New procedure
        </Button>
      }
    >
      <Section
        title="Procedures"
        description="Dimmi picks the procedure whose description matches what the customer asks."
      >
        {procedures.map((procedure) => (
          <OpenRow
            key={procedure.id}
            target={{ to: "/d/procedures/$procedure", params: { procedure: procedure.id } }}
            mark={<KindMark icon={RiListOrdered2} tint="lime" />}
            title={procedure.name}
            detail={`${procedure.when} · ${procedure.used}`}
          >
            <StatusChip status={procedure.status} />
          </OpenRow>
        ))}
      </Section>
    </ListPage>
  );
}
