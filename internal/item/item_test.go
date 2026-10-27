package item

import "testing"

func TestParseLevelFailsClosed(t *testing.T) {
	for in, want := range map[string]Level{"safe": Safe, "review": Review, "caution": Review, "live": Live, "protected": Protected, "": Protected, "yolo": Protected} {
		if got := ParseLevel(in); got != want {
			t.Errorf("%q → %s, want %s", in, got, want)
		}
	}
}

func TestSelectable(t *testing.T) {
	cases := []struct {
		l    Level
		k    Kind
		want bool
	}{
		{Safe, Command, true}, {Review, Trash, true}, {Review, Delegate, true},
		{Protected, Command, false}, {Live, Trash, false},
		{Safe, Manual, false}, {Safe, Handoff, false}, {Safe, None, false},
	}
	for _, c := range cases {
		it := Item{Level: c.l, Action: Action{Kind: c.k}}
		if it.Selectable() != c.want {
			t.Errorf("%s/%s: %v", c.l, c.k, !c.want)
		}
	}
}

func TestAddKeepsTheStrictestLevel(t *testing.T) {
	it := Item{Level: Safe}
	it.Add("a", Review, "")
	it.Add("b", Safe, "")
	if it.Level != Review {
		t.Fatal(it.Level)
	}
	it.Add("c", Live, "")
	if it.Level != Live || !it.Has("b") || it.Has("z") {
		t.Fatal(it)
	}
}

func TestCommandLine(t *testing.T) {
	a := Action{Argv: []string{"/opt/homebrew/bin/sdkmanager", "--uninstall", "ndk;27.0", "it's"}}
	if got := a.CommandLine(); got != `/opt/homebrew/bin/sdkmanager --uninstall 'ndk;27.0' 'it'\''s'` {
		t.Fatal(got)
	}
	if got := a.Display(); got != `sdkmanager --uninstall 'ndk;27.0' 'it'\''s'` {
		t.Fatal(got)
	}
}

func TestSealIgnoresSizeButNotState(t *testing.T) {
	a := Item{ID: "x", Level: Safe, Bytes: 1, Action: Action{Kind: Command, Argv: []string{"go"}}}
	a.Seal()
	b := a
	b.Bytes = 99
	b.Seal()
	if a.Fingerprint != b.Fingerprint {
		t.Fatal("size changed the fingerprint")
	}
	b.Level = Live
	b.Seal()
	c := a
	c.Evidence = []string{"other inode"}
	c.Seal()
	if a.Fingerprint == b.Fingerprint || a.Fingerprint == c.Fingerprint {
		t.Fatal("state must change the fingerprint")
	}
}
