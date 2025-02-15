package main

import (
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/timer"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sergi/go-diff/diffmatchpatch"
	flag "github.com/spf13/pflag"
)

var (
	flagInterval = flag.DurationP("interval", "n", 2*time.Second, "time to wait between updates")
	flagErrExit  = flag.BoolP("errexit", "e", false, "exit if command has a non-zero exit")
	flagChgExit  = flag.BoolP("chgexit", "g", false, "exit when the output of command changes")
	flagClassic  = flag.Bool("no-tui", false, "do not use the TUI")
	flagNoAlt    = flag.Bool("no-alt", false, "do not start the TUI in alt screen")
	flagLog      = flag.String("log", "", "write debug logs to file")
	flagDebug    = flag.Bool("debug", false, "enable tracing logs")
	flagHelp     = flag.BoolP("help", "h", false, "display this help and exit")
)

//go:embed banner.txt
var banner string

var (
	colorDark   = lipgloss.Color("55")
	colorBlue   = lipgloss.Color("19")
	colorViolet = lipgloss.Color("135")
	colorPurple = lipgloss.Color("141")
	colorPink   = lipgloss.Color("219")
	colorLight  = lipgloss.Color("225")
	colorErr    = lipgloss.Color("162")

	headerStyle = lipgloss.NewStyle().
			Background(colorDark).
			Foreground(colorLight).
			Padding(0, 1)

	pagerTitleStyle = lipgloss.NewStyle().
			Foreground(colorPink).
			Padding(1, 0, 0, 0).
			Align(lipgloss.Center)
	pagerStyle = lipgloss.NewStyle().
			Border(lipgloss.InnerHalfBlockBorder(), true, false).
			BorderForeground(colorBlue)

	listItemTitleStyle = lipgloss.NewStyle().
				Foreground(colorPink).
				Border(lipgloss.NormalBorder(), false, false, false, true).
				BorderForeground(colorViolet).
				Padding(0, 1)
	listItemDescStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, false, false, true).
				BorderForeground(colorViolet).
				Padding(0, 1)

	statusKeyStyle = lipgloss.NewStyle().Foreground(colorPurple)
	statusValStyle = lipgloss.NewStyle().Foreground(colorPink)
	statusBarStyle = lipgloss.NewStyle().Align(lipgloss.Center)
	kvSep          = lipgloss.NewStyle().Foreground(colorBlue).Render("=")
	statusSep      = lipgloss.NewStyle().Foreground(colorBlue).Render(" • ")

	helpKeyStyle  = lipgloss.NewStyle().Foreground(colorPink).Bold(true)
	helpDescStyle = lipgloss.NewStyle().Foreground(colorPurple)

	errStyle = lipgloss.NewStyle().Foreground(colorErr).Padding(1)
)

const (
	errTxtExit = "Watched program exit with non-zero exit status"
	errTxtChg  = "Watched program output changed"
)

type focussedView uint

const (
	focussedPager focussedView = iota
	focussedList
)

// Disable logging
const LevelNoLogs = slog.LevelError + 1

type model struct {
	interval time.Duration
	errExit  bool
	chgExit  bool
	alt      bool
	cmd      []string

	width  int
	height int

	// Whether to show line-level diff or not
	lineDiff bool
	// Whether to follow the latest output
	follow bool
	// Whether to paused the command loop
	paused bool
	// Which view is visible / focussed
	focus focussedView
	// Command output history
	hist map[time.Time]*historyEntry
	// Time at which we received the last command output
	prevT *time.Time
	// Which command output is selected and displayed
	seleT *time.Time

	dmp *diffmatchpatch.DiffMatchPatch

	keys  keyMap
	help  help.Model
	timer timer.Model
	pager viewport.Model
	list  list.Model
}

type keyMap struct {
	toggleAltScreen   key.Binding
	switchFocus       key.Binding
	listSelect        key.Binding
	switchContentUp   key.Binding
	switchContentDown key.Binding
	diffMode          key.Binding
	toggleFollow      key.Binding
	togglePause       key.Binding
}

const (
	switchFocusKey       = "tab"
	switchFocusDescList  = "content"
	switchFocusDescPager = "list"
)

