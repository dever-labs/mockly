# CLI Reference

```
mockly start       [--config <file>] [--ui-port <n>] [--api-port <n>]
mockly apply       --config <file>
mockly config      validate [file]
mockly generate    <spec-file> [-o <file>] [--http-port <n>]
mockly list
mockly add http    --method GET --path /foo --status 200 --body '{"ok":true}'
mockly add page    --path /checkout --file ./page.html | --dir ./site/docs
mockly delete      <mock-id>
mockly status
mockly reset
mockly preset      list | show <name> | use <name>
mockly scenario    list | active | activate <id> | deactivate <id>
mockly fault       set [--status <n>] [--delay <d>] [--delay-min <d>] [--delay-max <d>] [--body <s>] [--error-rate <f>] [--rate-limit <n>] [--rate-limit-status <n>] | clear | status
```

> The `fault` CLI subcommand only controls the **HTTP** protocol's direct
> fault (`/api/fault/http`). To inject faults on other protocols (DNS, gRPC,
> Redis, Kafka, etc.), use the [Management API](management-api.md#fault-injection) directly
> (`POST/GET/DELETE /api/fault/{protocol}`) or bundle them into a
> [scenario](../testing/scenarios.md).

---
