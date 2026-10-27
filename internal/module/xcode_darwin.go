package module

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Matthew-Eucaristo/decruft/internal/item"
)

// Xcode: DerivedData and DeviceSupport folders (Xcode recreates both; Apple's
// Storage settings remove them the same way, as folders), Archives (shown,
// removed only in Xcode's Organizer), and simulators through `xcrun simctl`.
func Xcode() Module {
	return Module{ID: "xcode", Name: "Xcode & Simulators", Scan: scanXcode}
}

func scanXcode(ctx context.Context, env *Env) Result {
	var r Result
	dev := filepath.Join(env.Home, "Library", "Developer")
	xcodeRunning := env.Running("Xcode")
	live := func(it *item.Item) {
		if xcodeRunning {
			it.Add("xcode_running", item.Live, "Xcode is running; quit it first so it does not write here mid-move")
		}
	}

	derived := filepath.Join(dev, "Xcode", "DerivedData")
	for _, d := range subdirs(derived) {
		base := filepath.Base(d)
		name := base
		if i := strings.LastIndexByte(base, '-'); i > 0 && len(base)-i > 20 {
			name = base[:i] // Project-abcdefghijklmnopqrstuvwxyz
		}
		kind := "xcode-derived-data"
		explain := "Build products, indexes and logs for one project. Xcode rebuilds them."
		if base == "ModuleCache.noindex" {
			name, explain = "Module cache", "Precompiled Clang and Swift modules shared by every project."
		}
		it, ok := build(env, spec{
			source: "xcode", category: item.Xcode, kind: kind, name: "DerivedData · " + name, path: d,
			explain: explain, regen: "the next build in Xcode (slower once, re-indexes)", regrows: base == "ModuleCache.noindex",
		})
		if !ok {
			continue
		}
		if fi, err := os.Stat(d); err == nil {
			it.Note = "last written " + fi.ModTime().Format("2006-01-02")
		}
		live(&it)
		r.Items = append(r.Items, it)
	}

	for _, plat := range []string{"iOS", "watchOS", "tvOS", "visionOS", "macOS"} {
		dir := filepath.Join(dev, "Xcode", plat+" DeviceSupport")
		for _, d := range subdirs(dir) {
			it, ok := build(env, spec{
				source: "xcode", category: item.Xcode, kind: "xcode-device-support",
				name: plat + " DeviceSupport · " + filepath.Base(d), path: d, level: item.Review,
				explain: "Debug symbols Xcode copied from a device running this OS version.",
				regen:   "Xcode copies them again the next time such a device connects (takes minutes)",
			})
			if !ok {
				continue
			}
			it.Add("device_symbols", item.Review, "keep the versions your devices still run")
			live(&it)
			r.Items = append(r.Items, it)
		}
	}

	archives := filepath.Join(dev, "Xcode", "Archives")
	if isDir(archives) {
		if it, ok := build(env, spec{
			source: "xcode", category: item.Xcode, kind: "xcode-archives", name: "Archives", path: archives,
			level:   item.Protected,
			explain: "Signed builds you archived, with the dSYMs needed to read their crash reports.",
		}); ok {
			it.Action = item.Action{Kind: item.Manual, How: "Xcode › Window › Organizer › Archives: delete the ones you will never re-submit or symbolicate"}
			it.Add("archives", item.Protected, "holds dSYMs for crash symbolication; remove them in Xcode's Organizer")
			r.Items = append(r.Items, it)
		}
	}

	xcrun, err := env.LookPath("xcrun")
	if err != nil {
		return r
	}
	if _, err := env.Output(ctx, nil, xcrun, "--find", "simctl"); err != nil {
		return r // Command Line Tools only: no simulators
	}
	sims, warn := scanSimulators(ctx, env, xcrun)
	r.Items = append(r.Items, sims...)
	r.Warnings = append(r.Warnings, warn...)
	return r
}

type simDevice struct {
	UDID         string `json:"udid"`
	Name         string `json:"name"`
	State        string `json:"state"`
	IsAvailable  bool   `json:"isAvailable"`
	DataPath     string `json:"dataPath"`
	DataPathSize int64  `json:"dataPathSize"`
	LastUsedAt   string `json:"lastUsedAt"`
	Availability string `json:"availabilityError"`
}

type simRuntime struct {
	Identifier        string `json:"identifier"`
	RuntimeIdentifier string `json:"runtimeIdentifier"`
	Platform          string `json:"platformIdentifier"`
	Version           string `json:"version"`
	Build             string `json:"build"`
	Deletable         bool   `json:"deletable"`
	SizeBytes         int64  `json:"sizeBytes"`
	State             string `json:"state"`
	LastUsedAt        string `json:"lastUsedAt"`
	Kind              string `json:"kind"`
}

