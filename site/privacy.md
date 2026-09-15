---
layout: default
title: Privacy Policy
description: How go-trmnl handles data, including Google user data.
---

# Privacy Policy

_Last updated: 2026-09-15_

go-trmnl is open-source, self-hosted software for [TRMNL](https://usetrmnl.com/)
e-ink displays. This policy describes what the software does with data. It
applies to the go-trmnl source code and releases published at
<https://github.com/gesellix/go-trmnl>.

## Who processes your data

go-trmnl runs entirely on a server operated by whoever installed it (the
"operator"), typically on a home network. The authors of go-trmnl do not run a
hosted service, do not operate any server that go-trmnl reports to, and receive
no data from installations. There is no telemetry, analytics, or advertising.

The operator is responsible for their installation and for the people whose
data they connect to it.

## Google user data

The optional Family Calendar plugin can connect Google accounts through OAuth.

- **Scope:** read-only access to Google Calendar
  (`https://www.googleapis.com/auth/calendar.readonly`). go-trmnl cannot create,
  change, or delete calendar data.
- **Data accessed:**
  - The list of calendars (IDs and names). The primary calendar ID identifies
    the account's email address.
  - Events of the calendars the operator selects: title, start and end time,
    all-day flag, location, status, and event ID.
- **Use:** only to display upcoming events on the operator's TRMNL display and
  in the operator's admin interface.
- **Storage:** events are cached in the local database on the operator's
  server for a window of roughly one day in the past to 60 days ahead, and are
  replaced on each sync. OAuth tokens are stored in the same database,
  encrypted at rest by default.
- **Sharing:** Google user data is not sold, not shared with third parties, not
  used for advertising, and not used to train AI or machine learning models. It
  is not transferred anywhere except between Google's APIs and the operator's
  server.
- **Deletion:** removing an account in the admin interface deletes its stored
  tokens and cached events. Access can also be revoked at any time under
  [Google Account → Security → Third-party connections](https://myaccount.google.com/connections).

go-trmnl's use and transfer of information received from Google APIs adheres to
the [Google API Services User Data Policy](https://developers.google.com/terms/api-services-user-data-policy),
including the Limited Use requirements.

## Other data

Depending on the plugins an operator enables, the server stores or requests:

- **Device data:** TRMNL device identifiers (MAC address, API key), reported
  status such as battery level and firmware version, and device log entries
  (kept for 32 days by default).
- **CalDAV accounts** (e.g. iCloud): username, app-specific password (encrypted
  at rest by default), and events, handled like Google events above.
- **External services** contacted by optional plugins, without personal data
  beyond what the request itself contains (such as the server's IP address and
  a location entered by the operator): Open-Meteo (weather and geocoding),
  ZenQuotes and stoic-quotes.com (quotes).

## Changes

Changes to this policy are tracked in the project's [Git history](https://github.com/gesellix/go-trmnl/commits/main/site/privacy.md).

## Contact

Questions: open an issue at <https://github.com/gesellix/go-trmnl/issues>.
