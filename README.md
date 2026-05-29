# hook2slack

A webhook receiver that validates incoming JSON against a JSON
Schema and renders it to a Slack message through a Go
`text/template`. The Go code knows only HTTP, JSON, JSON Schema,
Go templates, and Slack incoming webhooks; the shape of any
specific webhook source lives in two external files (a schema and
a template).

For non-goals and design rationale, see [DESIGN.md](DESIGN.md).

## Install

```
go install github.com/zinrai/hook2slack@latest
```

## Subcommands

| Subcommand | Purpose |
|---|---|
| `serve --config <path>` | Run the HTTP server. |
| `check --config <path>` | Validate config, schema, and template without starting the server. Intended for CI. |
| `render --schema <path> --template <path> --payload <path>` | Render a sample payload through a given schema and template and print the JSON body that would be POSTed to Slack. No network request. |
| `version` | Print build version. |

## Configuration

```yaml
listen: ":9101"
schema_file: /etc/hook2slack/schema.json
template_file: /etc/hook2slack/template.tmpl
endpoints:
  - path: /webhooks/notify
    slack:
      url_file: /etc/hook2slack/notify.url
  - path: /webhooks/alert
    slack:
      url_file: /etc/hook2slack/alert.url
```

| Field | Required | Notes |
|---|---|---|
| `listen` | yes | HTTP listen address. |
| `schema_file` | yes | Absolute path to the JSON Schema file (Draft 2020-12). |
| `template_file` | yes | Absolute path to the Go `text/template` file. |
| `endpoints` | yes | At least one endpoint. |
| `endpoints[].path` | yes | URL path. Must start with `/`. Cannot be `/-/healthy` or `/-/ready`. |
| `endpoints[].slack.url_file` | yes | Absolute path to a file containing the Slack incoming webhook URL. Manage as a secret. |

Configuration changes require a process restart; there is no hot
reload.

## Schema file

A standard [JSON Schema](https://json-schema.org/) document
(Draft 2020-12) describing the expected shape of the JSON the
sender will POST. Every request is validated before rendering;
failures return HTTP 400.

A sample is at [examples/schema.json](examples/schema.json).

## Template file

A Go [`text/template`](https://pkg.go.dev/text/template) that
renders one Slack attachment as a JSON object. The server wraps
it as `{"attachments": [<rendered>]}` and POSTs to the Slack
incoming webhook URL.

Templates have the standard Go template built-ins (`index`,
`range`, `if`, `with`, comparison operators) and one custom
function:

| Function | Purpose |
|---|---|
| `json` | Encodes its argument as a JSON value, performing the quoting and escaping JSON syntax requires. Use to embed strings, numbers, and objects into the rendered JSON: `{{ .x \| json }}`. |

A sample is at [examples/template.tmpl](examples/template.tmpl).

## Try it

The `examples/` directory ships a sample schema, template,
payload, and configuration. To preview the rendered Slack
message without starting the server:

```
hook2slack render \
  --schema examples/schema.json \
  --template examples/template.tmpl \
  --payload examples/payload.json
```

See [examples/README.md](examples/README.md) for a runnable
deployment layout.

## HTTP API

### Webhook endpoints

| Method | Path | Status | Meaning |
|---|---|---|---|
| POST | `/<configured-path>` | 200 | Rendered and posted to Slack. |
| POST | `/<configured-path>` | 400 | JSON decoding or schema validation failed. |
| POST | `/<configured-path>` | 405 | Method other than POST. |
| POST | `/<configured-path>` | 500 | Template rendering or Slack POST failed. |

### Health endpoints

| Method | Path | Notes |
|---|---|---|
| GET | `/-/healthy` | Liveness. Always 200. |
| GET | `/-/ready` | Readiness. Always 200. |

The HTTP endpoints are unauthenticated. Put a reverse proxy in
front for access control and for request/error/latency
observability.

## License

This project is licensed under the [MIT License](./LICENSE).
