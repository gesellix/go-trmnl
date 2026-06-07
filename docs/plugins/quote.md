# Quote plugin

Type: `quote`. Shows a quotation from a selectable provider. Each provider is
normalized to a common `{text, author}` shape, so the screen looks the same
whichever source you pick.

[← All plugins](../PLUGINS.md)

## Settings

| Key            | Type   | Default        | Notes                                                            |
|----------------|--------|----------------|------------------------------------------------------------------|
| `provider`     | string | `motivational` | `motivational`, `zenquotes`, `stoic`, or `custom`                |
| `mode`         | string | `random`       | ZenQuotes only: `random` or `today`                              |
| `url`          | string | (empty)        | `custom` only: JSON endpoint to poll                             |
| `text_field`   | string | `text`         | `custom` only: JSON key holding the quote text                   |
| `author_field` | string | `author`       | `custom` only: JSON key holding the author                       |
| `label`        | string | (empty)        | Small text shown bottom-left                                     |

Cache TTL hint: ~6 hours (also keeps keyless ZenQuotes within its rate limit).

### Providers

- **`motivational`** — a built-in, offline list shipped in the binary. No
  network, no API key, no rate limits. Rotates daily.
- **`zenquotes`** — [zenquotes.io](https://zenquotes.io), keyless. `random` or
  the daily `today` quote. A small `zenquotes.io` credit is shown on screen, as
  their free tier requires.
- **`stoic`** — [stoic-quotes.com](https://stoic-quotes.com), keyless.
- **`custom`** — poll any JSON endpoint and map its fields. If the response is a
  JSON array, the first element is used (so ZenQuotes-shaped APIs work too).

```json
{ "provider": "zenquotes", "mode": "today" }
```

```json
{ "provider": "stoic", "label": "Daily Stoic" }
```

```json
{ "provider": "custom", "url": "https://example.com/quote.json", "text_field": "quote", "author_field": "by" }
```

### CUT/daily and other niche sources

There is no dedicated provider for some sources (e.g. the
[CUT/daily](https://trmnl.com/recipes/248986) quote of the day for film/TV
editors). Use the `custom` provider pointed at the source's JSON feed with the
matching `text_field`/`author_field`. Adding a first-class provider for a new
source is a small code change in `internal/plugins/quote/providers.go`.

## How to test

```sh
go run ./cmd/trmnl-render -plugin quote -settings '{"provider":"motivational"}' -out quote
# live providers (need internet):
go run ./cmd/trmnl-render -plugin quote -settings '{"provider":"zenquotes","mode":"today"}' -out quote
go run ./cmd/trmnl-render -plugin quote -settings '{"provider":"stoic"}' -out quote
```

The unit tests in `internal/plugins/quote/` exercise each provider against an
`httptest` server, and render a long quote to check word-wrapping.

## Notes

- The quote is word-wrapped and the font size steps down for longer text.
- A provider failure (network/parse) surfaces as the placeholder image on a
  device. The `motivational` provider never fails (no network).
