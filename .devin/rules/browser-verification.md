# Browser verification before handoff

Always verify UI work in a real browser before telling the user to look.

- Drive the app in a browser after any page, route or screen change: load the
  page, exercise the feature, take screenshots and confirm the expected
  result renders.
- Never hand over on compile/typecheck alone — a building page can still be
  blank, crash or misbehave. Check the browser console and network calls
  for errors, and verify the backend log has no request errors.
- Provide evidence: describe what was loaded, what was clicked, and what
  the screenshot/console showed.
