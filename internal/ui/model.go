// Package ui is decruft's terminal interface: an Apple-style storage bar
// with real developer categories, a list with the exact command for every
// item, a squarified treemap to see what is big, and a measured receipt.
package ui

import (
	"context"
	"os/exec"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/Matthew-Eucaristo/decruft/internal/act"
	"github.com/Matthew-Eucaristo/decruft/internal/disk"
	"github.com/Matthew-Eucaristo/decruft/internal/family"
	"github.com/Matthew-Eucaristo/decruft/internal/inventory"
	"github.com/Matthew-Eucaristo/decruft/internal/item"
	"github.com/Matthew-Eucaristo/decruft/internal/report"
)

// Options configure the UI.
type Options struct {
	Scanner *inventory.Scanner
	Exec    *act.Executor
	Home    string
	Version string
	Disk    func(context.Context, string) disk.Info
}

type screen int

const (
	overview screen = iota
	listing
	treemap
	confirming
	receipt
	legend
)

type state int

const (
	idle state = iota
	queued
	working
	done
	failed
)

// Model is the Bubble Tea model.
type Model struct {
	opts          Options
	width, height int
	pal           palette
	dark, mono    bool

	ctx    context.Context
	cancel context.CancelFunc

	// scan
	gen      int
	sources  []inventory.Source
	raw      []item.Item
	items    []item.Item
	warnings []string
	dupes    int
	scanning bool
	disk     disk.Info
	haveDisk bool
	spin     spinner.Model

	// selection and progress
	selected map[string]bool
	status   map[string]state
	errs     map[string]string

	// navigation
	screen, back screen
	curCat       item.Category
	moved        bool // the user moved the cursor; until then it follows the top row
	listCat      item.Category
	listCursor   int
	listOffset   int
	tmCat        item.Category // -1: all categories
	tmKey        string        // the tile under the cursor: a category key or an item ID

	// run
	runCtx    context.Context
	runCancel context.CancelFunc
	exec      act.Executor
	steps     []act.Step
	stepAt    int
	running   bool
	stopping  bool // ctrl+c once: finish the current step, skip the rest
	quitWarn  bool
	runStart  time.Time
	rcpt      report.Receipt
	ranOnce   bool

	anim      springs
	animating bool
}

// New builds the model.
func New(o Options) *Model {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Model{
		opts: o, ctx: ctx, cancel: cancel, width: 100, height: 30,
		selected: map[string]bool{}, status: map[string]state{}, errs: map[string]string{},
		tmCat: -1, anim: newSprings(), dark: true,
		spin: spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(faint)),
	}
	m.pal = newPalette(m.dark, m.mono)
	m.rcpt = report.Receipt{Before: -1, After: -1}
	return m
}

// Receipt is what the run did, for printing after the program exits.
func (m *Model) Receipt() report.Receipt { return m.rcpt }

type (
	eventMsg struct {
		gen int
		ev  inventory.Event
		ok  bool
		ch  <-chan inventory.Event
	}
	diskMsg struct {
		gen  int
		info disk.Info
	}
	stepMsg struct {
		outs          []act.Outcome
		before, after int64
	}
	frameMsg   struct{}
	handoffMsg struct{ err error }
)

func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.startScan(), m.spin.Tick, tea.RequestBackgroundColor)
}

// startScan (re)starts every source and the disk reading.
func (m *Model) startScan() tea.Cmd {
	m.gen++
	m.scanning = true
	m.raw, m.items, m.warnings, m.dupes = nil, nil, nil, 0
	m.sources = m.opts.Scanner.Names()
	ch := m.opts.Scanner.Start(m.ctx)
	gen := m.gen
	readDisk := func() tea.Msg { return diskMsg{gen, m.opts.Disk(m.ctx, m.opts.Home)} }
	return tea.Batch(waitEvent(gen, ch), readDisk)
}

func waitEvent(gen int, ch <-chan inventory.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return eventMsg{gen: gen}
		}
		return eventMsg{gen: gen, ev: ev, ok: true, ch: ch}
	}
}

func frame() tea.Cmd {
	return tea.Tick(time.Second/fps, func(time.Time) tea.Msg { return frameMsg{} })
}

// animate retargets the springs and starts the frame clock if needed.
func (m *Model) animate() tea.Cmd {
	for _, c := range item.Categories() {
		m.anim.set(catKey(c), float64(m.catBytes(c)))
	}
	if m.haveDisk {
		m.anim.set("free", float64(max(m.disk.Free, 0)))
	}
	if m.animating || m.anim.settled() {
		return nil
	}
	m.animating = true
	return frame()
}

func catKey(c item.Category) string { return "cat:" + c.Key() }

// catBytes is what a category still holds: items not yet removed.
func (m *Model) catBytes(c item.Category) int64 {
	var n int64
	for i := range m.items {
		it := &m.items[i]
		if it.Category == c && m.status[it.ID] != done {
			n += max(it.Size(), 0)
		}
	}
	return n
}

func (m *Model) merge() {
	m.items, m.dupes = inventory.Dedupe(inventory.Visible(m.raw))
	inventory.Sort(m.items)
	for id := range m.selected {
		if m.find(id) == nil {
			delete(m.selected, id)
		}
	}
}

