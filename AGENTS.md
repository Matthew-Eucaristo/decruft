# decruft

The family's umbrella TUI/CLI: one scan of everything a developer machine accumulates, an Apple-style storage bar with developer categories, a list with the exact command per item, a squarified treemap, and a measured receipt. Go + Bubble Tea v2 + Lip Gloss v2 + Bubbles v2 (spinner) + harmonica (springs) + go-colorful. macOS first, Linux second, Windows: cross-platform modules only. Family rules: `../AGENTS.md`.

## Layout
- `internal/item`: the Item model (level, action kind, delegation, fingerprint `Seal()` over ID/kind/path/level/action + `Evidence`) and categories with their colours.
- `internal/module`: built-in modules, read-only. `Env` abstracts the machine (LookPath, Output, Running, Measure) for tests. One file per area (`golang.go`, `homebrew.go`, `xcode_darwin.go`, `packages.go`, `docker.go`, `android.go`, `linux.go`); `registry_<os>.go` picks them per OS. `docs/modules.md` lists every command and its source.
- `internal/family`: discovery (`capabilities --json`, schema 1), `scan --json` → items, `apply --plan -` for fell and nodekill; legacy flutterkill `--json` → handoff items (opened with `o`).
- `internal/inventory`: runs every source in parallel, streams events, `Visible` + `Dedupe` (one owner per path: built-in with a command > family tool > built-in info; outer beats inner; symlinks resolved).
- `internal/act`: the only code that changes anything. `Plan` (priority order, one batch per tool), `Do`: re-scan the module without sizing, fingerprint match, act on the fresh copy, guard folder moves, measure statfs before/after; family batches via `Family.Apply`.
- `internal/disk`: statfs, purgeable (JXA → Foundation), local snapshots, swap, Trash size. `internal/report`: `--list`, `--json`, receipt. `internal/ui`: model/keys/view, `treemap.go` (squarify + snap + nearest), `tmview.go`, `style.go` (palette, storage bar with sub-cell boundaries, springs).
- `internal/size`, `format`, `pathx`, `trash`: shared with nodekill (same author, MIT).

## Invariants
- Built-ins remove only through the tool's documented command (absolute path from LookPath, fixed argv, quiet env, stdin closed) or, for DerivedData/DeviceSupport, a folder move to the Trash. No command → shown (Manual/None), never acted on. `sudo` commands are shown, never run.
- Family items are handed back by `{id, fingerprint[, action]}` only; decruft never passes `--force`/`--unlock`. Unknown level → protected; unknown schema or unreadable output → source skipped with a warning.
- Every action re-reads its module and refuses on fingerprint mismatch; it acts on the fresh item, never on the one it was handed. Folder moves: absolute, inside home, ≥3 levels deep, not a symlink.
- `--list`, `--json`, `--dry-run` never change anything; `--yes` takes safe items only. Default Trash; `--permanent` deletes folders.
- Receipt keeps Trash, freed-now (commands) and delegated apart; free space is measured; a command that frees far less than promised is flagged "claimed, not freed".
- No state, config or network. JSON writes home paths as `~/…`.
- TUI: fell's look (● ▲ ■ ◉ badges, `?` help, cursor ▌), truecolor only for category data (bar, treemap); fits 40+ columns; NO_COLOR → glyphs; first ctrl+c stops after the running step.

## Verify
`gofmt -l .` (empty), `go vet ./...`, `GOOS=linux go vet ./...`, `GOOS=windows go vet ./...`, `go test -race ./...`, `go run honnef.co/go/tools/cmd/staticcheck@latest ./...`, `go run golang.org/x/vuln/cmd/govulncheck@latest ./...`. Tests swap the trash (`Executor.MoveToTrash`) and the command runner; never test removal on real folders. On this machine use `--list`, `--json` or `--dry-run`.

## Demo
`demo/setup.sh` builds a fake HOME on an APFS sparse image with fixtures, stand-in brew/xcrun/pod/uv/pip3 and the real go/git/family tools, checks that every reported path is inside it, then `exec env -i` into a clean shell. `vhs demo/demo.tape` re-records `demo/demo.gif` locally (never in CI). Check frames for anything from the real machine before committing.
