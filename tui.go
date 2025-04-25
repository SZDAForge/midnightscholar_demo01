package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// tuiModel - A TUI felület állapotát tároló struktúra
type tuiModel struct {
	choices      []string
	cursor       int // A teljes lista pozíciója
	selected     map[int]struct{}
	textInput    textinput.Model
	viewport     viewport.Model
	help         help.Model
	keys         tuiKeyMap
	state        int
	frameRate    float64
	outputDir    string
	resultsFile  string
	videoFile    string
	analysisDone bool
	errorMsg     string
	timeInterval float64
	editingParam bool // Paraméter szerkesztés állapota
}

// tuiKeyMap - Billentyűparancsok definíciói
type tuiKeyMap struct {
	Up    key.Binding
	Down  key.Binding
	Space key.Binding
	Enter key.Binding
	Esc   key.Binding
	Tab   key.Binding
	Save  key.Binding
	Quit  key.Binding
}

// Implement help.KeyMap interface
func (k tuiKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Enter, k.Quit}
}

func (k tuiKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Space, k.Enter},
		{k.Esc, k.Tab, k.Save, k.Quit},
	}
}

var (
	tuiTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF06B7")).
			Bold(true).
			Padding(1, 2)

	tuiSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#04B575")).
				Bold(true)

	tuiNormalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))
)

// initialTuiModel - Kezdeti TUI modell létrehozása
func initialTuiModel() tuiModel {
	ti := textinput.New()
	ti.Placeholder = "Adja meg az értéket..."
	ti.CharLimit = 256
	ti.Width = 50

	// Program válaszablak inicializálása
	vp := viewport.New(80, 10)
	vp.Style = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#874BFD")).
		Padding(1, 2)

	return tuiModel{
		choices: []string{
			"Kontraszt Elemzés",
			"Fényerősség Elemzés",
			"Mozgás Érzékelés",
			"Képkivágás Típus Felismerés",
		},
		selected:     make(map[int]struct{}),
		textInput:    ti,
		viewport:     vp,
		help:         help.New(),
		keys:         newTuiKeyMap(),
		state:        0,
		frameRate:    1.0,
		outputDir:    "frames_output",
		resultsFile:  "analysis_results.txt",
		analysisDone: false,
		errorMsg:     "",
		timeInterval: 1.0,
		editingParam: false,
	}
}

// newTuiKeyMap - Billentyűparancsok definíciója
func newTuiKeyMap() tuiKeyMap {
	return tuiKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "fel"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "le"),
		),
		Space: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "kiválasztás"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "tovább/mentés"),
		),
		Esc: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "vissza"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "szerkesztés"),
		),
		Save: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "mentés"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c", "q"),
			key.WithHelp("ctrl+c/q", "kilépés"),
		),
	}
}

func (m tuiModel) Init() tea.Cmd {
	return nil
}