func (m *Model) find(id string) *item.Item {
	for i := range m.items {
		if m.items[i].ID == id {
			return &m.items[i]
		}
	}
	return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.BackgroundColorMsg:
		m.dark = msg.IsDark()
		m.pal = newPalette(m.dark, m.mono)
		return m, nil
	case tea.ColorProfileMsg:
		m.mono = msg.Profile == colorprofile.Ascii || msg.Profile == colorprofile.NoTTY
		m.pal = newPalette(m.dark, m.mono)
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	case eventMsg:
		if msg.gen != m.gen {
			return m, nil
		}
		if !msg.ok {
			m.scanning = false
			return m, m.animate()
		}
		for i := range m.sources {
			if m.sources[i].Name == msg.ev.Source.Name {
				m.sources[i] = msg.ev.Source
			}
		}
		m.raw = append(m.raw, msg.ev.Items...)
		m.warnings = append(m.warnings, msg.ev.Warnings...)
		m.merge()
		return m, tea.Batch(waitEvent(msg.gen, msg.ch), m.animate())
	case diskMsg:
		if msg.gen == m.gen {
			m.disk, m.haveDisk = msg.info, true
		}
		return m, m.animate()
	case frameMsg:
		if !m.anim.step() {
			m.animating = false
			return m, nil
		}
		return m, frame()
	case stepMsg:
		return m, m.stepDone(msg)
	case handoffMsg:
		return m, m.startScan()
	case tea.KeyPressMsg:
		return m.key(msg)
	}
	return m, nil
}

// --- running ---------------------------------------------------------------

// plan is the selection in display order, split into what will run and what
// is selected but not allowed (shown as "not included").
func (m *Model) plan() (run, refuse []item.Item) {
	for _, it := range m.items {
		if !m.selected[it.ID] || m.status[it.ID] == done {
			continue
		}
		if it.Selectable() && (it.Action.Kind != item.Delegate || m.toolReady(it.Delegate.Tool)) {
			run = append(run, it)
		} else {
			refuse = append(refuse, it)
		}
	}
	return
}

func (m *Model) toolReady(name string) bool {
	for _, s := range m.sources {
		if s.Name == name && s.Tool != nil && s.Tool.Contract {
			return true
		}
	}
	return false
}

func (m *Model) startRun() tea.Cmd {
	run, _ := m.plan()
	if len(run) == 0 {
		return nil
	}
	// The executor gets its own copy of the tools, so a scan finishing
	// meanwhile never touches what a running step reads.
	m.exec = *m.opts.Exec
	m.exec.Tools = map[string]family.Tool{}
	for _, s := range m.sources {
		if s.Tool != nil {
			m.exec.Tools[s.Name] = *s.Tool
		}
	}
	m.steps, m.stepAt = act.Plan(run), 0
	for _, it := range run {
		m.status[it.ID] = queued
	}
	m.running, m.stopping, m.ranOnce = true, false, true
	m.runStart = time.Now()
	m.rcpt = report.Receipt{Before: -1, After: -1}
	m.runCtx, m.runCancel = context.WithCancel(m.ctx)
	m.screen = overview
	return m.nextStep()
}

func (m *Model) nextStep() tea.Cmd {
	if m.stepAt >= len(m.steps) || m.stopping {
		return m.finishRun()
	}
	st := m.steps[m.stepAt]
	for _, it := range st.Items {
		m.status[it.ID] = working
	}
	ex, ctx, home, first := &m.exec, m.runCtx, m.opts.Home, m.stepAt == 0
	return func() tea.Msg {
		var before int64 = -1
		if first {
			before = ex.Free(home)
		}
		outs := ex.Do(ctx, st)
		return stepMsg{outs: outs, before: before, after: ex.Free(home)}
	}
}

func (m *Model) stepDone(msg stepMsg) tea.Cmd {
	if msg.before >= 0 {
		m.rcpt.Before = msg.before
	}
	m.rcpt.After = msg.after
	for _, o := range msg.outs {
		m.rcpt.Outcomes = append(m.rcpt.Outcomes, o)
		switch o.How {
		case act.Refused, act.Failed:
			m.status[o.ID] = failed
			m.errs[o.ID] = o.Error
		case act.Dry:
			m.status[o.ID] = idle
		default:
			m.status[o.ID] = done
			delete(m.selected, o.ID)
			if it := m.find(o.ID); it != nil && it.Regrows {
				m.rcpt.Regrows = true
			}
		}
	}
	if msg.after >= 0 && m.haveDisk {
		m.disk.Free = msg.after
	}
	m.stepAt++
	return tea.Batch(m.animate(), m.nextStep())
}

func (m *Model) finishRun() tea.Cmd {
	for id, s := range m.status {
		if s == queued || s == working {
			m.status[id] = idle
		}
	}
	m.running = false
	m.rcpt.Took = time.Since(m.runStart)
	if m.runCancel != nil {
		m.runCancel()
	}
	m.screen = receipt
	// The receipt shows free space growing from before to after.
	if m.rcpt.Before >= 0 {
		m.anim.vals["free"] = &sval{pos: float64(m.rcpt.Before), target: float64(m.rcpt.Before)}
	}
	return m.animate()
}

// handoff opens a family tool without a contract in the terminal; decruft
// rescans when it exits.
func (m *Model) handoff(it *item.Item) tea.Cmd {
	for _, s := range m.sources {
		if s.Name != it.Source || s.Tool == nil {
			continue
		}
		args := []string{}
		if m.opts.Exec.Mode == act.DryRun {
			args = append(args, "--dry-run")
		}
		args = append(args, s.Tool.Roots...)
		if len(args) > 0 && s.Name == "flutterkill" && len(s.Tool.Roots) > 1 {
			args = args[:len(args)-len(s.Tool.Roots)+1]
		}
		return tea.ExecProcess(exec.Command(s.Tool.Path, args...), func(err error) tea.Msg { return handoffMsg{err} })
	}
	return nil
}
