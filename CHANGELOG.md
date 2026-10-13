# Changelog

## Unreleased

### Security

- Validate and escape caller-provided admin API path segments for tier,
  scanner, inventory, and node operations. Invalid values are rejected before
  sending a request. See
  [issue #689](https://github.com/minio/madmin-go/issues/689) and
  [pull request #690](https://github.com/minio/madmin-go/pull/690).
