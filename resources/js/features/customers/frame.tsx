/*
 * The frame of the customer pages: one panel with the section's title and
 * the page's own actions on the right. The lists (people, companies,
 * saved audiences, the blocked and, for those who may erase customers,
 * the erasures) share one frame with tabs between them, mounted once by
 * their layout route so the tabs' underline slides from one to the next;
 * each list puts its actions in the header through CustomersActions. A
 * single company or audience gets a frame of its own, without tabs. The
 * panel is BoardUI's page ground, as its app shell puts a data table: the
 * table's search and header, and the secondary cards of the pages, are
 * drawn for that surface.
 */
import { useQuery } from "@tanstack/react-query";
import { Outlet, useLocation, useNavigate } from "@tanstack/react-router";
import { createContext, type ReactNode, useContext, useState } from "react";
import { createPortal } from "react-dom";

import { Tab, TabList, Tabs } from "@/components/base/tabs/tabs";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { callerQuery } from "@/lib/api/workspace";

import { useWorkspace } from "./names";

const tabs = [
  { to: "/d/customers", label: "People" },
  { to: "/d/customers/companies", label: "Companies" },
  { to: "/d/customers/audiences", label: "Audiences" },
  { to: "/d/customers/blocked", label: "Blocked" },
  { to: "/d/customers/erasures", label: "Erasures" },
] as const;

/*
 * Sections are BoardUI's underline tabs; picking one goes to its page,
 * and the address says which is current.
 */
function Sections() {
  const { pathname } = useLocation();
  const navigate = useNavigate();
  const workspace = useWorkspace();
  const { data: caller } = useQuery({ ...callerQuery(workspace), enabled: workspace !== "" });
  const canErase = caller?.member?.permissions.includes("contacts:erase") ?? false;
  const shown = tabs.filter((tab) => tab.to !== "/d/customers/erasures" || canErase);
  const current = shown.find(
    (tab) => pathname === tab.to || (pathname.startsWith(`${tab.to}/`) && tab.to !== "/d/customers"),
  );

  return (
    <div className="overflow-x-auto px-4 pb-3 [scrollbar-width:none]">
      <Tabs
        className="w-max min-w-full"
        selectedKey={current?.to ?? "/d/customers"}
        onSelectionChange={(key) => void navigate({ to: String(key) })}
      >
        <TabList aria-label="Customers" items={shown}>
          {(tab) => <Tab id={tab.to}>{tab.label}</Tab>}
        </TabList>
      </Tabs>
    </div>
  );
}

/*
 * ActionsSlot is where the lists' header takes the current page's
 * actions.
 */
const ActionsSlot = createContext<HTMLElement | null>(null);

/*
 * CustomersActions puts a list's own actions, such as New contact, in the
 * shared header.
 */
export function CustomersActions({ children }: { children: ReactNode }) {
  const slot = useContext(ActionsSlot);

  return slot ? createPortal(children, slot) : null;
}

/*
 * CustomersLists is the lists' layout: the panel, the header with its
 * actions slot, the tabs, and the current list below them.
 */
export function CustomersLists() {
  const [slot, setSlot] = useState<HTMLElement | null>(null);

  return (
    <Panel label="Customers" ground className="h-full flex-1">
      <PageHeader title="Customers">
        <div ref={setSlot} className="flex items-center gap-2" />
      </PageHeader>
      <Sections />
      <ScrollArea ground>
        <ActionsSlot.Provider value={slot}>
          <Outlet />
        </ActionsSlot.Provider>
      </ScrollArea>
    </Panel>
  );
}

/*
 * CustomersFrame frames one company or audience, without the tabs.
 */
export function CustomersFrame({
  title = "Customers",
  actions,
  children,
}: {
  title?: ReactNode;
  actions?: ReactNode;
  children: ReactNode;
}) {
  return (
    <Panel label="Customers" ground className="h-full flex-1">
      <PageHeader title={title}>{actions}</PageHeader>
      <ScrollArea ground>{children}</ScrollArea>
    </Panel>
  );
}
