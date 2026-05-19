package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
)

type browseTarget int

const (
	browseInput browseTarget = iota
	browseOutput
)

type browseItem struct {
	name string
	path string
	dir  bool
}

func (i browseItem) Title() string {
	if i.dir {
		return i.name + string(os.PathSeparator)
	}
	return i.name
}
func (i browseItem) Description() string { return i.path }
func (i browseItem) FilterValue() string { return i.name }

func newBrowseList() list.Model {
	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.SetShowTitle(false)
	l.DisableQuitKeybindings()
	l.SetFilteringEnabled(true)
	l.Styles.FilterPrompt = lipgloss.NewStyle().Foreground(colorAccent)
	l.Styles.FilterCursor = lipgloss.NewStyle().Foreground(colorAccent)
	return l
}

func readDirItems(dir string) ([]list.Item, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var dirs []browseItem
	var files []browseItem
	for _, e := range ents {
		name := e.Name()
		// Hide common noisy paths.
		if name == ".git" || name == "bin" {
			continue
		}
		p := filepath.Join(dir, name)
		it := browseItem{name: name, path: p, dir: e.IsDir()}
		if it.dir {
			dirs = append(dirs, it)
		} else {
			files = append(files, it)
		}
	}

	sort.Slice(dirs, func(i, j int) bool { return strings.ToLower(dirs[i].name) < strings.ToLower(dirs[j].name) })
	sort.Slice(files, func(i, j int) bool { return strings.ToLower(files[i].name) < strings.ToLower(files[j].name) })

	out := make([]list.Item, 0, len(dirs)+len(files))
	for _, d := range dirs {
		out = append(out, d)
	}
	for _, f := range files {
		out = append(out, f)
	}
	return out, nil
}

func browseHeader(dir string, target browseTarget) string {
	t := "Input"
	if target == browseOutput {
		t = "Output"
	}
	title := lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("Browse (%s)", t))
	where := lipgloss.NewStyle().Foreground(colorDim).Render(dir)
	return title + "\n" + where + "\n" + lipgloss.NewStyle().Foreground(colorDim).Render("Enter: open/select  Backspace: up  Esc: back")
}
