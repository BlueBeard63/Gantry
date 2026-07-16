package main

import (
	"flag"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/B-Commissions/Gantry/appshell"
	"github.com/B-Commissions/Gantry/docs"
)

// cmdDocs browses the embedded documentation: a sidebar with search and
// the category tree on the left, the rendered page on the right. Works
// fully offline - the docs travel inside the gantry exe.
func cmdDocs(args []string) error {
	fs := flag.NewFlagSet("docs", flag.ExitOnError)
	printOnly := fs.Bool("print", false, "print the page as plain markdown instead of the browser")
	if err := fs.Parse(args); err != nil {
		return err
	}

	pages, err := loadDocs()
	if err != nil {
		return err
	}

	start := 0
	if topic := strings.Join(fs.Args(), " "); topic != "" {
		start = bestMatch(pages, topic)
	}

	if *printOnly {
		fmt.Println(pages[start].raw)
		return nil
	}

	m := newDocsModel(pages, start)
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

type docPage struct {
	path     string // e.g. "shell/window.md"
	title    string // first heading
	category string // folder name, "" for root
	raw      string
	lower    string // lowercased raw for search
}

func loadDocs() ([]docPage, error) {
	var pages []docPage
	err := fs.WalkDir(docs.FS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".md") {
			return err
		}
		data, err := docs.FS.ReadFile(p)
		if err != nil {
			return err
		}
		raw := string(data)
		title := p
		for _, line := range strings.Split(raw, "\n") {
			if strings.HasPrefix(line, "# ") {
				title = strings.TrimPrefix(line, "# ")
				break
			}
		}
		category := ""
		if i := strings.Index(p, "/"); i >= 0 {
			category = p[:i]
		}
		pages = append(pages, docPage{path: p, title: title, category: category, raw: raw, lower: strings.ToLower(raw)})
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(pages) == 0 {
		return nil, fmt.Errorf("no docs embedded (build the CLI from the Gantry repo)")
	}
	// README first, then category order matching the reading order.
	rank := map[string]int{"": 0, "getting-started": 1, "shell": 2, "ui": 3, "cli": 4, "advanced": 5}
	sort.SliceStable(pages, func(i, j int) bool {
		ri, rj := rank[pages[i].category], rank[pages[j].category]
		if ri != rj {
			return ri < rj
		}
		return pages[i].path < pages[j].path
	})
	return pages, nil
}

func bestMatch(pages []docPage, topic string) int {
	t := strings.ToLower(topic)
	best, bestScore := 0, -1
	for i, p := range pages {
		score := 0
		if strings.Contains(strings.ToLower(p.title), t) {
			score += 100
		}
		if strings.Contains(p.path, t) {
			score += 50
		}
		score += strings.Count(p.lower, t)
		if score > bestScore {
			best, bestScore = i, score
		}
	}
	return best
}

var linkRe = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

type docLink struct{ label, target string }

func pageLinks(p docPage) []docLink {
	var links []docLink
	for _, m := range linkRe.FindAllStringSubmatch(p.raw, -1) {
		if strings.HasPrefix(m[2], "#") {
			continue
		}
		links = append(links, docLink{label: m[1], target: m[2]})
	}
	return links
}

// docsModel is the bubbletea model: focus moves between the sidebar
// (search + page list) and the content viewport; f enters link mode.
type docsModel struct {
	pages    []docPage
	filtered []int // indexes into pages matching the search
	cursor   int   // position in filtered
	current  int   // open page index
	history  []int
	future   []int

	search  textinput.Model
	view    viewport.Model
	links   []docLink
	linkIdx int
	mode    string // "browse" | "search" | "links"
	focus   string // "side" | "main"
	status  string
	width   int
	height  int
	ready   bool
}

func newDocsModel(pages []docPage, start int) *docsModel {
	ti := textinput.New()
	ti.Placeholder = "search (/)"
	ti.CharLimit = 64
	m := &docsModel{pages: pages, current: start, mode: "browse", focus: "side"}
	m.search = ti
	m.refilter("")
	for i, idx := range m.filtered {
		if idx == start {
			m.cursor = i
		}
	}
	return m
}

func (m *docsModel) refilter(q string) {
	q = strings.ToLower(strings.TrimSpace(q))
	m.filtered = m.filtered[:0]
	for i, p := range m.pages {
		if q == "" || strings.Contains(strings.ToLower(p.title), q) || strings.Contains(p.lower, q) {
			m.filtered = append(m.filtered, i)
		}
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = 0
	}
}

func (m *docsModel) open(idx int, pushHistory bool) {
	if idx < 0 || idx >= len(m.pages) {
		return
	}
	if pushHistory && idx != m.current {
		m.history = append(m.history, m.current)
		m.future = nil
	}
	m.current = idx
	m.links = pageLinks(m.pages[idx])
	m.linkIdx = 0
	m.render()
	m.view.GotoTop()
}

func (m *docsModel) render() {
	if !m.ready {
		return
	}
	width := m.view.Width
	if width < 20 {
		width = 20
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width-2),
	)
	if err != nil {
		m.view.SetContent(m.pages[m.current].raw)
		return
	}
	out, err := r.Render(m.pages[m.current].raw)
	if err != nil {
		out = m.pages[m.current].raw
	}
	m.view.SetContent(out)
}

