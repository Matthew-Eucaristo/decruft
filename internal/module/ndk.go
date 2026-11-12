package module

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// ndkUsers maps an NDK version to the projects under the scan roots whose
// Android build names it. Three sources, all read-only:
//   - a literal ndkVersion in android/app/build.gradle(.kts), android/build.gradle(.kts)
//     or a *ndkVersion key in android/gradle.properties;
//   - `flutter.ndkVersion`, resolved to the default in the installed Flutter SDK;
//   - React Native's default, from node_modules/react-native/gradle/libs.versions.toml,
//     for a React Native project that does not set its own.
//
// A version with no users here may still be the Android Gradle plugin's own
// default, so callers must not treat "no users" as unused.
func ndkUsers(env *Env) map[string][]string {
	users := map[string][]string{}
	flutterDefault := flutterNDK(env)
	add := func(v, name string) {
		if v != "" && !slices.Contains(users[v], name) {
			users[v] = append(users[v], name)
		}
	}
	for _, root := range projectRoots(env) {
		walkProjects(root, 6, func(dir string) {
			if filepath.Base(dir) == "android" {
				return // read from the project that holds it
			}
			android := dir
			if isDir(filepath.Join(dir, "android")) {
				android = filepath.Join(dir, "android")
			}
			if !isFile(filepath.Join(android, "settings.gradle")) && !isFile(filepath.Join(android, "settings.gradle.kts")) {
				return
			}
			name := filepath.Base(dir)
			found := false
			for _, f := range []string{
				filepath.Join(android, "app", "build.gradle"), filepath.Join(android, "app", "build.gradle.kts"),
				filepath.Join(android, "build.gradle"), filepath.Join(android, "build.gradle.kts"),
			} {
				for _, v := range gradleNDK(f, flutterDefault) {
					add(v, name)
					found = true
				}
			}
			if v := propertiesNDK(filepath.Join(android, "gradle.properties")); v != "" {
				add(v, name)
				found = true
			}
			if !found {
				add(tomlNDK(filepath.Join(dir, "node_modules", "react-native", "gradle", "libs.versions.toml")), name)
			}
		})
	}
	for v := range users {
		slices.Sort(users[v])
	}
	return users
}

// projectRoots are the roots passed on the command line, or ~/dev, the
// family tools' default.
func projectRoots(env *Env) []string {
	if len(env.Roots) > 0 {
		return env.Roots
	}
	if d := filepath.Join(env.Home, "dev"); isDir(d) {
		return []string{d}
	}
	return nil
}

// walkProjects calls visit for every directory up to depth levels below
// root, skipping hidden directories, dependency installs and build output.
// Symlinks are not followed.
func walkProjects(root string, depth int, visit func(dir string)) {
	base := strings.Count(filepath.Clean(root), string(filepath.Separator))
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			if err != nil && d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if path != root {
			switch n := d.Name(); {
			case strings.HasPrefix(n, "."), n == "node_modules", n == "Pods", n == "build", n == "vendor", n == "target":
				return fs.SkipDir
			}
		}
		visit(path)
		if strings.Count(path, string(filepath.Separator))-base >= depth {
			return fs.SkipDir
		}
		return nil
	})
}

var (
	gradleNDKLiteral = regexp.MustCompile(`\bndkVersion\s*(?:=\s*)?["']([0-9]+(?:\.[0-9]+)+)["']`)
	gradleNDKFlutter = regexp.MustCompile(`\bndkVersion\s*(?:=\s*)?flutter\.ndkVersion\b`)
	propertiesNDKKey = regexp.MustCompile(`^\s*[A-Za-z0-9_.]*ndkVersion\s*[=:]\s*([0-9]+(?:\.[0-9]+)+)\s*$`)
	tomlNDKKey       = regexp.MustCompile(`^\s*ndkVersion\s*=\s*"([0-9]+(?:\.[0-9]+)+)"`)
	flutterNDKKey    = regexp.MustCompile(`val\s+ndkVersion\s*:\s*String\s*=\s*"([0-9]+(?:\.[0-9]+)+)"`)
)

// gradleNDK returns the NDK versions a Gradle build script names.
func gradleNDK(path, flutterDefault string) []string {
	b, err := readSmall(path)
	if err != nil {
		return nil
	}
	var out []string
	for _, m := range gradleNDKLiteral.FindAllStringSubmatch(b, -1) {
		out = append(out, m[1])
	}
	if flutterDefault != "" && gradleNDKFlutter.MatchString(b) {
		out = append(out, flutterDefault)
	}
	return out
}

func propertiesNDK(path string) string { return firstMatch(path, propertiesNDKKey) }
func tomlNDK(path string) string       { return firstMatch(path, tomlNDKKey) }

// flutterNDK is the default NDK of the Flutter SDK on PATH, or "".
func flutterNDK(env *Env) string {
	bin, err := env.LookPath("flutter")
	if err != nil {
		return ""
	}
	if r, err := filepath.EvalSymlinks(bin); err == nil {
		bin = r
	}
	sdk := filepath.Dir(filepath.Dir(bin))
	b, err := readSmall(filepath.Join(sdk, "packages", "flutter_tools", "gradle", "src", "main", "kotlin", "FlutterExtension.kt"))
	if err != nil {
		return ""
	}
	if m := flutterNDKKey.FindStringSubmatch(b); m != nil {
		return m[1]
	}
	return ""
}

func firstMatch(path string, re *regexp.Regexp) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		if m := re.FindStringSubmatch(s.Text()); m != nil {
			return m[1]
		}
	}
	return ""
}

// readSmall reads a regular file of at most 1 MiB; build scripts are small,
// and anything else is not worth reading.
func readSmall(path string) (string, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !fi.Mode().IsRegular() || fi.Size() > 1<<20 {
		return "", fs.ErrInvalid
	}
	b, err := os.ReadFile(path)
	return string(b), err
}

func isFile(path string) bool {
	fi, err := os.Lstat(path)
	return err == nil && fi.Mode().IsRegular()
}
