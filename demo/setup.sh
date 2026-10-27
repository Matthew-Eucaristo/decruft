#!/usr/bin/env bash
# Builds a synthetic developer machine for the demo recording: a fake HOME
# (on macOS a small APFS disk image, so free space and the Trash are the
# image's own), Go caches, Xcode DerivedData and simulators, Homebrew
# leftovers, package caches, Node and Flutter projects and an agent
# worktree. Stand-in brew/xcrun/pod/uv/docker answer from these fixtures and
# act only on them; go, git and the family tools are the real ones. Nothing
# from the real machine appears in the recording.
# Usage: source demo/setup.sh   (from the repository root)
# Remove: hdiutil detach /tmp/decruft-demo/home (macOS); rm -rf /tmp/decruft-demo
set -e

DEMO="$(cd /tmp && pwd -P)/decruft-demo"
[ -d "$DEMO/home" ] && command -v hdiutil >/dev/null && { hdiutil detach -quiet "$DEMO/home" 2>/dev/null || true; }
[ -d "$DEMO" ] && chmod -R u+w "$DEMO" && rm -rf "$DEMO"
mkdir -p "$DEMO/bin" "$DEMO/home"
go build -trimpath -o "$DEMO/bin/decruft" .
for t in fell nodekill flutterkill go git; do
  p="$(command -v "$t")" && ln -s "$p" "$DEMO/bin/$t"
done
if command -v hdiutil >/dev/null; then
  hdiutil create -quiet -size 6g -fs APFS -volname demo -type SPARSE "$DEMO/home.sparseimage"
  hdiutil attach -quiet -nobrowse -mountpoint "$DEMO/home" "$DEMO/home.sparseimage"
fi
H="$DEMO/home"

export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 GIT_OPTIONAL_LOCKS=0
export GIT_AUTHOR_NAME=demo GIT_AUTHOR_EMAIL=demo@example.com GIT_COMMITTER_NAME=demo GIT_COMMITTER_EMAIL=demo@example.com
blob() { mkdir -p "$1" && dd if=/dev/urandom of="$1/${3:-blob}" bs=1048576 count="$2" status=none; }
stamp_ago() { local t=$(($(date +%s) - $1 * 86400)); date -d "@$t" +%Y%m%d%H%M 2>/dev/null || date -r "$t" +%Y%m%d%H%M; }
iso_ago() { date -u -d "@$(($(date +%s) - $1 * 86400))" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -v-"$1"d +%Y-%m-%dT%H:%M:%SZ; }
age() { find "$1" -exec touch -h -t "$(stamp_ago "$2")" {} + 2>/dev/null; }
repo() { # dir days gitignore
  mkdir -p "$1/src" && echo 'export default {}' > "$1/src/index.js"
  printf '%b' "$3" > "$1/.gitignore"
  git init -q -b main "$1" && git -C "$1" add -A
  GIT_AUTHOR_DATE=$(iso_ago "$2") GIT_COMMITTER_DATE=$(iso_ago "$2") git -C "$1" commit -q -m init
}

# ── Go: the real go finds these through HOME ──
GC="$H/Library/Caches/go-build"
echo "This directory holds cached build artifacts from the Go build system." > "$(mkdir -p "$GC" && echo "$GC")/README"
for d in 0a 3f 7c b2 e9; do blob "$GC/$d" 120 "$d-d"; done
blob "$H/go/pkg/mod/cache/download/github.com/acme/api/@v" 260 v1.4.0.zip
blob "$H/go/pkg/mod/github.com/acme/api@v1.4.0" 90 api.go
chmod -R a-w "$H/go/pkg/mod/github.com"

# ── Xcode: DerivedData per project, DeviceSupport, Archives, simulators ──
DEV="$H/Library/Developer"
blob "$DEV/Xcode/DerivedData/ShopApp-bqzdlxkfmcvnwhatyrpeoiusgjxa/Build" 340
blob "$DEV/Xcode/DerivedData/WeatherKit-dkfjqpwoeirutyalskdjfhgzmxnc/Index.noindex" 150
age "$DEV/Xcode/DerivedData/WeatherKit-dkfjqpwoeirutyalskdjfhgzmxnc" 120
blob "$DEV/Xcode/DerivedData/ModuleCache.noindex" 90
blob "$DEV/Xcode/iOS DeviceSupport/iPhone15,2 17.5 (21F79)/Symbols" 180
blob "$DEV/Xcode/Archives/2026-03-02/ShopApp.xcarchive" 60
SIM_OLD=6E3F0D1A-1111-4C5B-9A0B-0D5E7F1A2B3C
SIM_NEW=9B2C4D6E-2222-4F70-8A1B-2C3D4E5F6A7B
blob "$DEV/CoreSimulator/Devices/$SIM_OLD/data" 220
blob "$DEV/CoreSimulator/Devices/$SIM_NEW/data" 70

# ── Homebrew leftovers (stand-in prefix inside HOME) ──
BREW="$H/.brew"
blob "$BREW/Cellar/icu4c@76/76.1" 70
blob "$BREW/Cellar/python@3.12/3.12.9" 110
blob "$H/Library/Caches/Homebrew/downloads" 280

# ── package caches ──
blob "$H/Library/Caches/CocoaPods/Pods/Release" 90
blob "$H/.cache/uv/wheels-v5" 130
blob "$H/Library/Caches/pip/http-v2" 40

