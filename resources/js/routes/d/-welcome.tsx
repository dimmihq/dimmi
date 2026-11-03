import { useState } from "react";

import { Button } from "@/components/base/buttons/button";

/*
 * A placeholder that proves the toolchain end to end: the Go page, the
 * router, BoardUI's tokens and type scale, and state kept across hot
 * updates.
 */
export function Welcome() {
  const [clicks, setClicks] = useState(0);

  return (
    <main className="flex min-h-screen flex-col items-center justify-center gap-6 bg-background-full p-8">
      <h1 className="text-title-1-medium text-text-primary">Dimmi</h1>
      <p className="text-body-regular text-text-secondary">The web app is running.</p>
      <Button variant="primary" onClick={() => setClicks((n) => n + 1)}>
        Clicked {clicks} times
      </Button>
    </main>
  );
}
