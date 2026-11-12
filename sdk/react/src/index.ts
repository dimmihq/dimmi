/*
 * @dimmi/react puts the dimmi messenger in a React app. Render
 * <DimmiProvider> once near the root; it loads the messenger after the
 * first render, so server rendering never touches window. useDimmi()
 * answers the typed client anywhere below it, and useUnreadCount() the
 * visitor's unread messages as they change.
 */

import { type DimmiClient, type DimmiOptions, getDimmi, loadDimmi } from "@dimmi/browser";
import {
  type ButtonHTMLAttributes,
  createContext,
  createElement,
  type ReactNode,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

export type { DimmiClient, DimmiOptions, IdentitySource, MessengerSettings } from "@dimmi/browser";

const Context = createContext<DimmiClient | null>(null);

export interface DimmiProviderProps extends DimmiOptions {
  children?: ReactNode;
}

/*
 * DimmiProvider loads the messenger once and shares its client. A new
 * identity function or settings object is sent again; keep them stable
 * with useCallback and useMemo to avoid needless identity checks.
 */
export function DimmiProvider({ children, ...options }: DimmiProviderProps): ReactNode {
  const client = useMemo(() => getDimmi(), []);
  const { appId, host, apiBase, nonce, identityUrl, identity, settings } = options;

  useEffect(() => {
    loadDimmi({ appId, host, apiBase, nonce, identityUrl, identity, settings });
  }, [appId, host, apiBase, nonce, identityUrl, identity, settings]);

  return createElement(Context.Provider, { value: client }, children);
}

/*
 * Dimmi is the provider without children, for apps that only need the
 * messenger on the page.
 */
export function Dimmi(props: DimmiOptions): ReactNode {
  return createElement(DimmiProvider, props);
}

/*
 * useDimmi answers the messenger's client. Outside a provider it still
 * works, against a messenger loaded some other way.
 */
export function useDimmi(): DimmiClient {
  const client = useContext(Context);
  return useMemo(() => client ?? getDimmi(), [client]);
}

/*
 * useUnreadCount answers how many messages the visitor has not read.
 */
export function useUnreadCount(): number {
  const dimmi = useDimmi();
  const [count, setCount] = useState(0);
  useEffect(() => dimmi.onUnreadCountChange(setCount), [dimmi]);
  return count;
}

/*
 * useTrackPage calls page() whenever key changes, such as your router's
 * pathname.
 */
export function useTrackPage(key: unknown): void {
  const dimmi = useDimmi();
  const first = useRef(true);
  // biome-ignore lint/correctness/useExhaustiveDependencies: key is what triggers the call.
  useEffect(() => {
    if (first.current) {
      first.current = false;
      return;
    }
    dimmi.page();
  }, [dimmi, key]);
}

export interface DimmiButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  /* What a click does: open, toggle, or start a new message. */
  action?: "open" | "toggle" | "new";
  /* The text a new message starts with. */
  message?: string;
}

/*
 * DimmiButton is a plain button that opens the messenger.
 */
export function DimmiButton({ action = "open", message, onClick, ...rest }: DimmiButtonProps): ReactNode {
  const dimmi = useDimmi();
  return createElement("button", {
    type: "button",
    ...rest,
    onClick: (event: Parameters<NonNullable<typeof onClick>>[0]) => {
      onClick?.(event);
      if (event.defaultPrevented) {
        return;
      }
      if (action === "toggle") {
        dimmi.toggle();
      } else if (action === "new") {
        dimmi.showNewMessage(message);
      } else {
        dimmi.show();
      }
    },
  });
}