// Update - A TUI állapotának frissítése
func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Ha paraméter szerkesztés módban vagyunk
		if m.editingParam && m.state == 2 {
			switch {
			case key.Matches(msg, m.keys.Enter):
				// Paraméter értékének mentése
				paramIdx := m.cursor - len(m.choices)
				value := m.textInput.Value()
				switch paramIdx {
				case 0: // Képkocka sebesség
					if val, err := strconv.ParseFloat(value, 64); err == nil && val >= 0.05 && val <= 2.00 {
						m.frameRate = val
						m.errorMsg = ""
					} else {
						m.errorMsg = "⚠️ Érvénytelen képkocka sebesség (0.05-2.00)!"
						return m, nil
					}
				case 1: // Időintervallum
					if val, err := strconv.ParseFloat(value, 64); err == nil && val > 0 {
						m.timeInterval = val
						m.errorMsg = ""
					} else {
						m.errorMsg = "⚠️ Érvénytelen időintervallum!"
						return m, nil
					}
				case 2: // Kimeneti mappa
					m.outputDir = value
				case 3: // Eredmény fájl
					m.resultsFile = value
				}
				m.textInput.Reset()
				m.editingParam = false
				return m, nil
			case key.Matches(msg, m.keys.Esc):
				// Kilépés szerkesztés módból
				m.textInput.Reset()
				m.editingParam = false
				return m, nil
			default:
				m.textInput, cmd = m.textInput.Update(msg)
				return m, cmd
			}
		}

		// Normál mód kezelése
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keys.Esc):
			if m.state > 0 {
				m.state = 0
				m.errorMsg = ""
				m.viewport.SetContent("")
				m.textInput.Reset()
				m.editingParam = false
				m.cursor = 0                        // Reset cursor position
				m.selected = make(map[int]struct{}) // Reset selections
				return m, tea.ClearScreen           // Clear the entire screen
			}

		case key.Matches(msg, m.keys.Up):
			if m.state == 2 {
				if m.cursor > 0 {
					m.cursor--
				}
			} else if m.cursor > 0 {
				m.cursor--
			}

		case key.Matches(msg, m.keys.Down):
			totalItems := len(m.choices) + 4 // 3 elemzési módszer + 4 paraméter
			if m.state == 2 {
				if m.cursor < totalItems-1 {
					m.cursor++
				}
			} else if m.cursor < len(m.choices)-1 {
				m.cursor++
			}

		case key.Matches(msg, m.keys.Space):
			if m.state == 2 && m.cursor < len(m.choices) {
				_, ok := m.selected[m.cursor]
				if ok {
					delete(m.selected, m.cursor)
				} else {
					m.selected[m.cursor] = struct{}{}
				}
			}

		case key.Matches(msg, m.keys.Enter):
			if m.state == 2 && m.cursor >= len(m.choices) {
				// Paraméter szerkesztés módba lépés
				m.editingParam = true
				m.textInput.Focus()
				// Aktuális érték beállítása
				paramIdx := m.cursor - len(m.choices)
				switch paramIdx {
				case 0:
					m.textInput.SetValue(fmt.Sprintf("%.2f", m.frameRate))
				case 1:
					m.textInput.SetValue(fmt.Sprintf("%.2f", m.timeInterval))
				case 2:
					m.textInput.SetValue(m.outputDir)
				case 3:
					m.textInput.SetValue(m.resultsFile)
				}
				return m, nil
			}

			switch m.state {
			case 0: // Főmenü
				switch m.cursor {
				case 0: // Videó Fájl Kiválasztása
					m.state = 1
					if m.videoFile != "" {
						m.textInput.SetValue(m.videoFile)
					}
					m.textInput.Focus()
				case 1: // Elemzés Beállítása
					m.state = 2
				case 2: // Elemzés Indítása
					if m.videoFile == "" {
						m.errorMsg = "⚠️ Először válasszon videó fájlt!"
						return m, nil
					}
					m.state = 3
				case 3: // Eredmények Megtekintése
					if !m.analysisDone {
						m.errorMsg = "⚠️ Először futtasson elemzést!"
						return m, nil
					}
					m.state = 5
				}

			case 1:
				m.videoFile = m.textInput.Value()
				if _, err := os.Stat(m.videoFile); err == nil {
					m.state = 0
					m.errorMsg = ""
				} else {
					m.errorMsg = fmt.Sprintf("⚠️ A fájl nem található: %s", m.videoFile)
				}

			case 2: // Elemzési módszerek
				if m.cursor < len(m.choices) {
					m.cursor = len(m.choices)
				} else {
					m.cursor = 0
				}

			case 3: // Paraméterek
				m.state = 4
				// Elemzés konfigurálása és indítása
				config := AnalysisConfig{
					FrameRate:        m.frameRate,
					TimeInterval:     m.timeInterval,
					OutputDir:        m.outputDir,
					ResultsFile:      m.resultsFile,
					AnalyzeContrast:  isSelected(m.selected, 0),
					AnalyzeLuminance: isSelected(m.selected, 1),
					AnalyzeMotion:    isSelected(m.selected, 2),
					AnalyzeShotType:  isSelected(m.selected, 3),
				}
				if err := analyzeVideo(m.videoFile, config); err != nil {
					m.viewport.SetContent(fmt.Sprintf("Hiba: %v", err))
				} else {
					m.viewport.SetContent("Az elemzés sikeresen befejeződött!")
				}
				m.analysisDone = true
			}

		case key.Matches(msg, m.keys.Tab):
			if m.state == 2 {
				if m.cursor < len(m.choices) {
					m.cursor = len(m.choices)
				} else {
					m.cursor = 0
				}
			}
		}

		if m.state == 1 {
			m.textInput, cmd = m.textInput.Update(msg)
		}
	}

	return m, cmd
}

