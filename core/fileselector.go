package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

var (
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	dirStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	pageStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	ErrCanceled = errors.New("canceled")
)

type FileSelector struct {
	selected       string
	dir            string
	rootDir        string
	Selected       map[string]*FileHeader
	nBytesSelected int64
	filter         string
	page           int
}

func NewFileSelector(dir string) *FileSelector {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	return &FileSelector{
		dir:      abs,
		Selected: make(map[string]*FileHeader),
		page:     0,
	}
}

func (f *FileSelector) filteredEntries() ([]os.DirEntry, error) {
	entries, err := os.ReadDir(f.dir)
	if err != nil {
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})

	if f.filter != "" {
		filtered := make([]os.DirEntry, 0)
		filterLower := strings.ToLower(f.filter)
		for _, entry := range entries {
			if strings.Contains(strings.ToLower(entry.Name()), filterLower) {
				filtered = append(filtered, entry)
			}
		}
		return filtered, nil
	}

	return entries, nil
}

func (f *FileSelector) RunRecur() error {
	entries, err := f.filteredEntries()
	if err != nil {
		return err
	}

	totalItems := len(entries)
	totalPages := (totalItems + PAGESIZE - 1) / PAGESIZE
	if totalPages == 0 {
		totalPages = 1
	}

	if f.page < 0 {
		f.page = 0
	}

	var options []huh.Option[string]

	if f.dir != "/" {
		options = append(options, huh.NewOption("../", filepath.Dir(f.dir)))
	}

	filterText := "Filter files"
	if f.filter != "" {
		filterText = fmt.Sprintf("Filter: '%s'", f.filter)
	}

	options = append(options, huh.NewOption(filterText, "filter"))

	if totalPages > 1 {
		pageInfo := fmt.Sprintf("Page %d of %d (%d items)", f.page+1, totalPages, totalItems)
		options = append(options, huh.NewOption(pageStyle.Render(pageInfo), "page_info"))

		if f.page > 0 {
			options = append(options, huh.NewOption("<-", "prev_page"))
		}
		if f.page < totalPages-1 {
			options = append(options, huh.NewOption("->", "next_page"))
		}
	}

	start := f.page * PAGESIZE
	end := min(start+PAGESIZE, len(entries))

	for i := start; i < end; i++ {
		entry := entries[i]
		path := filepath.Join(f.dir, entry.Name())
		name := entry.Name()

		if entry.IsDir() {
			name = dirStyle.Render(name + "/")
		}

		if _, ok := f.Selected[path]; ok {
			name = selectedStyle.Render("✓ " + name)
		}

		options = append(options, huh.NewOption(name, path))
	}

	options = append(options,
		huh.NewOption("Done", "done"),
		huh.NewOption("Cancel", "cancel"),
	)

	title := fmt.Sprintf("Choose files (%d selected):", len(f.Selected))
	if f.filter != "" {
		title += fmt.Sprintf(" [Filter: %s]", f.filter)
	}

	form := huh.NewSelect[string]().
		Title(title).
		Options(options...).
		Value(&f.selected).
		Height(20)

	err = form.Run()
	if err != nil {
		return err
	}

	switch f.selected {
	case "cancel":
		return ErrCanceled
	case "done":
		return nil
	case "filter":
		err := f.Filter()
		if err != nil {
			return err
		}
		return f.RunRecur()
	case "prev_page":
		f.page--
		return f.RunRecur()
	case "next_page":
		f.page++
		return f.RunRecur()
	case "page_info":
		return f.RunRecur()
	default:
		err := f.Selection()
		if err != nil {
			return err
		}
		return f.RunRecur()
	}
}

func (f *FileSelector) Filter() error {
	var newFilter string

	form := huh.NewInput().
		Title("Filter:").
		Value(&newFilter).
		Placeholder(f.filter)

	err := form.Run()
	if err != nil {
		return err
	}

	f.filter = strings.TrimSpace(newFilter)
	f.page = 0
	return err
}

func (f *FileSelector) Selection() error {
	stat, err := os.Stat(f.selected)
	if err != nil {
		return f.RunRecur()
	}

	if stat.IsDir() {
		var action string
		form := huh.NewSelect[string]().
			Title(fmt.Sprintf("Directory: %s", filepath.Base(f.selected))).
			Options(
				huh.NewOption("Navigate", "navigate"),
				huh.NewOption("Select all", "select_all"),
				huh.NewOption("Back", "back"),
			).
			Value(&action)

		err := form.Run()
		if err != nil {
			return f.RunRecur()
		}

		switch action {
		case "navigate":
			f.dir = f.selected
			f.page = 0
		case "select_all":
			return f.SelectDir(f.selected)
		}
	} else {
		f.Select(f.selected, f.dir, stat)
	}

	return nil
}

func (f *FileSelector) Select(fullPath, path string, stat os.FileInfo) {
	if path == f.dir {
		path = "."
	}

	if _, ok := f.Selected[fullPath]; ok {
		f.nBytesSelected -= stat.Size()
		delete(f.Selected, fullPath)
	} else {
		path = strings.TrimPrefix(path, f.dir)
		path = strings.TrimPrefix(path, string(filepath.Separator))
		f.nBytesSelected += stat.Size()
		f.Selected[fullPath] = &FileHeader{name: stat.Name(), size: stat.Size(), abspath: fullPath, path: path}
	}
}

func (f *FileSelector) SelectDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			err := f.SelectDir(path)
			if err != nil {
				continue
			}
		} else {
			stat, err := os.Stat(path)
			if err != nil {
				continue
			}
			f.Select(path, dir, stat)
		}
	}
	return nil
}

func (f *FileSelector) GetSelectedPaths() []string {
	paths := make([]string, 0, len(f.Selected))
	for path := range f.Selected {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func (f *FileSelector) ClearSelection() {
	f.Selected = make(map[string]*FileHeader)
}
