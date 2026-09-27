# Contributing to Nostromo

Thanks for taking the time to improve **Nostromo**! 🛰️ Contributions of all
kinds are welcome — bug reports, feature ideas, documentation fixes and code.

## Getting started

1. Fork the repository and clone your fork.
2. Build and run the tests:

   ```sh
   make check   # gofmt + go vet + go test
   make build   # produces bin/nostromo
   ```

3. Create a topic branch for your change.

## Commit & PR conventions

This project follows the [Angular / Conventional Commits](https://www.conventionalcommits.org/) style:

- `feat: add NIP-xx support`
- `fix: correct filter matching for empty tags`
- `docs: clarify persistence behavior`
- `refactor: simplify frame writer`
- `test: add coverage for ephemeral events`
- `chore: bump toolchain`

Keep commits focused, and make sure `make check` passes before opening a pull
request.

## Reporting issues

When opening an issue, please include:

- The Nostromo version (`nostromo --help` banner or NIP-11 `version`).
- Steps to reproduce, expected and actual behavior.
- A minimal client transcript or event JSON when relevant.

Security-sensitive issues should **not** be filed in the public tracker;
contact the maintainer privately via the relay `contact` field.

## Code style

- Standard library only — please do not add third-party dependencies without
  discussing it in an issue first.
- Run `gofmt -s` and `go vet`.
- Add tests for new behavior and keep the public API small.

## Implementing a NIP

If you add support for a NIP:

1. Implement the protocol/store/relay logic with unit tests.
2. Add an end-to-end case where it affects the wire flow.
3. List the NIP number in `supported_nips` (see `internal/server/server.go`).
4. Document it in the README.

By contributing, you agree that your contributions are licensed under the
project's [MIT License](LICENSE).
