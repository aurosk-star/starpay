# Contributing

Thanks for helping improve StarPay Go SDK.

## Development

Run tests before submitting changes:

```bash
go test ./...
```

Keep the SDK small and stable. Do not add payment gateway server code or frontend code to this repository.

## Compatibility

Public APIs should remain backward compatible within the same major version. If a breaking change is required, document it in `CHANGELOG.md`.

## Pull Requests

Include:

- A short summary of the change.
- Test results.
- Any API or behavior changes.
