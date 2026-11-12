/*
 * @dimmi/angular puts the dimmi messenger in an Angular app. Add
 * provideDimmi({ appId, host }) to your application's providers and
 * inject DimmiService wherever you need it. The messenger loads only in
 * the browser, so server-side rendering is safe, and the service's
 * methods do nothing on the server. unreadCount is a signal.
 */
import {
  ENVIRONMENT_INITIALIZER,
  type EnvironmentProviders,
  InjectionToken,
  inject,
  makeEnvironmentProviders,
  PLATFORM_ID,
  type Signal,
  signal,
} from "@angular/core";

import {
  type DimmiClient,
  type DimmiOptions,
  getDimmi,
  type IdentitySource,
  loadDimmi,
  type MessengerSettings,
} from "@dimmi/browser";

export type { DimmiClient, DimmiOptions, IdentitySource, MessengerSettings } from "@dimmi/browser";

export const DIMMI_OPTIONS = new InjectionToken<DimmiOptions>("DIMMI_OPTIONS");

/*
 * DimmiService wraps the messenger's client for Angular.
 */
export class DimmiService {
  readonly browser: boolean;
  private readonly client: DimmiClient;
  private readonly count = signal(0);
  private readonly stop: () => void;

  constructor(options: DimmiOptions, browser: boolean) {
    this.browser = browser;
    this.client = browser ? loadDimmi(options) : getDimmi();
    this.stop = this.client.onUnreadCountChange((next) => this.count.set(next));
  }

  /* The visitor's unread count. */
  get unreadCount(): Signal<number> {
    return this.count.asReadonly();
  }

  get ready(): Promise<void> {
    return this.client.ready;
  }

  update(settings: MessengerSettings): void {
    this.client.update(settings);
  }

  show(): void {
    this.client.show();
  }

  hide(): void {
    this.client.hide();
  }

  toggle(): void {
    this.client.toggle();
  }

  showNewMessage(text?: string): void {
    this.client.showNewMessage(text);
  }

  identify(source?: IdentitySource): void {
    this.client.identify(source);
  }

  shutdown(): void {
    this.client.shutdown();
  }

  trackEvent(name: string, metadata?: Record<string, unknown>): void {
    this.client.trackEvent(name, metadata);
  }

  page(): void {
    this.client.page();
  }

  command(name: string, ...args: unknown[]): void {
    this.client.command(name, ...args);
  }

  ngOnDestroy(): void {
    this.stop();
  }
}

/*
 * provideDimmi provides DimmiService and loads the messenger when the
 * application starts in a browser.
 */
export function provideDimmi(options: DimmiOptions): EnvironmentProviders {
  return makeEnvironmentProviders([
    { provide: DIMMI_OPTIONS, useValue: options },
    {
      provide: DimmiService,
      // PLATFORM_ID is "browser" in the browser; isPlatformBrowser from
      // @angular/common checks the same, without pulling that package in.
      useFactory: () => new DimmiService(inject(DIMMI_OPTIONS), inject(PLATFORM_ID) === "browser"),
    },
    // ENVIRONMENT_INITIALIZER rather than provideEnvironmentInitializer,
    // which Angular 17 and 18 lack.
    {
      provide: ENVIRONMENT_INITIALIZER,
      multi: true,
      useValue: () => {
        inject(DimmiService);
      },
    },
  ]);
}