func newModel(cmd []string) model {
	listDelegate := list.NewDefaultDelegate()
	listDelegate.Styles.SelectedTitle = listItemTitleStyle
	listDelegate.Styles.SelectedDesc = listItemDescStyle

	m := model{
		interval: *flagInterval,
		errExit:  *flagErrExit,
		chgExit:  *flagChgExit,
		alt:      !*flagNoAlt,
		width:    0,
		height:   0,
		lineDiff: true,
		follow:   true,
		paused:   false,
		focus:    focussedPager,
		cmd:      cmd,
		dmp:      diffmatchpatch.New(),
		hist:     make(map[time.Time]*historyEntry),
		prevT:    nil,
		seleT:    nil,
		keys: keyMap{
			toggleAltScreen: key.NewBinding(
				key.WithKeys("a"),
				key.WithHelp("a", "alt screen"),
			),
			switchFocus: key.NewBinding(
				key.WithKeys(switchFocusKey),
				key.WithHelp(switchFocusKey, switchFocusDescPager),
			),
			listSelect: key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "select"),
				key.WithDisabled(),
			),
			switchContentUp: key.NewBinding(
				key.WithKeys("shift+up", "K"),
				key.WithHelp("⇧+k", "content up"),
			),
			switchContentDown: key.NewBinding(
				key.WithKeys("shift+down", "J"),
				key.WithHelp("⇧+j", "content down"),
			),
			diffMode: key.NewBinding(
				key.WithKeys("d"),
				key.WithHelp("d", "switch diff mode"),
			),
			toggleFollow: key.NewBinding(
				key.WithKeys("f"),
				key.WithHelp("f", "toggle follow"),
			),
			togglePause: key.NewBinding(
				key.WithKeys("p"),
				key.WithHelp("p", "toggle pause"),
			),
		},
		help:  help.New(),
		timer: timer.Model{}, //nolint:exhaustruct // We don't use it before re-creating it
		pager: viewport.New(0, 0),
		list:  list.New([]list.Item{}, listDelegate, 0, 0),
	}

	m.help.Styles.ShortKey = helpKeyStyle
	m.help.Styles.ShortDesc = helpDescStyle
	m.help.Styles.FullKey = helpKeyStyle
	m.help.Styles.FullDesc = helpDescStyle

	m.list.SetShowTitle(false)
	m.list.SetShowStatusBar(false)
	m.list.SetShowHelp(false)
	m.list.InfiniteScrolling = false
	m.list.Filter = list.UnsortedFilter
	m.list.KeyMap = list.KeyMap{
		CursorUp: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		CursorDown: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		NextPage: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "next page"),
		),
		PrevPage: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "prev page"),
		),
		GoToStart: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("g/home", "go to start"),
		),
		GoToEnd: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("G/end", "go to end"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		ClearFilter: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "clear filter"),
		),
		CancelWhileFiltering: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
		AcceptWhileFiltering: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "apply filter"),
		),
		ShowFullHelp: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "more"),
		),
		CloseFullHelp: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "close help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", "quit"),
		),
		ForceQuit: key.NewBinding(key.WithKeys("ctrl+c")),
	}

	m.pager.Style = pagerStyle
	m.pager.KeyMap = viewport.KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "scroll up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "scroll down"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "f"),
			key.WithHelp("f/pgdn", "page down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "b"),
			key.WithHelp("b/pgup", "page up"),
		),
		HalfPageUp: key.NewBinding(
			key.WithKeys("u", "ctrl+u"),
			key.WithHelp("u", "½ page up"),
		),
		HalfPageDown: key.NewBinding(
			key.WithKeys("d", "ctrl+d"),
			key.WithHelp("d", "½ page down"),
		),
	}

	return m
}

type historyEntry struct {
	plain        string
	diffC, diffL *string
	prevT        *time.Time
}

func newHistoryEntry(txt string, prevT *time.Time) *historyEntry {
	return &historyEntry{plain: txt, prevT: prevT, diffC: nil, diffL: nil}
}

type listItem struct {
	t         time.Time
	title     string
	nChars    int
	nLines    int
	levDist   *int
	additions *int
	deletions *int
}

func newListItem(t time.Time, chars, lines int) listItem {
	return listItem{
		t: t, title: t.String(), nChars: chars, nLines: lines,
		levDist: nil, additions: nil, deletions: nil,
	}
}
func (i listItem) Title() string       { return i.title }
func (i listItem) FilterValue() string { return i.title }
func (i listItem) Description() string {
	return fmt.Sprintf("chars=%d lines=%d lev=%s +%s -%s",
		i.nChars, i.nLines, intp2String(i.levDist), intp2String(i.additions), intp2String(i.deletions))
}

