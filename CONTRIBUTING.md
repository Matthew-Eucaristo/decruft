# Contributing

Thanks for helping. decruft aims to stay small, honest and easy to audit.

## Development

```sh
gofmt -l .
go vet ./...
GOOS=linux go vet ./...
GOOS=windows go vet ./...
go test -race ./...
go run honnef.co/go/tools/cmd/staticcheck@latest ./...
go run . --dry-run
```

To try the TUI on a synthetic machine, run `source demo/setup.sh`, then `decruft`. When the TUI changes visibly, re-record the README GIF locally with `vhs demo/demo.tape` and check the frames before committing.

## Adding a module

A module is one area with **one official cleanup command**. It goes in `internal/module`, reads through `Env` only, and returns items whose `Action.Argv` is that command with the tool's absolute path. Add:

- a row in `docs/modules.md` with the command's documentation;
- tests with a fake `Env`, including what must never be offered (tool missing, unknown output, the app running);
- the module to `registry_<os>.go` for the systems where the command exists.

If the area has no official command, show it with the owner's way to remove it (`item.Manual` or `item.None`) and never act on it. Project artifacts (`node_modules`, build folders) belong in the family tools, not here.

## Ground rules

- All changes to the disk go through `internal/act`, which re-reads every item right before acting. Keep it that way.
- Read, never write, other tools' state. Unknown formats fail closed.
- Keep dependencies to Charm modules, go-colorful and `golang.org/x/sys`.
- No real paths, usernames or project names in code, tests, docs or screenshots. Use placeholders such as `~/dev/shop-web`.
- Pull requests are squash-merged, so the PR title becomes the release-notes entry. Use a conventional prefix: `feat:`, `fix:`, `docs:`, `test:`, `ci:` or `chore:` (the last four are left out of release notes).

## Reporting security issues

See [SECURITY.md](SECURITY.md).