func (m *docsModel) followLink(l docLink) {
	if strings.HasPrefix(l.target, "http://") || strings.HasPrefix(l.target, "https://") {
		if err := appshell.OpenInBrowser(l.target); err != nil {
			if clipboard.WriteAll(l.target) == nil {
				m.status = "could not open browser - link copied to clipboard"
			} else {
				m.status = "could not open: " + l.target
			}
		} else {
			m.status = "opened in browser: " + l.target
		}
		return
	}
	// Internal: resolve relative to the current page's folder.
	target := path.Clean(path.Join(path.Dir(m.pages[m.current].path), strings.Split(l.target, "#")[0]))
	for i, p := range m.pages {
		if p.path == target {
			m.open(i, true)
			m.mode = "browse"
			return
		}
	}
	m.status = "page not found: " + target
}

func (m *docsModel) Init() tea.Cmd { return nil }

func (m *docsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		side := m.sidebarWidth()
		m.view = viewport.New(m.width-side-3, m.height-2)
		m.ready = true
		m.render()
		return m, nil

	case tea.KeyMsg:
		m.status = ""
		if m.mode == "search" {
			switch msg.String() {
			case "esc":
				m.mode = "browse"
				m.search.Blur()
			case "enter":
				m.mode = "browse"
				m.search.Blur()
				if len(m.filtered) > 0 {
					m.open(m.filtered[m.cursor], true)
				}
			case "up":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down":
				if m.cursor < len(m.filtered)-1 {
					m.cursor++
				}
			default:
				var cmd tea.Cmd
				m.search, cmd = m.search.Update(msg)
				m.refilter(m.search.Value())
				return m, cmd
			}
			return m, nil
		}
		if m.mode == "links" {
			switch msg.String() {
			case "esc", "f":
				m.mode = "browse"
			case "up", "k":
				if m.linkIdx > 0 {
					m.linkIdx--
				}
			case "down", "j":
				if m.linkIdx < len(m.links)-1 {
					m.linkIdx++
				}
			case "enter":
				if len(m.links) > 0 {
					m.followLink(m.links[m.linkIdx])
				}
				if m.mode == "links" {
					m.mode = "browse"
				}
			case "q", "ctrl+c":
				return m, tea.Quit
			}
			return m, nil
		}
		// browse mode
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "/":
			m.mode = "search"
			m.focus = "side"
			m.search.Focus()
			return m, textinput.Blink
		case "f":
			if len(m.links) > 0 {
				m.mode = "links"
				m.linkIdx = 0
			} else {
				m.status = "no links on this page"
			}
		case "tab":
			if m.focus == "side" {
				m.focus = "main"
			} else {
				m.focus = "side"
			}
		case "b", "backspace":
			if n := len(m.history); n > 0 {
				idx := m.history[n-1]
				m.history = m.history[:n-1]
				m.future = append(m.future, m.current)
				m.open(idx, false)
			}
		case "n":
			if n := len(m.future); n > 0 {
				idx := m.future[n-1]
				m.future = m.future[:n-1]
				m.history = append(m.history, m.current)
				m.open(idx, false)
			}
		case "up", "k":
			if m.focus == "side" {
				if m.cursor > 0 {
					m.cursor--
				}
			} else {
				m.view.ScrollUp(1)
			}
		case "down", "j":
			if m.focus == "side" {
				if m.cursor < len(m.filtered)-1 {
					m.cursor++
				}
			} else {
				m.view.ScrollDown(1)
			}
		case "pgup":
			m.view.PageUp()
		case "pgdown", " ":
			m.view.PageDown()
		case "enter":
			if m.focus == "side" && len(m.filtered) > 0 {
				m.open(m.filtered[m.cursor], true)
			}
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.view, cmd = m.view.Update(msg)
	return m, cmd
}

func (m *docsModel) sidebarWidth() int {
	w := m.width / 3
	if w < 28 {
		w = 28
	}
	if w > 44 {
		w = 44
	}
	return w
}

var (
	sideStyle     = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderRight(true).BorderForeground(lipgloss.Color("240"))
	selStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	catStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("110")).Bold(true)
	statusStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	linkSelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	linkItemStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
)

func (m *docsModel) View() string {
	if !m.ready {
		return "loading..."
	}
	side := m.renderSidebar()
	main := m.view.View()
	body := lipgloss.JoinHorizontal(lipgloss.Top, sideStyle.Height(m.height-2).Render(side), " "+main)
	help := "tab focus | enter open | / search | f links | b back | q quit"
	if m.mode == "links" {
		help = "up/down pick link | enter follow | esc cancel"
	}
	if m.status != "" {
		help = m.status
	}
	return body + "\n" + statusStyle.Render(" "+help)
}

func (m *docsModel) renderSidebar() string {
	w := m.sidebarWidth() - 2
	var b strings.Builder
	b.WriteString(" " + m.search.View() + "\n\n")

	if m.mode == "links" {
		b.WriteString(catStyle.Render(" Links on this page") + "\n")
		for i, l := range m.links {
			line := fmt.Sprintf(" %d. %s", i+1, l.label)
			if len(line) > w {
				line = line[:w]
			}
			if i == m.linkIdx {
				b.WriteString(linkSelStyle.Render(line) + "\n")
			} else {
				b.WriteString(linkItemStyle.Render(line) + "\n")
			}
		}
		return b.String()
	}

	lastCat := "\x00"
	for i, idx := range m.filtered {
		p := m.pages[idx]
		if p.category != lastCat {
			lastCat = p.category
			label := p.category
			if label == "" {
				label = "start"
			}
			b.WriteString(catStyle.Render(" "+label) + "\n")
		}
		line := "  " + p.title
		if len(line) > w {
			line = line[:w]
		}
		switch {
		case i == m.cursor && m.focus == "side":
			b.WriteString(selStyle.Render("> "+line[2:]) + "\n")
		case idx == m.current:
			b.WriteString(selStyle.Render(line) + "\n")
		default:
			b.WriteString(dimStyle.Render(line) + "\n")
		}
	}
	return b.String()
}
