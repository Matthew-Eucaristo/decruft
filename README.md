# decruft

**See everything your developer machine has accumulated, and reclaim it the official way.**

decruft is a better "Storage" settings screen for programmers, in the terminal. One scan covers Go caches, Xcode and its simulators, Homebrew leftovers, CocoaPods/uv/pip caches, Docker, the Android SDK and Gradle, plus everything the family cleaners find: `node_modules` and JS caches ([nodekill](https://github.com/Matthew-Eucaristo/nodekill)), Flutter builds ([flutterkill](https://github.com/Matthew-Eucaristo/flutterkill)) and forgotten agent worktrees ([fell](https://github.com/Matthew-Eucaristo/fell)). It shows the whole disk as one bar with real developer categories, explains what macOS hides behind "System Data" and "purgeable", and removes only what you pick, through each tool's own cleanup command.

![decruft scanning a synthetic developer machine: the storage bar by category, the Xcode list with safety labels and the exact command per item, Flutter items handed back to flutterkill, a treemap zoomed into Xcode, then every safe item cleaned, the bars springing to their new sizes and a receipt with free space measured before and after](demo/demo.gif)

It is a single Go binary. macOS is first-class, Linux is supported, and Windows runs the cross-platform modules. decruft never contacts the network itself and writes no config, cache or state.

## Install

```sh
brew install matthew-eucaristo/tap/decruft
```

That one command is all you need: Homebrew installs `fell`, `nodekill` and `flutterkill` with it as dependencies, and decruft runs them for their areas (each still works on its own). With Go 1.26+: `go install github.com/Matthew-Eucaristo/decruft@latest` (then install the family tools you want; decruft finds them on `PATH` and skips the ones that are missing). Release archives for macOS, Linux and Windows are on the [releases page](https://github.com/Matthew-Eucaristo/decruft/releases).

## Use

```sh
decruft                  # interactive
decruft --dry-run        # interactive, but nothing changes
decruft --list           # plain table, read-only (the default when output is not a terminal)
decruft --json           # machine-readable, read-only; home paths are written as ~/…
decruft --yes            # headless: clean every safe item (never review items)
decruft ~/work ~/side    # roots for fell, nodekill and flutterkill (default: ~/dev)
decruft --no-family      # built-in modules only
```

**Overview.** A full-width storage bar like Apple's, but split into developer categories, then one row per category with what removing it frees, how much is safe and how much needs a look, and the biggest safe wins. `enter` opens a category, `space` selects it, `s` selects every safe item, `c` shows the plan.

**List.** Every item with its size, safety level and the exact command or action. The pane at the bottom says what the item is, exactly what will run, how it comes back and why it has its level.

**Treemap.** `t` draws a squarified treemap in truecolor blocks, corrected for the shape of terminal cells. Arrow keys move between tiles, `enter` zooms into a category, `esc` zooms out, `space` selects. Cleaned tiles stay in place, hatched, so the map does not jump.

**Receipt.** After a run, the bars spring to their new sizes and decruft shows free space before and after, measured from the disk. The receipt keeps three things apart, because they mean different things:

```
decruft · done in 2s
  ✓ Moved to Trash        621.8 MB    3 item(s)  freed when you empty the Trash; Put Back restores
  ✓ Freed now               1.8 GB    7 item(s)  xcrun simctl delete unavailable · go clean -cache · go clean -modcache · 4 more
  ✓ Delegated             667.9 MB    4 item(s)  flutterkill ×2 · nodekill ×2 (each tool's own rules)
    Free space  2.5 GB → 4.2 GB   +1.8 GB measured · about 621.8 MB more when you empty the Trash
    ↺ Build caches (Go, Homebrew, DerivedData) come back as you work; that is normal.
```

`?` explains every colour, column and key. The layout adapts down to 40 columns, `NO_COLOR` switches to glyphs only, and the first `ctrl+c` during a run stops after the current step (a second one quits).

## What it covers

Built-in modules use the one official command each area has. decruft shows that exact command before running it.

| Area | Items | How it is removed | Level |
|---|---|---|---|
| Go | build cache, module cache, golangci-lint cache | `go clean -cache`, `go clean -modcache`, `golangci-lint cache clean` | safe, regrows |
| Xcode | DerivedData per project, DeviceSupport per OS version | folder to the Trash, only while Xcode is not running | safe · review |
| Xcode | Archives | shown only: delete them in Xcode › Organizer (they hold the dSYMs for crash reports) | protected |
| Simulators | unavailable devices · devices · runtimes | `xcrun simctl delete unavailable` · `xcrun simctl delete <udid>` · `xcrun simctl runtime delete <id>` | safe · review · review (live while booted) |
| Homebrew | unused dependencies · old versions, downloads, logs | `brew autoremove`, then `brew cleanup --prune=all` (sized with their own `-n`) | review · safe |
| Package caches | CocoaPods, uv, pip | `pod cache clean --all`, `uv cache prune`, `pip cache purge` | safe |
| Docker | stopped containers, dangling images, build cache (never volumes) | `docker system prune --force`, sized by `docker system df` | review |
| Android | NDKs, system images · emulators | `sdkmanager --uninstall <pkg>` · `avdmanager delete avd -n <name>`; shown only without the SDK command-line tools | review |
| Gradle | `~/.gradle/caches`, wrapper distributions | shown only: Gradle expires unused entries itself and has no prune command | protected |
| Linux | apt downloads · unused packages · journal | shown with the command (`sudo apt-get clean`, `sudo apt-get autoremove`, `sudo journalctl --vacuum-time=2weeks`); decruft never runs sudo | protected |
| Linux | unused Flatpak runtimes (user) | `flatpak uninstall --user --unused` | review |

Family tools, run in parallel through their JSON contract ([fell's contract](https://github.com/Matthew-Eucaristo/fell/blob/main/docs/contract.md)):

| Tool | Area | How decruft hands items back |
|---|---|---|
| `fell` | git and agent worktrees | `fell apply --plan -` with `retire` (or `strip` when only that is allowed) |
| `nodekill` | `node_modules`, framework caches, package-manager caches, Node versions | `nodekill apply --plan -` |
| `flutterkill` | Flutter/Dart build outputs, pub/Gradle/FVM/Android Studio caches | `flutterkill apply --plan -` (0.3+); with 0.2, items are shown and `o` opens flutterkill to clean |

Every byte is offered once: when two sources report the same folder (or one inside the other), the built-in module wins, then the outer item, so fell's worktree hides nodekill's `node_modules` inside it. The sources of every command are in [docs/modules.md](docs/modules.md).

## What macOS does not show you

Settings' Storage screen lumps developer data into "Developer" and "System Data" and counts purgeable space as free. decruft shows:

- **Free space as the filesystem reports it** (statfs), which is what every before/after measurement uses.
- **Purgeable space**: what macOS can free by itself and what Finder adds to "available".
- **Inside System Data**: simulator runtimes (often several GB each), swap files and APFS local snapshots (`tmutil listlocalsnapshots /`, shown, never touched).
- **The Trash**, which still takes space until emptied. Reading it needs Full Disk Access for your terminal; without it decruft says so instead of guessing.
- **What removal frees now**: on APFS, blocks shared with clones and snapshots are left out (`ATTR_CMNEXT_PRIVATESIZE`), so a pnpm or Claude worktree that is mostly clones is not promised.

## Safety model

- **The official way only.** Caches are removed by their own tool's command, folders without one go to the Trash, and family items go back to their own tool, which re-checks them with its own rules. Commands that need `sudo`, and apps that manage their own data (Xcode Archives, Gradle), are shown with the way to remove them, never acted on.
- **Pinned to what you saw.** Every item carries a fingerprint of its state (path, folder identity, level, command). Before acting, decruft re-reads the item and refuses it if anything changed. It acts on the fresh copy only.
- **Live means hands off.** DerivedData while Xcode runs, a booted simulator or its runtime, the SDK while Android Studio runs: all shown as live and never selectable.
- **Trash by default.** `--permanent` deletes folders instead. Official commands delete by their own design, so the confirm sheet and the receipt keep them apart.
- **Fail closed.** An unknown safety level, an unreadable tool output or a family tool with another contract version means the item is protected or the source is skipped, with a warning.
- **Read-only modes.** `--list`, `--json` and `--dry-run` never change anything. Folder moves are refused outside the home folder, within two levels of it, and through symlinks.
- **No residue.** decruft writes no config, cache or state.

## Uninstall

```sh
brew uninstall decruft && brew autoremove   # also removes fell, nodekill and flutterkill if only decruft needed them
brew untap matthew-eucaristo/tap            # if you no longer use the tap
```

decruft writes nothing to your disk, so there is nothing else to remove. `brew autoremove` keeps any family tool you installed yourself with `brew install`. With `go install`: `rm "$(go env GOPATH)/bin/decruft"`.

## Demo

The GIF is recorded with [VHS](https://github.com/charmbracelet/vhs) against a synthetic machine: `demo/setup.sh` builds a fake HOME on a small disk image with stand-in `brew`/`xcrun`/`pod`/`uv`/`pip3` and the real `go`, `git` and family tools. Re-record with `vhs demo/demo.tape`.

## License

[MIT](LICENSE). Ideas, not code, from [Mole](https://github.com/tw93/Mole) (GPL); the project-artifact catalogs live in nodekill, flutterkill and fell.