func scanSimulators(ctx context.Context, env *Env, xcrun string) ([]item.Item, []string) {
	var items []item.Item
	var warns []string
	out, err := env.Output(ctx, nil, xcrun, "simctl", "list", "devices", "-j")
	if err != nil {
		return nil, []string{"simctl list devices: " + err.Error()}
	}
	var dl struct {
		Devices map[string][]simDevice `json:"devices"`
	}
	if err := json.Unmarshal(out, &dl); err != nil || dl.Devices == nil {
		return nil, []string{"simctl list devices: unrecognised output; simulators skipped"}
	}
	booted := map[string]bool{} // runtime identifiers with a booted device
	var unavailable []simDevice
	runtimesOf := map[string]int{}
	keys := make([]string, 0, len(dl.Devices))
	for k := range dl.Devices {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, rt := range keys {
		for _, d := range dl.Devices[rt] {
			runtimesOf[rt]++
			if d.State == "Booted" {
				booted[rt] = true
			}
			if !d.IsAvailable {
				unavailable = append(unavailable, d)
				continue
			}
			it := simItem(env, xcrun, rt, d)
			items = append(items, it)
		}
	}
	if len(unavailable) > 0 {
		var bytes int64
		for _, d := range unavailable {
			bytes += max(d.DataPathSize, 0)
		}
		it, _ := build(env, spec{
			source: "xcode", category: item.Xcode, kind: "simctl-unavailable",
			name:    fmt.Sprintf("Unavailable simulators (%d)", len(unavailable)),
			argv:    []string{xcrun, "simctl", "delete", "unavailable"},
			explain: "Simulator devices whose runtime is no longer installed. They cannot boot.",
		}, fmt.Sprint(len(unavailable)))
		it.Bytes, it.Frees = bytes, bytes
		items = append(items, it)
	}

	out, err = env.Output(ctx, nil, xcrun, "simctl", "runtime", "list", "-j")
	if err != nil {
		return items, append(warns, "simctl runtime list: "+err.Error())
	}
	var rts map[string]simRuntime
	if err := json.Unmarshal(out, &rts); err != nil {
		return items, append(warns, "simctl runtime list: unrecognised output; runtimes skipped")
	}
	perPlatform := map[string]int{}
	for _, rt := range rts {
		perPlatform[rt.Platform]++
	}
	ids := make([]string, 0, len(rts))
	for id := range rts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		rt := rts[id]
		plat := platformName(rt.Platform)
		it, _ := build(env, spec{
			source: "xcode", category: item.Xcode, kind: "simctl-runtime",
			name:    fmt.Sprintf("%s %s simulator runtime", plat, rt.Version),
			argv:    []string{xcrun, "simctl", "runtime", "delete", rt.Identifier},
			level:   item.Review,
			explain: fmt.Sprintf("The %s %s (%s) system image simulators boot from. macOS counts it as System Data.", plat, rt.Version, rt.Build),
			regen:   "Xcode › Settings › Components, or xcodebuild -downloadPlatform " + plat,
		}, rt.State, fmt.Sprint(rt.Deletable))
		it.Bytes, it.Frees, it.SystemData = rt.SizeBytes, rt.SizeBytes, true
		if t, err := time.Parse(time.RFC3339, rt.LastUsedAt); err == nil {
			it.Note = "last used " + t.Format("2006-01-02")
		}
		if n := runtimesOf[rt.RuntimeIdentifier]; n > 0 {
			it.Note = joinNote(it.Note, fmt.Sprintf("%d simulator device(s) use it and become unavailable", n))
		}
		if perPlatform[rt.Platform] == 1 {
			it.Add("only_runtime", item.Review, "the only "+plat+" runtime: you will need to download it again (several GB) to run "+plat+" simulators")
		} else {
			it.Add("runtime", item.Review, "large download to get back")
		}
		switch {
		case !rt.Deletable:
			it.Add("not_deletable", item.Protected, "simctl says this runtime cannot be deleted (it came with Xcode or the system)")
			it.Action = item.Action{Kind: item.Manual, How: "Xcode › Settings › Components"}
		case booted[rt.RuntimeIdentifier]:
			it.Add("booted", item.Live, "a simulator using this runtime is booted")
		}
		it.Evidence = append(it.Evidence, fmt.Sprint(booted[rt.RuntimeIdentifier]))
		items = append(items, it)
	}
	return items, warns
}

func simItem(env *Env, xcrun, runtimeID string, d simDevice) item.Item {
	dir := filepath.Dir(d.DataPath) // Devices/<udid>
	it, _ := build(env, spec{
		source: "xcode", category: item.Xcode, kind: "simctl-device",
		name:    fmt.Sprintf("Simulator · %s (%s)", d.Name, runtimeLabel(runtimeID)),
		argv:    []string{xcrun, "simctl", "delete", d.UDID},
		level:   item.Review,
		explain: "A simulator device: its installed apps, their data and settings.",
		regen:   "Xcode › Window › Devices and Simulators › +, then reinstall your app",
	}, d.State)
	it.Path = dir
	it.Bytes, it.Frees = max(d.DataPathSize, 0), -1
	if m := env.Measure(dir); !m.Missing {
		it.Bytes, it.Frees = m.Bytes, m.Frees
	}
	if t, err := time.Parse(time.RFC3339, d.LastUsedAt); err == nil {
		it.Note = "last used " + t.Format("2006-01-02")
	}
	it.Add("app_data", item.Review, "deletes the apps and data inside this simulator")
	if d.State != "Shutdown" {
		it.Add("booted", item.Live, "this simulator is "+strings.ToLower(d.State))
	}
	it.Evidence = append(it.Evidence, identity(dir))
	return it
}

func platformName(id string) string {
	switch {
	case strings.Contains(id, "iphone"):
		return "iOS"
	case strings.Contains(id, "watch"):
		return "watchOS"
	case strings.Contains(id, "appletv"):
		return "tvOS"
	case strings.Contains(id, "xr"):
		return "visionOS"
	}
	return id
}

// runtimeLabel turns com.apple.CoreSimulator.SimRuntime.iOS-27-0 into iOS 27.0.
func runtimeLabel(id string) string {
	s := id[strings.LastIndexByte(id, '.')+1:]
	if i := strings.IndexByte(s, '-'); i > 0 {
		return s[:i] + " " + strings.ReplaceAll(s[i+1:], "-", ".")
	}
	return s
}
