package disk

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// volumeScript asks Foundation, through JavaScript for Automation, for the
// numbers Finder uses: "important usage" capacity includes purgeable space.
const volumeScript = `ObjC.import("Foundation");
function run(argv) {
  var keys = ["NSURLVolumeLocalizedNameKey", "NSURLVolumeAvailableCapacityKey", "NSURLVolumeAvailableCapacityForImportantUsageKey"];
  var r = $.NSURL.fileURLWithPath(argv[0]).resourceValuesForKeysError($(keys), null);
  return keys.map(function (k) { return String(ObjC.unwrap(r.objectForKey(k))); }).join("\n");
}`

func platform(ctx context.Context, home string, in *Info) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "/usr/bin/osascript", "-l", "JavaScript", "-e", volumeScript, home).Output(); err == nil {
		f := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(f) == 3 {
			if f[0] != "" && f[0] != "undefined" {
				in.Name = f[0]
			}
			avail, err1 := strconv.ParseInt(f[1], 10, 64)
			important, err2 := strconv.ParseInt(f[2], 10, 64)
			if err1 == nil && err2 == nil && important >= avail {
				in.Purgeable = important - avail
			}
		}
	}
	if out, err := exec.CommandContext(ctx, "/usr/bin/tmutil", "listlocalsnapshots", "/").Output(); err == nil {
		in.Snapshots = parseSnapshots(string(out))
	}
	in.Swap = swapSize("/private/var/vm")
}