func (i *listItem) update(dmp *diffmatchpatch.DiffMatchPatch, diffs []diffmatchpatch.Diff) {
	i.levDist = new(int)
	*i.levDist = dmp.DiffLevenshtein(diffs)

	i.additions = new(int)
	i.deletions = new(int)
	for _, d := range diffs {
		switch d.Type {
		case diffmatchpatch.DiffInsert:
			*i.additions++
		case diffmatchpatch.DiffDelete:
			*i.deletions++
		}
	}
}

type cmdMsg struct {
	out []byte
	err error
}

func (m model) Init() tea.Cmd {
	return m.runCmd
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	slog.Debug("New message", "type", reflect.TypeOf(msg))

	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		if !m.list.SettingFilter() {
			cmd = m.handleKey(msg)
			cmds = append(cmds, cmd)
		}

	case cmdMsg:
		cmd, earlyExit := m.handleCmdCycle(msg)
		if earlyExit {
			return m, cmd
		}
		cmds = append(cmds, cmd)

	case timer.TickMsg, timer.StartStopMsg:
		m.timer, cmd = m.timer.Update(msg)
		cmds = append(cmds, cmd)

	case timer.TimeoutMsg:
		m.timer, cmd = m.timer.Update(msg)
		cmds = append(cmds, cmd, m.runCmd)

	}

	switch m.focus {
	case focussedList:
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	case focussedPager:
		m.pager, cmd = m.pager.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *model) handleKey(msg tea.KeyMsg) tea.Cmd {
	slog.Debug("Key press", "key", msg.String())

	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
		lkm  = &m.list.KeyMap
	)

	switch {

	case key.Matches(msg, m.keys.toggleAltScreen):
		cmd = tea.EnterAltScreen
		if m.alt {
			cmd = tea.ExitAltScreen
		}
		m.alt = !m.alt
		cmds = append(cmds, cmd)

	case key.Matches(msg, m.keys.switchFocus):
		switch m.focus {
		case focussedList:
			m.focus = focussedPager
			m.keys.switchFocus.SetHelp(switchFocusKey, switchFocusDescPager)
			m.keys.listSelect.SetEnabled(false)
			cmd = m.switchContent()
			cmds = append(cmds, cmd)
		case focussedPager:
			m.focus = focussedList
			m.keys.switchFocus.SetHelp(switchFocusKey, switchFocusDescList)
			m.keys.listSelect.SetEnabled(true)
		}

	case key.Matches(msg, m.keys.listSelect):
		if m.focus == focussedList && !m.list.SettingFilter() {
			m.focus = focussedPager
			m.keys.switchFocus.SetHelp(switchFocusKey, switchFocusDescPager)
			m.keys.listSelect.SetEnabled(false)
			cmd = m.switchContent()
			cmds = append(cmds, cmd)
		}

	case key.Matches(msg, lkm.CursorUp, lkm.CursorDown, lkm.NextPage, lkm.PrevPage):
		if m.focus == focussedList {
			m.follow = false
		}

	case key.Matches(msg, lkm.Filter):
		if m.focus == focussedList {
			m.keys.listSelect.SetEnabled(false)
		}

	case key.Matches(msg, lkm.ClearFilter, lkm.CancelWhileFiltering, lkm.AcceptWhileFiltering):
		if m.focus == focussedList {
			m.keys.listSelect.SetEnabled(true)
		}

	case key.Matches(msg, m.keys.switchContentUp):
		m.follow = false
		m.list.CursorUp()
		cmd = m.switchContent()
		cmds = append(cmds, cmd)

	case key.Matches(msg, m.keys.switchContentDown):
		m.follow = false
		m.list.CursorDown()
		cmd = m.switchContent()
		cmds = append(cmds, cmd)

	case key.Matches(msg, m.keys.diffMode):
		m.lineDiff = !m.lineDiff
		cmd = m.switchDiffContent()
		cmds = append(cmds, cmd)

	case key.Matches(msg, m.keys.toggleFollow):
		m.follow = !m.follow
		if m.follow {
			m.list.ResetFilter()
			i := m.list.Index()
			m.list.ResetSelected()
			if m.focus == focussedPager && i != 0 {
				cmd = m.switchContent()
				cmds = append(cmds, cmd)
			}
		}

	case key.Matches(msg, m.keys.togglePause):
		m.paused = !m.paused
		cmd = m.timer.Toggle()
		cmds = append(cmds, cmd)
		slog.Debug("Timer toggle", "t", m.timer.Timeout, "paused", m.paused)

	case key.Matches(msg, lkm.ClearFilter):
		if m.focus == focussedPager {
			m.list.ResetFilter()
		}

	case key.Matches(msg, lkm.ShowFullHelp, lkm.CloseFullHelp):
		m.help.ShowAll = !m.help.ShowAll

	case key.Matches(msg, lkm.Quit, lkm.ForceQuit):
		cmd = tea.Quit
		cmds = append(cmds, cmd)

	}

	return tea.Batch(cmds...)
}

