# @dimmi/angular

The dimmi messenger for Angular 17 and later: `provideDimmi()` and an injectable `DimmiService`. Server-rendering safe: the messenger loads only on the browser platform.

```sh
npm install @dimmi/angular
```

## Usage

```ts
// app.config.ts
import { provideDimmi } from "@dimmi/angular";

export const appConfig: ApplicationConfig = {
  providers: [provideDimmi({ appId: "wgt_...", host: "https://dimmi.example.com" })],
};
```

```ts
import { Component, inject } from "@angular/core";
import { DimmiService } from "@dimmi/angular";

@Component({
  selector: "app-help",
  standalone: true,
  template: `<button (click)="dimmi.show()">Chat with us ({{ dimmi.unreadCount() }})</button>`,
})
export class HelpComponent {
  readonly dimmi = inject(DimmiService);
}
```

## API

- `provideDimmi(options)`: provides `DimmiService` and loads the messenger at startup in the browser. Options are those of `@dimmi/browser`.
- `DimmiService`: `show`, `hide`, `toggle`, `showNewMessage`, `update`, `identify`, `shutdown`, `trackEvent`, `page`, `command`, `ready`, and `unreadCount`, a signal. On the server every method does nothing.
- `DIMMI_OPTIONS`: the injection token holding the options.

With SSR, sign identity tokens in `server.ts` with `@dimmi/server/express`:

```ts
import { dimmiIdentity } from "@dimmi/server/express";
app.get(
  "/api/dimmi/token",
  dimmiIdentity((req) => req.user ?? null),
);
```

The package ships plain ES modules with no decorators, so it needs no Angular compiler step.

## Regenerating after API changes

This package has no generated layer; it wraps `@dimmi/browser`. `pnpm sdk:generate` regenerates every generated SDK file in this repo.
