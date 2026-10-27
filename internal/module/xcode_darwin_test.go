package module

import (
	"context"
	"strings"
	"testing"

	"github.com/Matthew-Eucaristo/decruft/internal/item"
)

const devices = `{"devices":{
 "com.apple.CoreSimulator.SimRuntime.iOS-18-0":[{"udid":"OLD","name":"Old","state":"Shutdown","isAvailable":false,"dataPath":"/x/OLD/data","dataPathSize":100}],
 "com.apple.CoreSimulator.SimRuntime.iOS-27-0":[{"udid":"A","name":"iPhone","state":"Shutdown","isAvailable":true,"dataPath":"/x/A/data","dataPathSize":200},
                                                {"udid":"B","name":"iPad","state":"Booted","isAvailable":true,"dataPath":"/x/B/data","dataPathSize":300}]}}`

const runtimes = `{"R1":{"identifier":"R1","runtimeIdentifier":"com.apple.CoreSimulator.SimRuntime.iOS-27-0","platformIdentifier":"com.apple.platform.iphonesimulator","version":"27.0","build":"24A","deletable":true,"sizeBytes":8000000000,"state":"Ready"},
 "R2":{"identifier":"R2","runtimeIdentifier":"com.apple.CoreSimulator.SimRuntime.watchOS-11-0","platformIdentifier":"com.apple.platform.watchsimulator","version":"11.0","build":"22R","deletable":false,"sizeBytes":3000000000,"state":"Ready"}}`

func TestSimulators(t *testing.T) {
	env := fakeEnv(t, map[string]string{
		"xcrun --find simctl":          "/usr/bin/simctl",
		"xcrun simctl list devices -j": devices,
		"xcrun simctl runtime list -j": runtimes,
	})
	r := scanXcode(context.Background(), env)
	byName := map[string]item.Item{}
	for _, it := range r.Items {
		byName[it.Name] = it
	}
	if u := byName["Unavailable simulators (1)"]; u.Level != item.Safe || strings.Join(u.Action.Argv, " ") != "/bin/xcrun simctl delete unavailable" || u.Bytes != 100 {
		t.Fatalf("%+v", u)
	}
	if d := byName["Simulator · iPhone (iOS 27.0)"]; d.Level != item.Review || d.Action.Argv[3] != "A" {
		t.Fatalf("%+v", d)
	}
	if d := byName["Simulator · iPad (iOS 27.0)"]; d.Level != item.Live {
		t.Fatalf("a booted simulator is live: %+v", d)
	}
	rt := byName["iOS 27.0 simulator runtime"]
	if rt.Level != item.Live || !rt.SystemData || !rt.Has("only_runtime") {
		t.Fatalf("runtime with a booted device is live and the only iOS runtime: %+v", rt)
	}
	if w := byName["watchOS 11.0 simulator runtime"]; w.Selectable() || w.Action.Kind != item.Manual {
		t.Fatalf("not deletable: %+v", w)
	}
}

func TestDerivedDataIsLiveWhileXcodeRuns(t *testing.T) {
	env := fakeEnv(t, nil)
	mkdir(t, env.Home, "Library", "Developer", "Xcode", "DerivedData", "Shop-abcdefghijklmnopqrstuvwxyz")
	r := scanXcode(context.Background(), env)
	if len(r.Items) != 1 || r.Items[0].Name != "DerivedData · Shop" || r.Items[0].Level != item.Safe || r.Items[0].Action.Kind != item.Trash {
		t.Fatalf("%+v", r.Items)
	}
	env.Running = func(n string) bool { return n == "Xcode" }
	if it := scanXcode(context.Background(), env).Items[0]; it.Level != item.Live {
		t.Fatalf("%+v", it)
	}
}