func (m *model) handleCmdCycle(msg cmdMsg) (tea.Cmd, bool) {
	slog.Debug("Command completed")

	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	now := time.Now()
	msgS := string(msg.out)
	isDifferent := false

	if m.prevT == nil {
		isDifferent = true
		m.seleT = &now
		m.pager.SetContent(msgS)
	} else if m.hist[*m.prevT].plain != msgS {
		isDifferent = true
	}

	if isDifferent {
		m.hist[now] = newHistoryEntry(msgS, m.prevT)
		m.prevT = &now
		cmd = m.list.InsertItem(0, newListItem(now, len(msgS), strings.Count(msgS, "\n")))
		cmds = append(cmds, cmd)
		if m.follow {
			cmd = m.switchContent()
			cmds = append(cmds, cmd)
		} else {
			m.list.CursorDown()
		}
	}

	if msg.err != nil {
		var ee *exec.ExitError
		if errors.As(msg.err, &ee) {
			if !ee.ProcessState.Success() && m.errExit {
				printErr(errTxtExit)
				return tea.Quit, true
			}
		} else {
			printErrf("Failed to run command: %v", msg.err)
			return tea.Quit, true
		}
	}

	if m.chgExit && m.prevT != nil && isDifferent {
		printErr(errTxtChg)
		return tea.Quit, true
	}

	if !m.paused {
		m.timer = timer.New(m.interval)
		cmds = append(cmds, m.timer.Init())
	}

	return tea.Batch(cmds...), false
}

func (m *model) switchContent() tea.Cmd {
	return m.doSwitchContent(false)
}

func (m *model) switchDiffContent() tea.Cmd {
	return m.doSwitchContent(true)
}

func (m *model) doSwitchContent(changedDiffMode bool) tea.Cmd {
	si := m.list.SelectedItem()
	sli, ok := si.(listItem)
	if !ok {
		printErrf("Unexpected list item type: %v", si)
		return tea.Quit
	}
	if !changedDiffMode && m.seleT != nil && sli.t == *m.seleT {
		return nil
	}
	var (
		content *string
		cmd     tea.Cmd
	)
	seleHist := m.hist[sli.t]
	if seleHist.prevT == nil {
		slog.Debug("Switching content to oldest entry")
		content = &seleHist.plain
	} else {
		slog.Debug("Switching content to diff", "lineDiff", m.lineDiff)
		prevHist := m.hist[*seleHist.prevT]
		if m.lineDiff {
			if seleHist.diffL == nil {
				slog.Debug("Computing line diff")
				ti1, ti2, linesIdx := m.dmp.DiffLinesToChars(prevHist.plain, seleHist.plain)
				diffChars := m.dmp.DiffMain(ti1, ti2, true)
				diffs := m.dmp.DiffCharsToLines(diffChars, linesIdx)
				sli.update(m.dmp, diffs)
				cmd = m.list.SetItem(m.list.Index(), sli)
				diffsPretty := m.dmp.DiffPrettyText(diffs)
				seleHist.diffL = &diffsPretty
			}
			content = seleHist.diffL
		} else {
			if seleHist.diffC == nil {
				slog.Debug("Computing char diff")
				diffs := m.dmp.DiffMain(prevHist.plain, seleHist.plain, true)
				diffs = m.dmp.DiffCleanupSemanticLossless(diffs)
				sli.update(m.dmp, diffs)
				cmd = m.list.SetItem(m.list.Index(), sli)
				diffsPretty := m.dmp.DiffPrettyText(diffs)
				seleHist.diffC = &diffsPretty
			}
			content = seleHist.diffC
		}
	}
	slog.Debug("Setting content")
	m.pager.SetContent(*content)
	m.seleT = &sli.t
	return cmd
}

