# Roadmap

Loose, non-binding notes on possible future work. Nothing here is committed to.

## Using official / community TRMNL plugins

TRMNL's official, public, and private ("recipe") plugins are defined as
[Liquid](https://shopify.github.io/liquid/) templates plus the TRMNL HTML/CSS
[design framework](https://github.com/usetrmnl/framework), rendered to the
800x480 bitmap by a headless browser (this is what the TRMNL cloud and the local
`trmnlp` dev tool do). go-trmnl deliberately uses a **pure-Go image pipeline with
no headless browser** (see `docs/PLAN.md`), drawing screens natively with
`fogleman/gg` and rasterized SVG. So official plugins cannot run here unchanged.
Three ways to bridge that gap:

### 1. Native Go reimplementations (current approach)

Reproduce specific plugins we care about as native Go plugins (as done for
`clock`, `weather`, `familycalendar`). No new heavy dependencies; does not
"use" the official definitions, just reproduces selected ones. Good when only a
handful of plugins are wanted. This is the near-term direction.

### 2. TRMNL framework renderer (opt-in)

Add an optional plugin type that polls a JSON URL, renders a Liquid template
against the open `usetrmnl/framework` CSS, and rasterizes the resulting HTML to
a PNG via headless Chromium (e.g. `chromedp`), then to 1-bit. This is the only
path that genuinely unlocks the official/community library. Trade-off: it breaks
the "no Chromium" decision (a browser binary, more memory/CPU on a small box),
so it would be strictly opt-in, with the native plugins staying pure-Go.
Building blocks: a Go Liquid engine (e.g. `osteele/liquid`), the framework CSS,
a headless renderer, and the poll/webhook/static data strategies.

### 3. TRMNL Cloud pass-through proxy (opt-in, per screen)

Let a screen be served by TRMNL's hosted platform while the device still talks
only to the local server. go-trmnl already speaks the device protocol; the
device authenticates with an `ID` header (its MAC) and an `Access-Token` header
(its `api_key`), and `/api/display` returns an `image_url`.

A "TRMNL Cloud" screen type would store credentials for a device registered in a
TRMNL account (its id / access token and the cloud base URL). On render,
go-trmnl proxies a display request to the cloud API with those credentials,
takes the returned `image_url`, and re-serves the image locally (fetched and
cached like any other rendered screen, keyed by content hash). The local device
never needs cloud credentials of its own.

Notes and open questions:

- This is explicitly optional and per screen. Running a **mix of BYOS and Cloud
  screens is fine and is the user's decision.**
- Auth/identity: a (possibly virtual) device must exist in the TRMNL account to
  proxy; decide whether to reuse the physical device's identity or a separate
  cloud device token. Credentials are sensitive (encrypt at rest, like calendar
  tokens via `internal/secret`).
- Refresh/caching: honor the cloud's suggested refresh interval; cache the
  fetched image so repeated device polls do not hammer the cloud.
- Dependency: such a screen relies on TRMNL's uptime and rate limits.
- Likely shape: a new `internal/plugins/trmnlcloud` plugin whose `DataModel`
  fetches the cloud `image_url`, plus a small client; admin UI to enter the
  device id / token / base URL.

## Calendar

- CalDAV is implemented for Apple iCloud and generic servers. Validate the
  discovery/query path against a real iCloud account (currently only the iCal
  event mapping is unit-tested).
- Possible: per-screen layout option (agenda list vs. day panels).