func isSelected(selected map[int]struct{}, index int) bool {
	_, ok := selected[index]
	return ok
}

// getContextHelp - Az aktuális állapotnak megfelelő billentyűparancsok
func (m tuiModel) getContextHelp() string {
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
		Padding(1)

	var commands []string
	switch m.state {
	case 0: // Főmenü
		commands = []string{
			"↑/↓: Navigálás",
			"Enter: Kiválasztás",
			"q: Kilépés",
		}
	case 1: // Fájl kiválasztás
		commands = []string{
			"Enter: Mentés",
			"Esc: Vissza",
			"q: Kilépés",
		}
	case 2: // Elemzési módszerek
		commands = []string{
			"↑/↓: Navigálás",
			"Space: Ki/bekapcsolás",
			"Enter: Tovább",
			"Esc: Vissza",
			"q: Kilépés",
		}
	case 3: // Paraméterek
		commands = []string{
			"Enter: Érték mentése",
			"Esc: Vissza",
			"q: Kilépés",
		}
	case 4, 5: // Elemzés és eredmények
		commands = []string{
			"Esc: Vissza a főmenübe",
			"q: Kilépés",
		}
	}

	return helpStyle.Render("Billentyűparancsok: " + strings.Join(commands, " | "))
}

// View - A TUI megjelenítése
func (m tuiModel) View() string {
	header := tuiTitleStyle.Render("🎬 FilmVision Elemző") + "\n\n"

	errorSection := ""
	if m.errorMsg != "" {
		errorSection = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Render(m.errorMsg) + "\n\n"
	}

	content := ""
	switch m.state {
	case 0:
		content += "Válasszon egy opciót:\n\n"
		for i, choice := range []string{"Videó Fájl Kiválasztása", "Elemzés Beállítása", "Elemzés Indítása", "Eredmények Megtekintése"} {
			style := tuiNormalStyle
			cursor := "  "
			if m.cursor == i {
				style = tuiSelectedStyle
				cursor = "> "
			}
			content += fmt.Sprintf("%s%s\n", cursor, style.Render(choice))
		}

	case 1:
		content += "Adja meg a videó fájl útvonalát:\n\n"
		content += m.textInput.View() + "\n\n"

	case 2:
		content += "Elemzési beállítások:\n\n"

		// Define parameters
		params := []struct {
			name  string
			value string
		}{
			{"Képkocka sebesség (0.05-2.00)", fmt.Sprintf("%.2f", m.frameRate)},
			{"Időintervallum (másodperc)", fmt.Sprintf("%.2f", m.timeInterval)},
			{"Kimeneti mappa", m.outputDir},
			{"Eredmény fájl", m.resultsFile},
		}

		// Elemzési módszerek section
		content += "Elemzési módszerek:\n"
		for i, choice := range m.choices {
			style := tuiNormalStyle
			cursor := "  "
			if m.cursor == i {
				style = tuiSelectedStyle
				cursor = "> "
			}
			checked := " "
			if _, ok := m.selected[i]; ok {
				checked = "x"
			}
			content += fmt.Sprintf("%s[%s] %s\n", cursor, checked, style.Render(choice))
		}

		content += "\nParaméterek:\n"
		for i, param := range params {
			style := tuiNormalStyle
			cursor := "  "
			if m.cursor == len(m.choices)+i {
				style = tuiSelectedStyle
				cursor = "> "
			}
			content += fmt.Sprintf("%s%s: %s\n", cursor, style.Render(param.name), style.Render(param.value))
		}

		content += "\nTab: Váltás a módszerek és paraméterek között"
		if m.editingParam {
			content += "\nEnter: Érték mentése | Esc: Mégse"
			content += "\n\nÚj érték: " + m.textInput.View()
		} else {
			content += "\nEnter: Érték módosítása"
		}

	case 4:
		content += "Elemzés folyamatban...\n\n"

	case 5:
		content += "Elemzési eredmények:\n\n"
	}

	response := m.viewport.View()
	help := m.getContextHelp()

	return header + errorSection + content + "\n" + response + "\n" + help
}

// runTUI - A TUI alkalmazás indítása
func runTUI() {
	p := tea.NewProgram(initialTuiModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Hiba a TUI futtatása közben: %v\n", err)
		os.Exit(1)
	}
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