func (m model) headerView() string {
	left := fmt.Sprintf("Every %s: %s", m.interval, strings.Join(m.cmd, " "))
	time := fmt.Sprintf("Next in %s", m.timer.View())
	sty := lipgloss.NewStyle().Width(m.width/2 - 1)
	s := lipgloss.JoinHorizontal(lipgloss.Center,
		sty.Align(lipgloss.Left).Render(left),
		sty.Align(lipgloss.Right).Render(time))
	return headerStyle.Render(s)
}

func (m model) pagerTitleView() string {
	var s string
	if m.seleT == nil {
		s = "n/a"
	} else {
		s = m.seleT.String()
	}
	return pagerTitleStyle.Width(m.width).Render(s)
}

func (m model) statusView() string {
	var (
		diffMode string
		nItems   int
		filtered string
	)

	if m.lineDiff {
		diffMode = "line"
	} else {
		diffMode = "char"
	}

	if m.list.IsFiltered() {
		nItems = len(m.list.VisibleItems())
		filtered = "(filtered)"
	} else {
		nItems = len(m.list.Items())
	}

	renderKV := func(k, v string) string {
		return statusKeyStyle.Render(k) + kvSep + statusValStyle.Render(v)
	}

	out := renderKV("diff", diffMode) + statusSep
	out += renderKV("follow", bool2String(m.follow)) + statusSep
	out += renderKV("paused", bool2String(m.paused)) + statusSep
	out += renderKV("alt", bool2String(m.alt)) + statusSep
	out += renderKV("selected", fmt.Sprintf("%d/%d", m.list.Index()+1, nItems)+filtered)

	return statusBarStyle.Width(m.width).Render(out)
}

func (m model) helpListView() string {
	lkm := m.list.KeyMap
	if m.help.ShowAll {
		return m.help.FullHelpView([][]key.Binding{
			{lkm.CursorUp, lkm.CursorDown, lkm.PrevPage, lkm.NextPage, lkm.GoToStart, lkm.GoToEnd},
			{
				m.keys.switchFocus,
				lkm.Filter, lkm.ClearFilter, lkm.AcceptWhileFiltering, lkm.CancelWhileFiltering,
				lkm.CloseFullHelp, lkm.Quit,
			},
		})
	}
	return m.help.ShortHelpView([]key.Binding{
		m.keys.switchFocus, lkm.ShowFullHelp, lkm.Quit,
	})
}

func (m model) helpPagerView() string {
	pkm := m.pager.KeyMap
	if m.help.ShowAll {
		return m.help.FullHelpView([][]key.Binding{
			{pkm.Up, pkm.Down, pkm.PageUp, pkm.PageDown, pkm.HalfPageUp, pkm.HalfPageDown},
			{
				m.keys.switchContentUp, m.keys.switchContentDown,
				m.keys.diffMode, m.keys.toggleFollow, m.keys.togglePause,
				m.keys.toggleAltScreen,
			},
			{m.keys.switchFocus, m.list.KeyMap.ClearFilter, m.list.KeyMap.CloseFullHelp, m.list.KeyMap.Quit},
		})
	}
	return m.help.ShortHelpView([]key.Binding{
		m.keys.switchFocus, m.list.KeyMap.ShowFullHelp, m.list.KeyMap.Quit,
	})
}

func (m model) helpView() string {
	var view string
	if m.focus == focussedList {
		view = m.helpListView()
	} else {
		view = m.helpPagerView()
	}
	sty := lipgloss.NewStyle().Margin(1, 1, 0, 1)
	if !m.help.ShowAll {
		sty = sty.Width(m.width - 2).Align(lipgloss.Center)
	}
	return sty.Render(view)
}

func (m model) View() string {
	headerView := m.headerView()
	headerHeight := lipgloss.Height(headerView)

	views := []string{headerView}

	statusView := m.statusView()
	statusHeight := lipgloss.Height(statusView)

	m.help.Width = m.width - 2
	helpView := m.helpView()
	helpHeight := lipgloss.Height(helpView)

	switch m.focus {
	case focussedList:
		m.list.SetSize(m.width, m.height-headerHeight-statusHeight-helpHeight)
		views = append(views, m.list.View())
	case focussedPager:
		pagerTitleView := m.pagerTitleView()
		pagerTitleHeight := lipgloss.Height(pagerTitleView)
		m.pager.Width = m.width
		m.pager.Height = m.height - pagerTitleHeight - headerHeight - statusHeight - helpHeight
		views = append(views, pagerTitleView, m.pager.View())
	}
	views = append(views, statusView, helpView)
	return lipgloss.JoinVertical(lipgloss.Top, views...)
}

