# hook2slack - Design

For what the tool is and how to use it, see [README.md](README.md).
This document covers what is not visible from reading the code:
the shape the design takes and things deliberately not done.

## Design

hook2slack implements: receive a webhook payload as a data
structure, build the Slack message as a data structure, send it
to Slack. The Go code does exactly that.

Independence from any specific webhook source falls out of this
shape. The fields, enums, and identifiers a particular source
uses have nothing to do with HTTP, JSON, or Slack delivery, so
they do not appear in the code. They live in the JSON Schema
(the input data structure) and the Go `text/template` (the
output data structure), both external files that travel with
the deployment.

## Non-goals

These are intentional exclusions. Each is something hook2slack
could reasonably do, but does not.

- **Multiple schemas or templates per process.** Run multiple
  processes if multiple are required.

- **Conditional logic in the configuration.** Conditions belong
  in the template (using `{{ if }}`) or in the upstream sender.

- **Filtering or muting at the adapter layer.** If a webhook
  arrives, it is rendered and posted. Suppression belongs
  upstream.

- **A retry queue or persistent buffer for failed Slack
  deliveries.** The upstream sender retries by re-sending on its
  own schedule.

- **Custom template functions beyond `json`.** Path accessors,
  string manipulation helpers, date formatters, and other
  helpers are not provided. `json` is included because the
  template's output is a JSON document and the function performs
  the quoting and escaping JSON syntax requires; other helpers
  would extend the template language with capabilities
  orthogonal to that output format.

- **Authentication on the HTTP API.** Put a reverse proxy in
  front for access control.

- **In-process metrics endpoint.** Request totals, error rates,
  and latency are already produced by the reverse proxy in front
  of hook2slack or by any access-log shipper. An in-process
  surface would duplicate that without adding visibility.

- **Multiple Slack destinations from one endpoint.** To fan out,
  use multiple endpoints (each with its own webhook URL) and
  have the upstream sender post to each.

- **Aggregation, grouping, or de-duplication of webhooks.** Each
  incoming POST produces one outgoing Slack message. Batching
  belongs upstream.

