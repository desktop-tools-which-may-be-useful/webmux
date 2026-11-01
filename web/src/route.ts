// Route matching for the single-page app. It only looks at the pathname, so
// pages that read `location.search` or `#hash` keep their query intact.
export type Route =
  | { name: "terminal"; id: string }
  | { name: "code"; id: string }
  | { name: "settings" }
  | { name: "settings-tailscale" }
  | { name: "settings-tunnel" }
  | { name: "sessions" };

// Unknown paths fall back to the session list; there is no 404 page.
export function matchRoute(pathname: string): Route {
  const session = /^\/s\/([a-z2-7]{8})$/.exec(pathname);
  if (session?.[1]) return { name: "terminal", id: session[1] };
  const code = /^\/code\/([a-z2-7]{8})$/.exec(pathname);
  if (code?.[1]) return { name: "code", id: code[1] };
  if (pathname === "/settings/tailscale") return { name: "settings-tailscale" };
  if (pathname === "/settings/tunnel") return { name: "settings-tunnel" };
  if (pathname === "/settings") return { name: "settings" };
  return { name: "sessions" };
}
