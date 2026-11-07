/*
|--------------------------------------------------------------------------
| Service Worker
|--------------------------------------------------------------------------
|
| The dashboard's service worker, served from the site's root so it may
| receive pushes for the whole dashboard. It shows a push as a system
| notification, replacing an older one about the same thing, and a click
| opens the conversation in a dashboard tab that is already open, or a
| new one. It never caches anything, so a deploy reaches people at once.
|
*/

package web

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"
)

/*
serviceWorker is the worker's script.
*/
const serviceWorker = `self.addEventListener("install", () => self.skipWaiting());
self.addEventListener("activate", (event) => event.waitUntil(self.clients.claim()));
self.addEventListener("push", (event) => {
  let data = {};
  try { data = event.data ? event.data.json() : {}; } catch { data = { title: event.data && event.data.text() }; }
  event.waitUntil(self.registration.showNotification(data.title || "dimmi", {
    body: data.body || "", tag: data.tag, renotify: Boolean(data.tag), data: { url: data.url || "/d" },
  }));
});
self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  const url = (event.notification.data && event.notification.data.url) || "/d";
  event.waitUntil(self.clients.matchAll({ type: "window", includeUncontrolled: true }).then((tabs) => {
    for (const tab of tabs) {
      if (new URL(tab.url).origin === self.location.origin && "focus" in tab) {
        return tab.navigate(url).then((t) => (t || tab).focus());
      }
    }
    return self.clients.openWindow(url);
  }));
});
`

/*
ServiceWorker serves the dashboard's service worker.
*/
func ServiceWorker(w fhttp.ResponseWriter, _ *fhttp.Request) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Service-Worker-Allowed", "/")
	_, _ = w.Write([]byte(serviceWorker))
}
