/*
 * @dimmi/vue puts the dimmi messenger in a Vue 3 app. app.use(createDimmi
 * ({ appId, host })) loads it once in the browser and does nothing during
 * server rendering, so it suits Nuxt as a plugin too. useDimmi() answers
 * the typed client in any component, and useUnreadCount() a ref of the
 * visitor's unread messages.
 */

import { type DimmiClient, type DimmiOptions, getDimmi, isBrowser, loadDimmi } from "@dimmi/browser";
import { type App, getCurrentInstance, type InjectionKey, inject, onScopeDispose, type Ref, ref } from "vue";

export type { DimmiClient, DimmiOptions, IdentitySource, MessengerSettings } from "@dimmi/browser";

export const DimmiKey: InjectionKey<DimmiClient> = Symbol("dimmi");

/*
 * createDimmi is the plugin: it loads the messenger in the browser and
 * provides its client.
 */
export function createDimmi(options: DimmiOptions): { install(app: App): void } {
  return {
    install(app: App) {
      const client = isBrowser() ? loadDimmi(options) : getDimmi();
      app.provide(DimmiKey, client);
      app.config.globalProperties.$dimmi = client;
    },
  };
}

/*
 * useDimmi answers the messenger's client, even without the plugin,
 * against a messenger loaded some other way.
 */
export function useDimmi(): DimmiClient {
  return (getCurrentInstance() ? inject(DimmiKey, null) : null) ?? getDimmi();
}

/*
 * useUnreadCount answers a ref of the visitor's unread count, updated
 * until the calling component or scope goes away.
 */
export function useUnreadCount(): Ref<number> {
  const count = ref(0);
  const stop = useDimmi().onUnreadCountChange((next) => {
    count.value = next;
  });
  onScopeDispose(stop, true);
  return count;
}

declare module "vue" {
  interface ComponentCustomProperties {
    $dimmi: DimmiClient;
  }
}
