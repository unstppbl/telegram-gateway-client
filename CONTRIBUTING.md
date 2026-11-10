# Contributing

Bug reports, documentation improvements, and focused pull requests are welcome.
For larger API changes, open an issue first so we can agree on the public shape.

## Set up and check

Install Go 1.26 or later, then clone the repository:

```sh
git clone https://github.com/unstppbl/telegram-gateway-client.git
cd telegram-gateway-client
go test -race -cover ./...
go vet ./...
```

Format changed Go files with `gofmt`. CI requires every Go file to be formatted.
Tests must run without credentials, external network access, or paid API calls.
Use `httptest` servers and synthetic fixtures instead of real user data.

Keep the package small and idiomatic. Prefer standard-library facilities. Add
tests for new behavior and failure cases, and update the README, exported Go
documentation, and changelog when changing the public API. Check any changes
to provider behavior against the [Telegram Gateway API](https://core.telegram.org/gateway/api).

Never include tokens, real phone numbers, verification codes from real sessions,
or private callback payloads in issues, tests, or logs. See [SECURITY.md](SECURITY.md)
for vulnerability reports.

## Pull requests

Describe the problem, the resulting behavior, and how you validated the change.
Keep unrelated changes in separate pull requests. API changes before v1.0 may
break compatibility; document them clearly in the changelog.

## Maintainer release checklist

Releases are Go module tags. There is no binary or separate registry to publish.

1. Update `CHANGELOG.md` and the README installation version.
2. Run the checks above with the oldest supported Go release and the current
   stable release. Review the public API and check that no credentials or local
   artifacts are staged.
3. Merge the release commit to `main` and wait for its GitHub Actions CI run to
   pass. Release only that tested commit.
4. Create and push an annotated semantic-version tag, then create a GitHub
   release using the corresponding changelog entry:

   ```sh
   git switch main
   git pull --ff-only
   git tag -a v0.1.0 -m "Release v0.1.0"
   git push origin v0.1.0
   gh release create v0.1.0 --verify-tag --title "v0.1.0" --notes-file /path/to/release-notes.md
   ```

   Replace the example version for later releases. Never move or overwrite an
   existing release tag.
5. From a separate temporary module, verify the published version:

   ```sh
   go mod init example.com/gateway-smoke
   go get github.com/unstppbl/telegram-gateway-client@v0.1.0
   go list -m github.com/unstppbl/telegram-gateway-client
   ```

   Confirm that the tag is installable through the public Go module proxy and
   that its [Go reference page](https://pkg.go.dev/github.com/unstppbl/telegram-gateway-client)
   is available. Discovery can take time after the first release.
