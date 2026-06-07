# Days Left This Year plugin

Type: `days_left_year`. Shows how much of the current year has passed and how
much remains: big "Days Passed" / "Days Left" numbers, a dot grid of the whole
year with today highlighted, and a footer with the year. Modeled on the TRMNL
"Days Left This Year" screen. Performs no network access.

[← All plugins](../PLUGINS.md)

## Settings

| Key        | Type   | Default      | Notes                                               |
|------------|--------|--------------|-----------------------------------------------------|
| `timezone` | string | server local | IANA name, e.g. `Europe/Berlin`; decides "today"    |
| `label`    | string | (empty)      | Small text shown in the top-right corner            |

```json
{ "label": "2026", "timezone": "Europe/Berlin" }
```

Cache TTL hint: ~1 hour. The content only changes once per day; the
content-hashed image means re-renders within a day produce the same file.

## How to test

```sh
go run ./cmd/trmnl-render -plugin days_left_year -settings '{"label":"2026"}' -out daysleft
# open daysleft.png
```

Or in the admin UI: **Screens → create (Days Left This Year) → Render preview**.

## Notes

- Past days are drawn darker, future days lighter, and today as a solid black
  dot. Leap years render 366 dots; the grid columns are chosen automatically so
  the cells stay roughly square.
- "Today" is computed from the configured `timezone` (server local if unset or
  invalid). The binary embeds the timezone database (`time/tzdata`).