# ── projects for the family tools: nodekill, flutterkill, fell ──
P="$H/dev/shop-web"
repo "$P" 75 'node_modules/\n.next/\n.claude/worktrees/\n'
printf '{"name":"shop-web","private":true,"dependencies":{"next":"16.0.0"}}\n' > "$P/package.json"
echo '{}' > "$P/package-lock.json" && echo 'export default {}' > "$P/next.config.ts"
git -C "$P" add -A && GIT_AUTHOR_DATE=$(iso_ago 75) GIT_COMMITTER_DATE=$(iso_ago 75) git -C "$P" commit -q -m deps
blob "$P/node_modules/next" 260 && echo '{}' > "$P/node_modules/.package-lock.json"
blob "$P/.next/cache" 120
age "$P" 75
git -C "$P" worktree add -q --detach "$P/.claude/worktrees/agent-4f1c9a2e"
blob "$P/.claude/worktrees/agent-4f1c9a2e/node_modules" 150
age "$P/.claude/worktrees/agent-4f1c9a2e" 60

P="$H/dev/mobile-app"
repo "$P" 40 'build/\n.dart_tool/\n'
printf 'name: mobile_app\nenvironment:\n  sdk: ^3.9.0\ndependencies:\n  flutter:\n    sdk: flutter\n' > "$P/pubspec.yaml"
git -C "$P" add -A && GIT_AUTHOR_DATE=$(iso_ago 40) GIT_COMMITTER_DATE=$(iso_ago 40) git -C "$P" commit -q -m pubspec
blob "$P/build/app/outputs" 210 && blob "$P/.dart_tool" 40
age "$P" 40

# ── stand-in tools ──
cat > "$DEMO/bin/brew" <<EOF
#!/bin/sh
case "\$*" in
  "--cellar") echo "$BREW/Cellar" ;;
  "autoremove -n") [ -d "$BREW/Cellar/icu4c@76" ] && printf '==> Would autoremove 2 unneeded formulae:\nicu4c@76\npython@3.12\n' ;;
  "autoremove") rm -rf "$BREW/Cellar/icu4c@76" "$BREW/Cellar/python@3.12"; echo "==> Autoremoving 2 unneeded formulae" ;;
  "cleanup --prune=all -n") [ -d "$H/Library/Caches/Homebrew/downloads" ] && printf 'Would remove: %s (4 files, 280MB)\n==> This operation would free approximately 280.0MB of disk space.\n' "$H/Library/Caches/Homebrew/downloads" ;;
  "cleanup --prune=all") rm -rf "$H/Library/Caches/Homebrew/downloads" ;;
esac
EOF
cat > "$DEMO/bin/xcrun" <<EOF
#!/bin/sh
D="$DEV/CoreSimulator/Devices"
dev() { [ -d "\$D/\$1" ] && printf '{"udid":"%s","name":"%s","state":"Shutdown","isAvailable":%s,"dataPath":"%s/data","dataPathSize":%s,"lastUsedAt":"%s"}' "\$1" "\$2" "\$3" "\$D/\$1" "\$4" "\$5"; }
case "\$*" in
  "--find simctl") echo /usr/bin/simctl ;;
  "simctl list devices -j")
    old=\$(dev $SIM_OLD "iPhone 12 (old)" false 230686720 "$(iso_ago 300)")
    new=\$(dev $SIM_NEW "iPhone 17" true 73400320 "$(iso_ago 2)")
    printf '{"devices":{"com.apple.CoreSimulator.SimRuntime.iOS-18-0":[%s],"com.apple.CoreSimulator.SimRuntime.iOS-26-4":[%s]}}\n' "\$old" "\$new" ;;
  "simctl runtime list -j")
    printf '{"A1B2C3D4-0000-4000-8000-000000000264":{"identifier":"A1B2C3D4-0000-4000-8000-000000000264","runtimeIdentifier":"com.apple.CoreSimulator.SimRuntime.iOS-26-4","platformIdentifier":"com.apple.platform.iphonesimulator","version":"26.4","build":"23E244","deletable":true,"sizeBytes":1213000000,"state":"Ready","lastUsedAt":"%s"}}\n' "$(iso_ago 2)" ;;
  "simctl delete unavailable") rm -rf "\$D/$SIM_OLD" ;;
  simctl\ delete\ *) rm -rf "\$D/\$3" ;;
esac
EOF
cat > "$DEMO/bin/pod" <<EOF
#!/bin/sh
[ "\$*" = "cache clean --all" ] && rm -rf "$H/Library/Caches/CocoaPods"
EOF
cat > "$DEMO/bin/uv" <<EOF
#!/bin/sh
case "\$*" in "cache dir") echo "$H/.cache/uv" ;; "cache prune") rm -rf "$H/.cache/uv/wheels-v5" ;; esac
EOF
cat > "$DEMO/bin/pip3" <<EOF
#!/bin/sh
case "\$*" in "cache dir") echo "$H/Library/Caches/pip" ;; "cache purge") rm -rf "$H/Library/Caches/pip/http-v2" ;; esac
EOF
chmod +x "$DEMO/bin/"*

# Self-check: every path decruft reports must be inside the fake HOME.
cd "$H"
if env -i HOME="$H" PATH="$DEMO/bin:/usr/bin:/bin:/usr/sbin" GOTOOLCHAIN=local "$DEMO/bin/decruft" --json |
  grep -o '"/[^"]*"' | grep -v -e "\"$DEMO" -e '"/usr/' -e '"/bin/' -e '"/System/' >&2; then
  echo "setup.sh: decruft reported a path outside the demo" >&2
  return 1
fi
set +e
# A clean shell: nothing from the real environment.
exec env -i HOME="$H" PATH="$DEMO/bin:/usr/bin:/bin:/usr/sbin" TERM="${TERM:-xterm-256color}" LANG=en_US.UTF-8 \
  GOTOOLCHAIN=local PS1='$ ' bash --norc --noprofile
