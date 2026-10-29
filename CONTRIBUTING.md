# Contributing

The development environment is described in [docs/development.md](docs/development.md).

Pull requests pass the same checks as CI:

```bash
go run mvdan.cc/gofumpt@v0.12.0 -l .
go vet ./...
make check
```

Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/).

Changes to these cross-repo contracts are coordinated across the affected repos (protocol, verirelay, verifier, release):

- the `verirelay-guest-manifest/1` Guest Manifest;
- the `verirelay-build-report/1` Build Report;
- the launch profile;
- the OVMF build inputs, whose expected digest is pinned in the release repository's `versions.lock`.
