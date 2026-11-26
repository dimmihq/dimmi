/*
 * Conversation attributes: what the workspace wants known about every
 * conversation, such as the issue type or an order number. Teammates
 * fill them in the conversation's details, the AI fills the ones it may
 * as the customer writes, or both. Workflows branch and start on them,
 * and inbox views and reports filter by them.
 *
 * The list shows the active attributes, or the archived ones, each with
 * its type and who fills it. Clicking one opens its page, where every
 * setting lies flat; New attribute opens the same page empty. Active
 * attributes are dragged by their handle into the order the details
 * panel shows them in; the new order waits for Save in the bar.
 */
import { RiAddLine, RiArrowRightSLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useBlocker, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { toast } from "@/components/base/toast/toast";
import { usePermission } from "@/features/settings/inboxes";
import {
  type ConversationAttribute,
  conversationAttributesQuery,
  conversationAttributeTypes,
  filledByLabels,
  reorderConversationAttributes,
} from "@/lib/api/conversation-attributes";
import { teamsQuery } from "@/lib/api/teams";

import { Empty, Glyph, Section, SettingsPage } from "../layout";
import { SaveBar } from "../save-bar";
import { failureText } from "../teams";
import { useAccount } from "../use-account";
import { Movable } from "./conversation-attribute-parts";
import { typeIcons } from "./conversation-attribute-types";

const tabs = { active: "active", archived: "archived" } as const;

/*
 * Line is one attribute in the list, the way settings lists open an
 * item's page: the whole row is the button, it lights up under the
 * pointer, and a chevron at its end says it leads somewhere. It shows
 * the type's mark, the name, the type and who fills it, and its chips.
 */
function Line({
  attribute,
  teams,
  onOpen,
}: {
  attribute: ConversationAttribute;
  teams: Map<string, string>;
  onOpen: () => void;
}) {
  const teamNames = attribute.team_ids.map((id) => teams.get(id)).filter(Boolean);

  return (
    <button
      type="button"
      onClick={onOpen}
      aria-label={`Open ${attribute.label}`}
      className="-ms-1.5 flex min-h-[52px] w-[calc(100%+0.375rem)] cursor-pointer items-center gap-3 rounded-lg py-2.5 ps-1.5 pe-2.5 text-start outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
    >
      <span className="flex shrink-0 items-center">
        <Glyph icon={typeIcons[attribute.data_type]} />
      </span>
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-body-regular text-text-primary">{attribute.label}</span>
        <span className="truncate text-body-2-regular text-text-secondary">
          {conversationAttributeTypes[attribute.data_type]} · {filledByLabels[attribute.filled_by]}
        </span>
      </span>
      <span className="flex shrink-0 items-center gap-1.5">
        {attribute.required_to_close ? (
          <Chip variant="caption" color="blue">
            Required to close
          </Chip>
        ) : null}
        {teamNames.length > 0 ? (
          <Chip variant="caption" color="neutral" title={teamNames.join(", ")}>
            {teamNames.length === 1 ? teamNames[0] : `${teamNames.length} teams`}
          </Chip>
        ) : null}
        <RiArrowRightSLine aria-hidden className="size-5 text-foreground-icon-tertiary rtl:-scale-x-100" />
      </span>
    </button>
  );
}

/*
 * useOrder holds a new order not saved yet; Save sends it.
 */
function useOrder(workspace: string) {
  const client = useQueryClient();
  const [order, setOrder] = useState<string[] | null>(null);
  const save = useMutation({
    mutationFn: () => reorderConversationAttributes(workspace, order ?? []),
    onSuccess: async () => {
      setOrder(null);
      await client.invalidateQueries({ queryKey: conversationAttributesQuery(workspace).queryKey });
      toast.success("Order saved");
    },
  });

  return { order, setOrder, save, dirty: order !== null };
}

function List({
  attributes,
  tab,
  order,
  teams,
  canManage,
  onOrder,
}: {
  attributes: ConversationAttribute[];
  tab: "active" | "archived";
  order: string[] | null;
  teams: Map<string, string>;
  canManage: boolean;
  onOrder: (ids: string[]) => void;
}) {
  const navigate = useNavigate();
  const [dragging, setDragging] = useState<string | null>(null);
  const [over, setOver] = useState<number | null>(null);
  const ordered = order ? [...attributes].sort((a, b) => order.indexOf(a.id) - order.indexOf(b.id)) : attributes;
  const listed = ordered.filter((a) => a.status === tab);
  const movable = canManage && tab === "active" && listed.length > 1;
  const from = listed.findIndex((a) => a.id === dragging);
  const open = (id: string) =>
    navigate({ to: "/d/settings/conversation-attributes/$attribute", params: { attribute: id } });
  const move = (id: string, to: number) => {
    const ids = ordered.map((a) => a.id);
    const target = ids.indexOf(listed[Math.max(0, Math.min(listed.length - 1, to))]?.id ?? "");
    const at = ids.indexOf(id);
    if (at < 0 || target < 0 || at === target) {
      return;
    }
    ids.splice(at, 1);
    ids.splice(target, 0, id);
    onOrder(ids);
  };
  const lineAt = (index: number): "before" | "after" | null => {
    if (dragging === null || over !== index || from === index || from < 0) {
      return null;
    }
    return index < from ? "before" : "after";
  };

  return (
    <Section title={tab === "active" ? "Attributes" : "Archived"}>
      <ul className="contents">
        {listed.map((attribute, index) => (
          <Movable
            key={attribute.id}
            label={attribute.label}
            movable={movable}
            dragging={dragging === attribute.id}
            line={lineAt(index)}
            onStart={() => setDragging(attribute.id)}
            onEnd={() => {
              setDragging(null);
              setOver(null);
            }}
            onOver={() => setOver(index)}
            onDrop={() => {
              if (dragging) {
                move(dragging, index);
              }
              setDragging(null);
              setOver(null);
            }}
            onStep={(by) => move(attribute.id, index + by)}
          >
            <Line attribute={attribute} teams={teams} onOpen={() => open(attribute.id)} />
          </Movable>
        ))}
      </ul>
      {listed.length === 0 ? (
        <Empty>
          {tab === "active" ? "None yet. Add one, such as the issue type or an order number." : "Nothing archived."}
        </Empty>
      ) : null}
    </Section>
  );
}

export function ConversationAttributesSettings() {
  const workspace = useAccount().workspace.id;
  const navigate = useNavigate();
  const { data } = useQuery(conversationAttributesQuery(workspace));
  const teams = new Map((useQuery(teamsQuery(workspace)).data ?? []).map((t) => [t.id, t.name]));
  const manages = usePermission(workspace, "attributes:manage");
  const managesAI = usePermission(workspace, "ai_agent:manage");
  const canManage = manages || managesAI;
  const order = useOrder(workspace);
  const [tab, setTab] = useState<"active" | "archived">("active");
  useBlocker({
    shouldBlockFn: () => order.dirty && !window.confirm("Leave without saving the new order?"),
    enableBeforeUnload: () => order.dirty,
  });
  const activeCount = (data ?? []).filter((a) => a.status === "active").length;

  return (
    <SettingsPage
      title="Conversation attributes"
      description="What your team and the AI note on every conversation, for workflows, views and reports."
      actions={
        canManage ? (
          <Button
            size="small"
            variant="secondary"
            leadingIcon={RiAddLine}
            onClick={() =>
              navigate({ to: "/d/settings/conversation-attributes/$attribute", params: { attribute: "new" } })
            }
          >
            New attribute
          </Button>
        ) : undefined
      }
      footer={
        <SaveBar
          dirty={order.dirty}
          saving={order.save.isPending}
          error={order.save.error ? failureText(order.save.error) : null}
          message="New order not saved"
          onSave={() => order.save.mutate()}
          onDiscard={() => order.setOrder(null)}
        />
      }
    >
      <div className="flex flex-col gap-4">
        <div className="flex items-center justify-between gap-3 px-3">
          <SegmentedControl
            aria-label="Which attributes"
            selectedKeys={[tab]}
            onSelectionChange={(keys) => {
              const next = [...keys][0];
              if (next === "active" || next === "archived") {
                setTab(next);
              }
            }}
          >
            <SegmentedControlItem id={tabs.active}>Active</SegmentedControlItem>
            <SegmentedControlItem id={tabs.archived}>Archived</SegmentedControlItem>
          </SegmentedControl>
          <span className="text-body-2-regular text-text-tertiary">{activeCount} of 200 active</span>
        </div>
        {data ? (
          <List
            attributes={data}
            tab={tab}
            order={order.order}
            teams={teams}
            canManage={canManage}
            onOrder={order.setOrder}
          />
        ) : (
          <Section title="Attributes" loading rows={4} />
        )}
      </div>
    </SettingsPage>
  );
}
