# Changelog

## v0.1.2

- Preserve HTTP status and structured error details from gateway responses.
- Export gateway error code constants.
- Return structured SDK errors for non-JSON gateway responses.
- Accept both `ok` and `OK` success codes.
- Enforce a five-minute webhook timestamp window by default.
- Expose webhook delivery number and signing timestamp.
- Document order query, close, error handling, and webhook idempotency.

## v0.1.1

- Add open source project files for GitHub distribution.

## v0.1.0

- Initial Go SDK release.
- Support signed order creation, order query, order close, and webhook signature verification.