func (m model) runCmd() tea.Msg {
	cmd := exec.Command(m.cmd[0], m.cmd[1:]...) //nolint: gosec
	out, err := cmd.Output()
	return cmdMsg{out, err}
}

func mainTea(cmd []string) {
	m := newModel(cmd)

	var opts []tea.ProgramOption
	if !*flagNoAlt {
		opts = append(opts, tea.WithAltScreen())
	}
	if _, err := tea.NewProgram(m, opts...).Run(); err != nil {
		printErrf("Oops! %v", err)
		os.Exit(1)
	}
}

func mainClassic(cmd []string) {
	var prevOut *string
	for {
		fmt.Println("\x1B[2J\x1B[1;1H")

		c := exec.Command(cmd[0], cmd[1:]...) //nolint: gosec
		out, err := c.Output()
		outS := string(out)
		fmt.Println(outS)

		if err != nil && *flagErrExit {
			if ee, ok := err.(*exec.ExitError); ok {
				printErr(errTxtExit)
				if len(ee.Stderr) > 0 {
					printErrf("%s", ee.Stderr)
				}
			}
			os.Exit(c.ProcessState.ExitCode())
		}

		if *flagChgExit && prevOut != nil && *prevOut != outS {
			printErr(errTxtChg)
			os.Exit(c.ProcessState.ExitCode())
		}

		prevOut = &outS

		time.Sleep(*flagInterval)
	}
}

func usage() {
	bannerStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true).
		BorderForeground(colorViolet).
		Foreground(colorPink).
		Padding(3, 3, 1, 3).
		Margin(1, 3, 0, 3)
	progStyle := lipgloss.NewStyle().Foreground(colorPurple).Bold(true)
	commandStyle := lipgloss.NewStyle().Foreground(colorPink).Underline(true)
	optsStyle := lipgloss.NewStyle().Foreground(colorDark)
	usage := fmt.Sprintf("%s\n\n%s %s %s\n\n%s",
		bannerStyle.Render(banner),
		progStyle.Render(os.Args[0]),
		optsStyle.Render("[options]"),
		commandStyle.Render("command"),
		flag.CommandLine.FlagUsages(),
	)
	fmt.Fprintf(os.Stdout, "%s\n", lipgloss.NewStyle().Margin(0, 1).Render(usage))
}

func main() {
	flag.CommandLine.SortFlags = false
	flag.CommandLine.SetInterspersed(false)
	flag.Usage = usage

	flag.Parse()
	if *flagHelp {
		flag.Usage()
		os.Exit(0)
	}

	cmd := flag.Args()
	if len(cmd) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	var (
		// Requesting a minimum log level that is greater than the maximum used (i.e. error).
		// It should not try to actually print anything.
		logF = os.Stderr
		logL = LevelNoLogs
		err  error
	)

	if len(*flagLog) > 0 {
		logF, err = os.OpenFile(*flagLog, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
		if err != nil {
			printErrf("Cannot open log file: %v", err)
			os.Exit(1)
		}
		defer logF.Close()
		if *flagDebug {
			logL = slog.LevelDebug
		} else {
			logL = slog.LevelInfo
		}
	}

	slogOpts := slog.HandlerOptions{AddSource: true, Level: logL, ReplaceAttr: nil}
	logger := slog.New(slog.NewTextHandler(logF, &slogOpts))
	slog.SetDefault(logger)

	slog.Debug("startup", "colorProfile", lipgloss.DefaultRenderer().ColorProfile())

	if *flagClassic {
		mainClassic(cmd)
	} else {
		mainTea(cmd)
	}
}

func printErr(s string)             { fmt.Fprintf(os.Stderr, "%s\n", errStyle.Render(s)) }
func printErrf(f string, vs ...any) { printErr(fmt.Sprintf(f, vs...)) }

func bool2String(v bool) string {
	if v {
		return "y"
	}
	return "n"
}

func intp2String(v *int) string {
	if v == nil {
		return "n/a"
	}
	return fmt.Sprint(*v)
}
