# examples

A sample schema, template, payload, and configuration showing
the four artifacts a hook2slack deployment needs.

## Files

| File | What |
|---|---|
| `schema.json` | JSON Schema describing the expected payload shape (`title`, `body`, `level`, `url`). |
| `template.tmpl` | Go `text/template` rendering one Slack attachment, with a colour chosen from `level` and an optional button from `url`. |
| `payload.json` | Sample request body matching the schema. |
| `hook2slack.yaml` | Configuration tying the above together, using `/etc/hook2slack/...` as placeholder absolute paths. |

## Preview the rendered message

Without starting the server or sending to Slack:

```
hook2slack render \
  --schema examples/schema.json \
  --template examples/template.tmpl \
  --payload examples/payload.json
```

## Run end-to-end

Deploy the four files to absolute paths the hook2slack process
can read. A typical layout:

```
/etc/hook2slack/hook2slack.yaml
/etc/hook2slack/schema.json
/etc/hook2slack/template.tmpl
/etc/hook2slack/slack.url        # contains a Slack incoming webhook URL
```

Start the server:

```
hook2slack serve --config /etc/hook2slack/hook2slack.yaml
```

Send the sample payload from another terminal:

```
curl -X POST -H 'Content-Type: application/json' \
  --data @payload.json \
  http://localhost:9101/webhooks/default
```

A Slack message should appear in the channel bound to the
webhook URL.
