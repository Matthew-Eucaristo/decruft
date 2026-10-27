package module

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Matthew-Eucaristo/decruft/internal/item"
)

// Android & Gradle: SDK packages through `sdkmanager --uninstall` and
// emulators through `avdmanager delete avd`, both from the SDK's own
// command-line tools. Gradle has no prune command (it expires unused cache
// entries itself), so its folders are shown, not removed.
func Android() Module {
	return Module{ID: "android", Name: "Android & Gradle", Scan: scanAndroid}
}

func androidSDK(env *Env) string {
	for _, v := range []string{"ANDROID_HOME", "ANDROID_SDK_ROOT"} {
		if d := env.Getenv(v); d != "" && isDir(d) {
			return d
		}
	}
	var d string
	switch env.OS {
	case "darwin":
		d = filepath.Join(env.Home, "Library", "Android", "sdk")
	case "windows":
		d = filepath.Join(env.Getenv("LOCALAPPDATA"), "Android", "Sdk")
	default:
		d = filepath.Join(env.Home, "Android", "Sdk")
	}
	if isDir(d) {
		return d
	}
	return ""
}

// sdkTool finds sdkmanager or avdmanager in the SDK's command-line tools,
// then on PATH.
func sdkTool(env *Env, sdk, name string) string {
	if runtime.GOOS == "windows" {
		name += ".bat"
	}
	for _, dir := range []string{"latest", ""} {
		p := filepath.Join(sdk, "cmdline-tools", dir, "bin", name)
		if dir == "" {
			ms, _ := filepath.Glob(filepath.Join(sdk, "cmdline-tools", "*", "bin", name))
			if len(ms) == 0 {
				continue
			}
			p = ms[len(ms)-1]
		}
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}
	if p, err := env.LookPath(name); err == nil {
		return p
	}
	return ""
}

func scanAndroid(_ context.Context, env *Env) Result {
	var r Result
	studio := env.Running("studio") || env.Running("studio64")
	emulator := env.Running("qemu-system-aarch64") || env.Running("qemu-system-x86_64")
	if sdk := androidSDK(env); sdk != "" {
		sdkmanager := sdkTool(env, sdk, "sdkmanager")
		pkg := func(dir, id, name, explain string) {
			s := spec{source: "android", category: item.Android, kind: "android-sdk-package", name: name, path: dir,
				level: item.Review, explain: explain, regen: "Android Studio › SDK Manager, or sdkmanager \"" + id + "\" (network)"}
			if sdkmanager != "" {
				s.argv = []string{sdkmanager, "--uninstall", id}
			}
			it, ok := build(env, s)
			if !ok {
				return
			}
			if sdkmanager == "" {
				it.Action = item.Action{Kind: item.Manual, How: "Android Studio › Settings › Languages & Frameworks › Android SDK (or install the SDK command-line tools for sdkmanager)"}
				it.Add("no_sdkmanager", item.Protected, "the SDK's command-line tools are not installed, so there is no sdkmanager to remove it with")
			}
			it.Add("pinned", item.Review, "a project may pin this version (ndkVersion, compileSdk); Gradle downloads it again if so")
			if studio {
				it.Add("studio_running", item.Live, "Android Studio is running; quit it before changing the SDK")
			}
			r.Items = append(r.Items, it)
		}
		for _, d := range subdirs(filepath.Join(sdk, "ndk")) {
			v := filepath.Base(d)
			pkg(d, "ndk;"+v, "NDK "+v, "Native development kit for C/C++ code in Android apps.")
		}
		for _, api := range subdirs(filepath.Join(sdk, "system-images")) {
			for _, tag := range subdirs(api) {
				for _, abi := range subdirs(tag) {
					id := strings.Join([]string{"system-images", filepath.Base(api), filepath.Base(tag), filepath.Base(abi)}, ";")
					pkg(abi, id, "System image "+filepath.Base(api)+" "+filepath.Base(tag)+" "+filepath.Base(abi),
						"The Android OS image emulators boot from.")
				}
			}
		}
		avdmanager := sdkTool(env, sdk, "avdmanager")
		avdHome := env.Getenv("ANDROID_AVD_HOME")
		if avdHome == "" {
			avdHome = filepath.Join(env.Home, ".android", "avd")
		}
		for _, d := range subdirs(avdHome) {
			name, ok := strings.CutSuffix(filepath.Base(d), ".avd")
			if !ok {
				continue
			}
			s := spec{source: "android", category: item.Android, kind: "android-avd", name: "Emulator · " + name, path: d,
				level: item.Review, explain: "An Android emulator: its disk, installed apps and data.",
				regen: "Android Studio › Device Manager › Create device"}
			if avdmanager != "" {
				s.argv = []string{avdmanager, "delete", "avd", "-n", name}
			}
			it, ok := build(env, s)
			if !ok {
				continue
			}
			if avdmanager == "" {
				it.Action = item.Action{Kind: item.Manual, How: "Android Studio › Device Manager › ⋮ › Delete"}
				it.Add("no_avdmanager", item.Protected, "the SDK's command-line tools are not installed, so there is no avdmanager to remove it with")
			}
			it.Add("app_data", item.Review, "deletes the apps and data inside this emulator")
			if emulator {
				it.Add("emulator_running", item.Live, "an emulator is running")
			}
			r.Items = append(r.Items, it)
		}
	}

	gradle := env.Getenv("GRADLE_USER_HOME")
	if gradle == "" {
		gradle = filepath.Join(env.Home, ".gradle")
	}
	for _, g := range []struct{ sub, name, explain string }{
		{"caches", "Gradle caches", "Downloaded dependencies and build outputs Gradle reuses."},
		{filepath.Join("wrapper", "dists"), "Gradle wrapper distributions", "Gradle versions downloaded by projects' gradlew."},
	} {
		it, ok := build(env, spec{source: "android", category: item.Android, kind: "gradle-home", name: g.name,
			path: filepath.Join(gradle, g.sub), level: item.Protected, explain: g.explain})
		if !ok {
			continue
		}
		it.Action = item.Action{Kind: item.None, How: "Gradle deletes entries unused for 30 days by itself (configurable in an init script); there is no official prune command"}
		it.Add("no_command", item.Protected, "no official cleanup command; Gradle expires unused entries itself")
		r.Items = append(r.Items, it)
	}
	return r
}
