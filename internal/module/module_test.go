package module

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Matthew-Eucaristo/decruft/internal/item"
)

// fakeEnv answers commands from a table and finds tools named in it.
func fakeEnv(t *testing.T, answers map[string]string) *Env {
	home := t.TempDir()
	return &Env{
		Home: home, OS: "darwin", Getenv: func(string) string { return "" },
		LookPath: func(name string) (string, error) {
			for k := range answers {
				if strings.HasPrefix(k, name+" ") || k == name {
					return "/bin/" + name, nil
				}
			}
			return "", errors.New("not found")
		},
		Output: func(_ context.Context, _ []string, argv ...string) ([]byte, error) {
			key := strings.TrimPrefix(strings.Join(argv, " "), "/bin/")
			if out, ok := answers[key]; ok {
				return []byte(out), nil
			}
			return nil, errors.New("no answer for " + key)
		},
		Running: func(string) bool { return false },
		Measure: func(p string) Measured {
			if _, err := os.Lstat(p); err != nil {
				return Measured{Missing: true}
			}
			return Measured{Bytes: 4096, Frees: 4096}
		},
	}
}

func mkdir(t *testing.T, parts ...string) string {
	p := filepath.Join(parts...)
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParseSize(t *testing.T) {
	cases := map[string]int64{"373.3MB": 373_300_000, "1.2GB (50%)": 1_200_000_000, "0B": 0, "4.0 KiB": 4096, "1,024kB": 1_024_000, "12G": 12e9, "nope": -1, "5 parsecs": -1}
	for in, want := range cases {
		if got := parseSize(in); got != want {
			t.Errorf("parseSize(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestGoUsesGoEnvAndGoClean(t *testing.T) {
	env := fakeEnv(t, nil)
	cache, mod := mkdir(t, env.Home, "gocache"), mkdir(t, env.Home, "go", "pkg", "mod")
	env.LookPath = func(n string) (string, error) {
		if n == "go" {
			return "/bin/go", nil
		}
		return "", errors.New("no")
	}
	env.Output = func(_ context.Context, _ []string, argv ...string) ([]byte, error) {
		return json.Marshal(map[string]string{"GOCACHE": cache, "GOMODCACHE": mod})
	}
	r := scanGo(context.Background(), env)
	if len(r.Items) != 2 {
		t.Fatalf("%+v", r)
	}
	if got := strings.Join(r.Items[0].Action.Argv, " "); got != "/bin/go clean -cache" || !r.Items[0].Regrows {
		t.Fatalf("%s", got)
	}
	if got := strings.Join(r.Items[1].Action.Argv, " "); got != "/bin/go clean -modcache" {
		t.Fatalf("%s", got)
	}
}

func TestGoNotInstalledOffersNothing(t *testing.T) {
	if r := scanGo(context.Background(), fakeEnv(t, nil)); len(r.Items)+len(r.Warnings) != 0 {
		t.Fatalf("%+v", r)
	}
}

func TestHomebrew(t *testing.T) {
	env := fakeEnv(t, map[string]string{
		"brew autoremove -n":          "==> Would autoremove 2 unneeded formulae:\nicu4c@76\npython@3.12\n",
		"brew cleanup --prune=all -n": "Would remove: /x/a (1 file, 2MB)\nWould remove: /x/b (3 files, 371MB)\n==> This operation would free approximately 373.3MB of disk space.\n",
	})
	cellar := mkdir(t, env.Home, "Cellar")
	mkdir(t, cellar, "icu4c@76")
	mkdir(t, cellar, "python@3.12")
	inner := env.Output
	env.Output = func(ctx context.Context, e []string, argv ...string) ([]byte, error) {
		if strings.HasSuffix(strings.Join(argv, " "), "--cellar") {
			return []byte(cellar + "\n"), nil
		}
		return inner(ctx, e, argv...)
	}
	r := scanHomebrew(context.Background(), env)
	if len(r.Items) != 2 {
		t.Fatalf("%+v", r)
	}
	auto, clean := r.Items[0], r.Items[1]
	if auto.Level != item.Review || auto.Bytes != 8192 || auto.Priority >= clean.Priority || !strings.Contains(auto.Explain, "python@3.12") {
		t.Fatalf("autoremove %+v", auto)
	}
	if clean.Bytes != 373_300_000 || clean.Level != item.Safe || strings.Join(clean.Action.Argv, " ") != "/bin/brew cleanup --prune=all" {
		t.Fatalf("cleanup %+v", clean)
	}
}

func TestHomebrewNothingToDo(t *testing.T) {
	env := fakeEnv(t, map[string]string{"brew autoremove -n": "", "brew cleanup --prune=all -n": ""})
	if r := scanHomebrew(context.Background(), env); len(r.Items) != 0 {
		t.Fatalf("%+v", r.Items)
	}
}

func TestDocker(t *testing.T) {
	df := `{"Active":"1","Reclaimable":"1.2GB (50%)","Size":"2.4GB","TotalCount":"5","Type":"Images"}
{"Active":"0","Reclaimable":"100MB (100%)","Size":"100MB","TotalCount":"2","Type":"Containers"}
{"Active":"1","Reclaimable":"5GB (100%)","Size":"5GB","TotalCount":"3","Type":"Local Volumes"}
{"Active":"0","Reclaimable":"300MB","Size":"300MB","TotalCount":"9","Type":"Build Cache"}`
	r := scanDocker(context.Background(), fakeEnv(t, map[string]string{"docker system df --format {{json .}}": df}))
	if len(r.Items) != 1 {
		t.Fatalf("%+v", r)
	}
	it := r.Items[0]
	// Volumes are never pruned, so their reclaimable bytes are not promised.
	if it.Frees != 1_600_000_000 || !it.UpTo || it.Level != item.Review || !strings.Contains(strings.Join(it.Action.Argv, " "), "system prune --force") {
		t.Fatalf("%+v", it)
	}
}

func TestDockerNotRunning(t *testing.T) {
	env := fakeEnv(t, map[string]string{"docker": ""})
	r := scanDocker(context.Background(), env)
	if len(r.Items) != 0 || len(r.Warnings) != 1 {
		t.Fatalf("%+v", r)
	}
}

func TestPackagesPipDisabledIsQuiet(t *testing.T) {
	env := fakeEnv(t, map[string]string{"pip3": ""})
	env.Output = func(context.Context, []string, ...string) ([]byte, error) {
		return nil, errors.New("pip3: ERROR: pip cache commands can not function since cache is disabled.")
	}
	if r := scanPackages(context.Background(), env); len(r.Items)+len(r.Warnings) != 0 {
		t.Fatalf("%+v", r)
	}
}

func TestAndroid(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fixture's SDK tools are shell scripts")
	}
	env := fakeEnv(t, nil)
	sdk := filepath.Join(env.Home, "Library", "Android", "sdk")
	mkdir(t, sdk, "ndk", "27.0.1")
	mkdir(t, sdk, "system-images", "android-35", "google_apis", "arm64-v8a")
	mkdir(t, env.Home, ".android", "avd", "Pixel_9.avd")
	mkdir(t, env.Home, ".gradle", "caches")
	r := scanAndroid(context.Background(), env)
	kinds := map[string]item.Item{}
	for _, it := range r.Items {
		kinds[it.Name] = it
	}
	// Without the SDK's command-line tools nothing is removable.
	for _, name := range []string{"NDK 27.0.1", "System image android-35 google_apis arm64-v8a", "Emulator · Pixel_9"} {
		it, ok := kinds[name]
		if !ok || it.Selectable() || it.Action.Kind != item.Manual {
			t.Fatalf("%s: %+v", name, it)
		}
	}
	if g := kinds["Gradle caches"]; g.Selectable() || g.Action.Kind != item.None {
		t.Fatalf("Gradle has no official prune: %+v", g)
	}
	// With sdkmanager installed, packages go through it.
	tool := mkdir(t, sdk, "cmdline-tools", "latest", "bin")
	for _, n := range []string{"sdkmanager", "avdmanager"} {
		if err := os.WriteFile(filepath.Join(tool, n), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	r = scanAndroid(context.Background(), env)
	for _, it := range r.Items {
		if it.Name == "NDK 27.0.1" && (!it.Selectable() || it.Action.Argv[1] != "--uninstall" || it.Action.Argv[2] != "ndk;27.0.1") {
			t.Fatalf("%+v", it)
		}
		if it.Name == "Emulator · Pixel_9" && strings.Join(it.Action.Argv[1:], " ") != "delete avd -n Pixel_9" {
			t.Fatalf("%+v", it)
		}
	}
	// Android Studio running makes SDK changes live.
	env.Running = func(n string) bool { return n == "studio" }
	for _, it := range scanAndroid(context.Background(), env).Items {
		if strings.HasPrefix(it.Name, "NDK") && it.Level != item.Live {
			t.Fatalf("%+v", it)
		}
	}
}

func TestLinuxNeverRunsSudo(t *testing.T) {
	env := fakeEnv(t, map[string]string{
		"apt-get -s autoremove":   "Remv libfoo1 [1.0]\nRemv libbar2 [2.0]\n",
		"journalctl --disk-usage": "Archived and active journals take up 1.2G in the file system.\n",
	})
	r := scanLinux(context.Background(), env)
	if len(r.Items) == 0 {
		t.Fatal("no items")
	}
	for _, it := range r.Items {
		if it.Selectable() || it.Action.Kind != item.Manual || it.Action.Argv[0] != "sudo" {
			t.Fatalf("root commands must be shown only: %+v", it)
		}
	}
}

func TestAutoremoveNames(t *testing.T) {
	got := autoremoveNames("==> Would autoremove 2 unneeded formulae:\nicu4c@76\npython@3.12\n==> Other\nnot-me\n")
	if strings.Join(got, ",") != "icu4c@76,python@3.12" {
		t.Fatal(got)
	}
}
