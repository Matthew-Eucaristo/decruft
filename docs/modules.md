# Built-in modules: commands and sources

Each module offers only what its tool can remove with its own documented command. Where there is no such command, the item is shown with the owner's way to remove it, or not offered. Commands run with stdin closed and a quiet environment (`HOMEBREW_NO_AUTO_UPDATE`, `HOMEBREW_NO_INSTALL_CLEANUP`, `NO_COLOR`, `PIP_NO_INPUT`, …), so nothing prompts, updates itself or phones home.

| Module | Read with | Removed with | Source |
|---|---|---|---|
| Go build cache | `go env -json GOCACHE GOMODCACHE` | `go clean -cache` (includes test results) | [go clean](https://pkg.go.dev/cmd/go#hdr-Remove_object_files_and_cached_files) |
| Go module cache | same | `go clean -modcache` (the files are read-only; plain `rm` fails) | same |
| golangci-lint cache | `golangci-lint cache status` | `golangci-lint cache clean` | `golangci-lint cache --help` |
| Xcode DerivedData, DeviceSupport | folder listing | moved to the Trash while Xcode is not running; Xcode recreates DerivedData on the next build and copies DeviceSupport again when a device connects | Xcode › Settings › Locations; macOS Settings › Storage › Developer removes the same folders |
| Xcode Archives | folder listing | not removed: Xcode › Window › Organizer › Archives | they contain the dSYMs needed to symbolicate crash reports |
| Simulator devices | `xcrun simctl list devices -j` | `xcrun simctl delete unavailable`, `xcrun simctl delete <udid>` | `xcrun simctl help delete` |
| Simulator runtimes | `xcrun simctl runtime list -j` (`sizeBytes`, `deletable`) | `xcrun simctl runtime delete <id>`; not deletable → Xcode › Settings › Components | `xcrun simctl help runtime`; [Installing additional simulator runtimes](https://developer.apple.com/documentation/xcode/installing-additional-simulator-runtimes) |
| Homebrew | `brew autoremove -n`, `brew cleanup --prune=all -n` | `brew autoremove`, then `brew cleanup --prune=all` | [brew manpage](https://docs.brew.sh/Manpage) |
| CocoaPods | `~/Library/Caches/CocoaPods` (or `CP_CACHE_DIR`) | `pod cache clean --all` | [pod cache clean](https://guides.cocoapods.org/terminal/commands.html#pod_cache_clean) |
| uv | `uv cache dir` | `uv cache prune` (unused entries only; hand-deleting leaves the cache inconsistent) | [uv cache](https://docs.astral.sh/uv/concepts/cache/) |
| pip | `pip3 cache dir` (skipped when caching is disabled) | `pip3 cache purge` | [pip cache](https://pip.pypa.io/en/stable/cli/pip_cache/) |
| Docker | `docker system df --format '{{json .}}'` | `docker system prune --force` (stopped containers, unused networks, dangling images, build cache; never volumes) | [docker system prune](https://docs.docker.com/reference/cli/docker/system/prune/) |
| Android SDK packages | `ndk/*`, `system-images/*/*/*` under `ANDROID_HOME` | `sdkmanager --uninstall "<package>"` from the SDK's command-line tools | [sdkmanager](https://developer.android.com/tools/sdkmanager) |
| Android emulators | `~/.android/avd/*.avd` | `avdmanager delete avd -n <name>` | [avdmanager](https://developer.android.com/tools/avdmanager) |
| Gradle | `~/.gradle/caches`, `~/.gradle/wrapper/dists` | not removed: Gradle cleans unused entries itself | [Gradle user home cleanup](https://docs.gradle.org/current/userguide/directory_layout.html) |
| apt (Linux) | `/var/cache/apt/archives`, `apt-get -s autoremove` | shown only: `sudo apt-get clean`, `sudo apt-get autoremove` | `apt-get(8)` |
| journald (Linux) | `journalctl --disk-usage` | shown only: `sudo journalctl --vacuum-time=2weeks` | `journalctl(1)` |
| Flatpak (Linux) | `flatpak list --user --runtime` | `flatpak uninstall --user --unused --noninteractive` | [flatpak uninstall](https://docs.flatpak.org/en/latest/flatpak-command-reference.html) |

## Disk explanation (macOS)

| Number | Read with | Note |
|---|---|---|
| Free | `statfs` on the home volume | every before/after measurement |
| Purgeable | `NSURLVolumeAvailableCapacityForImportantUsageKey − NSURLVolumeAvailableCapacityKey`, through JavaScript for Automation | [Apple](https://developer.apple.com/documentation/foundation/urlresourcekey/volumeavailablecapacityforimportantusagekey) |
| Local snapshots | `tmutil listlocalsnapshots /` | listed only |
| Swap | sizes of `/private/var/vm/swapfile*` | part of System Data |
| What removal frees | `getattrlist(ATTR_CMNEXT_PRIVATESIZE)` per file | blocks shared with APFS clones and snapshots are left out |
