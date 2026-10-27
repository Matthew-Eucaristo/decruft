package ui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/Matthew-Eucaristo/decruft/internal/item"
)

func (m *Model) quit() (tea.Model, tea.Cmd) {
	m.cancel()
	return m, tea.Quit
}

func (m *Model) key(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	if k == "ctrl+c" {
		switch {
		case m.running && !m.stopping:
			m.stopping, m.quitWarn = true, true // let the running step finish
			return m, nil
		case m.running:
			if m.runCancel != nil {
				m.runCancel()
			}
		}
		return m.quit()
	}
	switch m.screen {
	case legend:
		m.screen = m.back
		return m, nil
	case confirming:
		m.screen = m.back
		if k == "y" {
			return m, m.startRun()
		}
		return m, nil
	case receipt:
		if k == "q" {
			return m.quit()
		}
		m.screen = overview
		return m, nil
	}
	if k == "q" {
		if m.running {
			m.quitWarn = true
			return m, nil
		}
		return m.quit()
	}
	m.quitWarn = false
	if k == "?" {
		m.back, m.screen = m.screen, legend
		return m, nil
	}
	if m.running {
		return m, nil // look around, but no new selections mid-run
	}
	switch k {
	case "R":
		return m, m.startScan()
	case "s":
		inList := m.screen == listing
		m.selectWhere(func(it *item.Item) bool { return it.Level == item.Safe && (!inList || it.Category == m.listCat) })
		return m, nil
	case "c":
		return m.confirm()
	}
	switch m.screen {
	case listing:
		return m.listKey(k)
	case treemap:
		return m.treemapKey(k)
	}
	return m.overviewKey(k)
}

func (m *Model) confirm() (tea.Model, tea.Cmd) {
	run, refuse := m.plan()
	if len(run)+len(refuse) == 0 {
		return m, nil
	}
	m.back, m.screen = m.screen, confirming
	return m, nil
}

// selectWhere selects every selectable, not yet removed item matching f.
func (m *Model) selectWhere(f func(*item.Item) bool) {
	for i := range m.items {
		it := &m.items[i]
		if it.Selectable() && m.status[it.ID] != done && f(it) {
			m.selected[it.ID] = true
		}
	}
}

// toggle selects a set of items, or clears it when all are selected.
func (m *Model) toggle(its []*item.Item) {
	all := true
	for _, it := range its {
		if it.Selectable() && m.status[it.ID] != done && !m.selected[it.ID] {
			all = false
		}
	}
	for _, it := range its {
		if !it.Selectable() || m.status[it.ID] == done {
			continue
		}
		if all {
			delete(m.selected, it.ID)
		} else {
			m.selected[it.ID] = true
		}
	}
}

func (m *Model) overviewKey(k string) (tea.Model, tea.Cmd) {
	cats := m.presentCats()
	i := 0
	for j, c := range cats {
		if c == m.curCat {
			i = j
		}
	}
	switch k {
	case "up", "k":
		if i > 0 {
			m.curCat, m.moved = cats[i-1], true
		}
	case "down", "j":
		if i < len(cats)-1 {
			m.curCat, m.moved = cats[i+1], true
		}
	case "enter", "right", "l":
		if len(cats) > 0 {
			m.listCat, m.listCursor, m.listOffset, m.screen = cats[i], 0, 0, listing
		}
	case "space":
		if len(cats) > 0 {
			m.toggle(m.catItems(cats[i]))
		}
	case "t":
		m.tmCat, m.tmKey, m.screen = -1, m.curCat.Key(), treemap
	}
	return m, nil
}

func (m *Model) listKey(k string) (tea.Model, tea.Cmd) {
	its := m.listItems()
	switch k {
	case "up", "k":
		m.listCursor = max(0, m.listCursor-1)
	case "down", "j":
		m.listCursor = min(len(its)-1, m.listCursor+1)
	case "pgup":
		m.listCursor = max(0, m.listCursor-10)
	case "pgdown":
		m.listCursor = min(len(its)-1, m.listCursor+10)
	case "home", "g":
		m.listCursor = 0
	case "end", "G":
		m.listCursor = len(its) - 1
	case "space":
		if it := m.listItem(); it != nil {
			m.toggle([]*item.Item{it})
		}
	case "a":
		m.toggle(its)
	case "enter":
		return m.confirm()
	case "o":
		if it := m.listItem(); it != nil && it.Action.Kind == item.Handoff {
			return m, m.handoff(it)
		}
	case "t":
		m.tmCat, m.screen = m.listCat, treemap
		if it := m.listItem(); it != nil {
			m.tmKey = it.ID
		}
	case "esc", "left", "h", "backspace":
		m.curCat, m.moved, m.screen = m.listCat, true, overview
	}
	return m, nil
}

func (m *Model) treemapKey(k string) (tea.Model, tea.Cmd) {
	ts := m.tiles()
	if len(ts) == 0 {
		if k == "esc" || k == "t" {
			m.screen = overview
		}
		return m, nil
	}
	cur := m.tmIndex(ts)
	cells := make([]cell, len(ts))
	for i, t := range ts {
		cells[i] = t.cell
	}
	move := func(dx, dy int) { m.tmKey = ts[nearest(cells, cur, dx, dy)].key }
	t := ts[cur]
	switch k {
	case "left", "h":
		move(-1, 0)
	case "right", "l":
		move(1, 0)
	case "up", "k":
		move(0, -1)
	case "down", "j":
		move(0, 1)
	case "enter":
		if m.tmCat < 0 && t.key != "rest" {
			m.tmCat, m.tmKey = t.cat, ""
		}
	case "esc", "backspace":
		if m.tmCat >= 0 {
			m.tmKey, m.tmCat = m.tmCat.Key(), -1
		} else {
			m.screen = overview
		}
	case "space":
		switch {
		case t.it != nil:
			m.toggle([]*item.Item{t.it})
		case m.tmCat < 0 && t.key != "rest":
			m.toggle(m.catItems(t.cat))
		}
	case "o":
		if t.it != nil && t.it.Action.Kind == item.Handoff {
			return m, m.handoff(t.it)
		}
	case "t":
		if m.tmCat >= 0 {
			m.listCat, m.listCursor, m.screen = m.tmCat, 0, listing
			for i, it := range m.listItems() {
				if it.ID == m.tmKey {
					m.listCursor = i
				}
			}
		} else {
			m.curCat, m.screen = t.cat, overview
		}
	}
	return m, nil
}
