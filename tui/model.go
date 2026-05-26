package tui

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/init0-lux/vcf-toolkit/internal/convert"
	"github.com/init0-lux/vcf-toolkit/internal/dedupe"
	"github.com/init0-lux/vcf-toolkit/internal/model"
	"github.com/init0-lux/vcf-toolkit/internal/normalize"
)

type workflow int

const (
	flowNormalizeCSV workflow = iota
	flowNormalizeJSON
	flowNormalizeVCF
	flowNormalizeDedupeCSV
	flowNormalizeDedupeVCF
	flowConvertVCF
	flowConvertDedupeVCF
)

type keys struct {
	Quit    key.Binding
	Back    key.Binding
	Next    key.Binding
	Verbose key.Binding
	LLM     key.Binding
	Save    key.Binding
	Browse  key.Binding
}

func (k keys) ShortHelp() []key.Binding {
	return []key.Binding{k.Back, k.Next, k.Verbose, k.LLM, k.Save, k.Browse, k.Quit}
}
func (k keys) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Back, k.Next, k.Verbose, k.LLM, k.Save, k.Browse, k.Quit}}
}

type screen int

const (
	screenMenu screen = iota
	screenInput
	screenOutput
	screenOptions
	screenRun
	screenDone
	screenBrowse
)

type menuItem struct {
	title string
	desc  string
	flow  workflow
}

func (m menuItem) Title() string       { return m.title }
func (m menuItem) Description() string { return m.desc }
func (m menuItem) FilterValue() string { return m.title }

type tuiModel struct {
	errOut io.Writer

	width  int
	height int

	keys keys
	help help.Model

	screen screen
	menu   list.Model
	browse list.Model

	browseDir    string
	browseTarget browseTarget

	inputPath  textinput.Model
	outputPath textinput.Model

	verbose bool
	useLLM  bool

	llmURL  textinput.Model
	llmAuth textinput.Model
	llmKey  textinput.Model
	llmBody textinput.Model

	status  string
	lastErr error
}

func newModel(errOut io.Writer) tuiModel {
	items := []list.Item{
		menuItem{"Normalize -> CSV", "Read CSV, normalize name/phone/email, write CSV", flowNormalizeCSV},
		menuItem{"Normalize -> JSON", "Read CSV, normalize name/phone/email, write JSON", flowNormalizeJSON},
		menuItem{"Normalize -> VCF", "Read CSV, normalize, write VCF (vCard 4.0)", flowNormalizeVCF},
		menuItem{"Normalize + Dedupe -> CSV", "Normalize + merge duplicates, write CSV", flowNormalizeDedupeCSV},
		menuItem{"Normalize + Dedupe -> VCF", "Normalize + merge duplicates, write VCF", flowNormalizeDedupeVCF},
		menuItem{"Convert CSV -> VCF", "Convert CSV to VCF (auto header mapping)", flowConvertVCF},
		menuItem{"Convert + Dedupe -> VCF", "Convert and dedupe during conversion", flowConvertDedupeVCF},
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.SetShowTitle(false)
	l.DisableQuitKeybindings()
	l.SetFilteringEnabled(true)
	l.Styles.FilterPrompt = lipgloss.NewStyle().Foreground(colorAccent)
	l.Styles.FilterCursor = lipgloss.NewStyle().Foreground(colorAccent)

	in := textinput.New()
	in.Placeholder = "path/to/input.csv"
	in.Prompt = "Input CSV: "
	in.CharLimit = 4096
	in.Width = 70

	out := textinput.New()
	out.Placeholder = "stdout (empty) or path/to/output"
	out.Prompt = "Output:   "
	out.CharLimit = 4096
	out.Width = 70

	u := textinput.New()
	u.Placeholder = "https://... (endpoint returning JSON {full_name,...})"
	u.Prompt = "LLM URL:  "
	u.CharLimit = 4096
	u.Width = 70

	a := textinput.New()
	a.Placeholder = "Header name (e.g. Authorization)"
	a.Prompt = "Auth hdr: "
	a.CharLimit = 1024
	a.Width = 70

	k := textinput.New()
	k.Placeholder = "Token (leave empty to use .env/env in future)"
	k.Prompt = "API key:  "
	k.CharLimit = 4096
	k.Width = 70
	k.EchoMode = textinput.EchoPassword

	bt := textinput.New()
	bt.Placeholder = "{\"input\": {input}}  (use {input} placeholder)"
	bt.Prompt = "Body:     "
	bt.CharLimit = 4096
	bt.Width = 70

	m := tuiModel{
		errOut: errOut,
		keys: keys{
			Quit:    key.NewBinding(key.WithKeys("q", "esc", "ctrl+c"), key.WithHelp("q", "quit")),
			Back:    key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("left", "back")),
			Next:    key.NewBinding(key.WithKeys("enter", "right", "l"), key.WithHelp("enter", "next")),
			Verbose: key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "verbose")),
			LLM:     key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "llm")),
			Save:    key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "save .env")),
			Browse:  key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "browse")),
		},
		help:       help.New(),
		screen:     screenMenu,
		menu:       l,
		browse:     newBrowseList(),
		inputPath:  in,
		outputPath: out,
		llmURL:     u,
		llmAuth:    a,
		llmKey:     k,
		llmBody:    bt,
	}

	if cwd, err := os.Getwd(); err == nil {
		m.browseDir = cwd
	}

	// Best-effort preload from .env if present.
	if c, err := loadDotEnv(".env"); err == nil {
		if c.LLMURL != "" {
			m.llmURL.SetValue(c.LLMURL)
		}
		if c.LLMAuthHeader != "" {
			m.llmAuth.SetValue(c.LLMAuthHeader)
		}
		if c.LLMAPIKey != "" {
			m.llmKey.SetValue(c.LLMAPIKey)
		}
		if c.LLMBodyTmpl != "" {
			m.llmBody.SetValue(c.LLMBodyTmpl)
		}
	}
	return m
}

func (m tuiModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.menu.StartSpinner())
}

type runDoneMsg struct{ err error }
type browseLoadedMsg struct {
	dir   string
	items []list.Item
	err   error
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case runDoneMsg:
		m.lastErr = msg.err
		m.status = "Done."
		m.screen = screenDone
		return m, nil
	case browseLoadedMsg:
		if msg.err != nil {
			m.lastErr = msg.err
			return m, nil
		}
		m.browseDir = msg.dir
		m.browse.SetItems(msg.items)
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.menu.SetSize(msg.Width-4, msg.Height-10)
		m.browse.SetSize(msg.Width-4, msg.Height-12)
		return m, nil

	case tea.KeyMsg:
		if key.Matches(msg, m.keys.Quit) {
			return m, tea.Quit
		}
		if key.Matches(msg, m.keys.Verbose) {
			m.verbose = !m.verbose
			return m, nil
		}
		if key.Matches(msg, m.keys.LLM) && m.screen == screenOptions {
			m.useLLM = !m.useLLM
			return m, m.focusCmd()
		}
		if key.Matches(msg, m.keys.Save) && m.screen == screenOptions {
			m.lastErr = writeDotEnv(".env", envConfig{
				LLMURL:        strings.TrimSpace(m.llmURL.Value()),
				LLMAuthHeader: strings.TrimSpace(m.llmAuth.Value()),
				LLMAPIKey:     strings.TrimSpace(m.llmKey.Value()),
				LLMBodyTmpl:   strings.TrimSpace(m.llmBody.Value()),
			})
			return m, nil
		}
		if key.Matches(msg, m.keys.Browse) && (m.screen == screenInput || m.screen == screenOutput) {
			if m.screen == screenInput {
				m.browseTarget = browseInput
			} else {
				m.browseTarget = browseOutput
			}
			m.screen = screenBrowse
			return m, m.loadBrowseDirCmd(m.browseDir)
		}
		if key.Matches(msg, m.keys.Back) {
			if m.screen > screenMenu {
				m.screen--
				m.lastErr = nil
				return m, m.focusCmd()
			}
			return m, nil
		}
		if key.Matches(msg, m.keys.Next) {
			switch m.screen {
			case screenMenu:
				m.screen = screenInput
				return m, m.focusCmd()
			case screenInput:
				m.screen = screenOutput
				return m, m.focusCmd()
			case screenOutput:
				m.screen = screenOptions
				return m, m.focusCmd()
			case screenOptions:
				m.screen = screenRun
				m.status = "Running..."
				m.lastErr = nil
				return m, tea.Batch(m.focusCmd(), m.runWorkflowCmd())
			case screenDone:
				m.screen = screenMenu
				return m, m.focusCmd()
			}
		}
	}

	var cmd tea.Cmd
	switch m.screen {
	case screenMenu:
		m.menu, cmd = m.menu.Update(msg)
	case screenInput:
		m.inputPath, cmd = m.inputPath.Update(msg)
	case screenOutput:
		m.outputPath, cmd = m.outputPath.Update(msg)
	case screenOptions:
		if m.useLLM {
			if m.llmURL.Focused() {
				m.llmURL, cmd = m.llmURL.Update(msg)
			} else if m.llmAuth.Focused() {
				m.llmAuth, cmd = m.llmAuth.Update(msg)
			} else if m.llmKey.Focused() {
				m.llmKey, cmd = m.llmKey.Update(msg)
			} else if m.llmBody.Focused() {
				m.llmBody, cmd = m.llmBody.Update(msg)
			} else {
				m.llmURL.Focus()
				cmd = textinput.Blink
			}
		}
	case screenBrowse:
		m.browse, cmd = m.browse.Update(msg)
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.String() {
			case "esc":
				if m.browseTarget == browseInput {
					m.screen = screenInput
				} else {
					m.screen = screenOutput
				}
				m.lastErr = nil
				return m, m.focusCmd()
			case "backspace":
				parent := filepath.Dir(m.browseDir)
				if parent != "" && parent != m.browseDir {
					return m, m.loadBrowseDirCmd(parent)
				}
			case "enter":
				if it, ok := m.browse.SelectedItem().(browseItem); ok {
					if it.dir {
						return m, m.loadBrowseDirCmd(it.path)
					}
					if m.browseTarget == browseInput {
						m.inputPath.SetValue(it.path)
						m.screen = screenInput
					} else {
						m.outputPath.SetValue(it.path)
						m.screen = screenOutput
					}
					m.lastErr = nil
					return m, m.focusCmd()
				}
			}
		}
	}
	return m, cmd
}

func (m tuiModel) View() string {
	header := banner(m.width)
	switch m.screen {
	case screenMenu:
		body := lipgloss.NewStyle().Padding(1, 2).Render(m.menu.View())
		return lipgloss.JoinVertical(lipgloss.Left, header, body, m.footer())
	case screenInput:
		return lipgloss.JoinVertical(lipgloss.Left, header, m.card("Choose input CSV", m.inputPath.View()), m.footer())
	case screenOutput:
		return lipgloss.JoinVertical(lipgloss.Left, header, m.card("Choose output destination", m.outputPath.View()), m.footer())
	case screenOptions:
		return lipgloss.JoinVertical(lipgloss.Left, header, m.optionsView(), m.footer())
	case screenRun:
		return lipgloss.JoinVertical(lipgloss.Left, header, m.card("Running", m.runStatus()), m.footer())
	case screenDone:
		return lipgloss.JoinVertical(lipgloss.Left, header, m.card("Result", m.doneStatus()), m.footer())
	case screenBrowse:
		body := lipgloss.NewStyle().Padding(1, 2).Render(browseHeader(m.browseDir, m.browseTarget) + "\n\n" + m.browse.View())
		return lipgloss.JoinVertical(lipgloss.Left, header, body, m.footer())
	default:
		return header
	}
}

func (m tuiModel) footer() string {
	return lipgloss.NewStyle().Padding(0, 2).Foreground(colorDim).Render(m.help.View(m.keys))
}

func (m tuiModel) card(title, body string) string {
	card := lipgloss.NewStyle().
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorAccent)
	return card.Render(lipgloss.NewStyle().Bold(true).Render(title) + "\n\n" + body)
}

func (m tuiModel) optionsView() string {
	lines := []string{
		fmt.Sprintf("Verbose: %v (press v)", m.verbose),
		fmt.Sprintf("LLM name normalization: %v (press l)", m.useLLM),
		"Save config: press s (writes .env, chmod 0600)",
	}
	body := strings.Join(lines, "\n")
	if m.useLLM {
		body += "\n\n" + m.llmURL.View() + "\n" + m.llmAuth.View() + "\n" + m.llmKey.View() + "\n" + m.llmBody.View()
		body += "\n" + lipgloss.NewStyle().Foreground(colorDim).Render("Response JSON: {\"full_name\":\"...\",\"first_name\":\"...\",\"last_name\":\"...\",\"organization\":\"...\"}")
		body += "\n" + lipgloss.NewStyle().Foreground(colorDim).Render("Body template: JSON containing {input} placeholder (replaced with a JSON string).")
	}
	return m.card("Options", body)
}

func (m tuiModel) runStatus() string {
	if m.lastErr != nil {
		return warn(m.lastErr)
	}
	return m.status
}

func (m tuiModel) doneStatus() string {
	if m.lastErr != nil {
		return warn(m.lastErr) + "\n\nPress Enter to return to menu."
	}
	return "Done.\n\nPress Enter to return to menu."
}

func (m tuiModel) focusCmd() tea.Cmd {
	m.inputPath.Blur()
	m.outputPath.Blur()
	m.llmURL.Blur()
	m.llmAuth.Blur()
	m.llmKey.Blur()
	m.llmBody.Blur()

	switch m.screen {
	case screenInput:
		m.inputPath.Focus()
		return textinput.Blink
	case screenOutput:
		m.outputPath.Focus()
		return textinput.Blink
	case screenOptions:
		if m.useLLM {
			m.llmURL.Focus()
			return textinput.Blink
		}
	case screenBrowse:
		return nil
	}
	return nil
}

func (m tuiModel) loadBrowseDirCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		items, err := readDirItems(dir)
		return browseLoadedMsg{dir: dir, items: items, err: err}
	}
}

func (m tuiModel) selectedFlow() workflow {
	if it, ok := m.menu.SelectedItem().(menuItem); ok {
		return it.flow
	}
	return flowNormalizeCSV
}

func (m tuiModel) runWorkflowCmd() tea.Cmd {
	flow := m.selectedFlow()
	in := strings.TrimSpace(m.inputPath.Value())
	out := strings.TrimSpace(m.outputPath.Value())
	verbose := m.verbose
	useLLM := m.useLLM
	llmURL := strings.TrimSpace(m.llmURL.Value())
	llmAuth := strings.TrimSpace(m.llmAuth.Value())
	llmKey := strings.TrimSpace(m.llmKey.Value())
	llmBody := strings.TrimSpace(m.llmBody.Value())

	return func() tea.Msg {
		if in == "" {
			return runDoneMsg{err: fmt.Errorf("input path is required")}
		}

		inFile, err := os.Open(in)
		if err != nil {
			return runDoneMsg{err: err}
		}
		defer inFile.Close()

		w, closeFn, err := openOutput(out)
		if err != nil {
			return runDoneMsg{err: err}
		}
		if closeFn != nil {
			defer closeFn()
		}

		var llm normalize.NameLLM
		if useLLM && llmURL != "" {
			headers := map[string]string{}
			if llmAuth != "" && llmKey != "" {
				headers[llmAuth] = "Bearer " + llmKey
			}
			llm = &normalize.HTTPNameLLM{
				URL:          llmURL,
				Headers:      headers,
				BodyTemplate: llmBody,
			}
		}

		switch flow {
		case flowNormalizeCSV, flowNormalizeJSON, flowNormalizeVCF, flowNormalizeDedupeCSV, flowNormalizeDedupeVCF:
			contacts, err := parseAndNormalizeRows(inFile, llm)
			if err != nil {
				return runDoneMsg{err: err}
			}

			if flow == flowNormalizeDedupeCSV || flow == flowNormalizeDedupeVCF {
				cfg := dedupe.DefaultConfig()
				result := dedupe.Deduplicate(toModelContacts(contacts), cfg)
				contacts = fromMergedContacts(result)
			}

			switch flow {
			case flowNormalizeJSON:
				return runDoneMsg{err: writeJSON(w, contacts)}
			case flowNormalizeVCF, flowNormalizeDedupeVCF:
				return runDoneMsg{err: writeVCF(w, contacts, verbose, m.errOut)}
			default:
				return runDoneMsg{err: writeCSV(w, contacts)}
			}

		case flowConvertVCF, flowConvertDedupeVCF:
			_, err := convert.ConvertCSVToVCF(inFile, w, convert.Config{
				Deduplicate: flow == flowConvertDedupeVCF,
				Verbose:     verbose,
				Stderr:      m.errOut,
			})
			return runDoneMsg{err: err}

		default:
			return runDoneMsg{err: fmt.Errorf("unknown workflow")}
		}
	}
}

func openOutput(path string) (io.Writer, func(), error) {
	if path == "" || path == "-" || strings.EqualFold(path, "stdout") {
		return os.Stdout, nil, nil
	}
	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, nil, err
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { _ = f.Close() }, nil
}

type normalizedContact struct {
	Name   string   `json:"name"`
	Phones []string `json:"phones"`
	Emails []string `json:"emails"`
	Org    string   `json:"org"`
}

func parseAndNormalizeRows(r io.Reader, llm normalize.NameLLM) ([]normalizedContact, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	rawHeaders, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("reading CSV headers: %w", err)
	}
	headers := make([]string, len(rawHeaders))
	for i, h := range rawHeaders {
		headers[i] = strings.TrimSpace(h)
	}
	mapping := buildMapping(headers)

	var contacts []normalizedContact
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		nc := normalizeRow(record, headers, mapping, llm)
		if nc.Name != "" || len(nc.Phones) > 0 || len(nc.Emails) > 0 || nc.Org != "" {
			contacts = append(contacts, nc)
		}
	}
	return contacts, nil
}

func buildMapping(headers []string) map[string]string {
	mapping := make(map[string]string, len(headers))
	for _, header := range headers {
		key := fieldKey(header)
		if key != "" {
			mapping[header] = key
		}
	}
	return mapping
}

func fieldKey(header string) string {
	s := strings.ToLower(header)
	s = strings.NewReplacer(" ", "_", "-", "_").Replace(s)
	switch {
	case matchesAny(s,
		"name", "fullname", "full_name",
		"contactname", "displayname",
		"first_name", "firstname", "given_name", "givenname",
		"last_name", "lastname", "family_name", "familyname", "surname"):
		return "name"
	case matchesAny(s,
		"phone", "telephone", "tel", "mobile", "cell",
		"phone_number", "phonenumber", "contactnumber"):
		return "phone"
	case matchesAny(s,
		"email", "e_mail", "email_address", "emailaddress", "mail"):
		return "email"
	case matchesAny(s,
		"org", "organization", "organisation", "company", "employer", "affiliation"):
		return "org"
	}
	return ""
}

func matchesAny(s string, vals ...string) bool {
	for _, v := range vals {
		if s == v {
			return true
		}
	}
	return false
}

func normalizeRow(record, headers []string, mapping map[string]string, llm normalize.NameLLM) normalizedContact {
	var nc normalizedContact
	for i, header := range headers {
		if i >= len(record) {
			break
		}
		val := strings.TrimSpace(record[i])
		if val == "" {
			continue
		}
		field, ok := mapping[header]
		if !ok {
			continue
		}
		switch field {
		case "name":
			cfg := normalize.NameConfig{}
			if llm != nil {
				cfg.UseLLM = true
				cfg.LLMClient = llm
			}
			n := normalize.NormalizeName(val, cfg)
			if n.Normalized && n.FullName != "" {
				nc.Name = n.FullName
				if nc.Org == "" && n.Organization != "" {
					nc.Org = n.Organization
				}
			} else if nc.Name == "" {
				nc.Name = val
			}
		case "phone":
			if p := normalize.NormalizePhone(val, normalize.PhoneConfig{}); p.Valid && p.Normalized != "" {
				nc.Phones = append(nc.Phones, p.Normalized)
			} else {
				nc.Phones = append(nc.Phones, val)
			}
		case "email":
			if e := normalize.NormalizeEmail(val, normalize.EmailConfig{StripAliases: true}); e.Valid && e.Normalized != "" {
				nc.Emails = append(nc.Emails, e.Normalized)
			} else {
				nc.Emails = append(nc.Emails, val)
			}
		case "org":
			if nc.Org == "" {
				nc.Org = val
			}
		}
	}
	return nc
}

func toModelContacts(in []normalizedContact) []model.Contact {
	out := make([]model.Contact, 0, len(in))
	for _, c := range in {
		out = append(out, model.Contact{
			Name:         c.Name,
			Phones:       append([]string(nil), c.Phones...),
			Emails:       append([]string(nil), c.Emails...),
			Organization: c.Org,
		})
	}
	return out
}

func fromMergedContacts(result model.DedupeResult) []normalizedContact {
	if len(result.Merged) == 0 {
		var out []normalizedContact
		for _, cl := range result.Clusters {
			if len(cl) != 1 {
				continue
			}
			c := cl[0]
			out = append(out, normalizedContact{
				Name:   c.Name,
				Phones: append([]string(nil), c.Phones...),
				Emails: append([]string(nil), c.Emails...),
				Org:    c.Organization,
			})
		}
		return out
	}

	out := make([]normalizedContact, 0, len(result.Merged))
	for _, mc := range result.Merged {
		out = append(out, normalizedContact{
			Name:   mc.Contact.Name,
			Phones: append([]string(nil), mc.Contact.Phones...),
			Emails: append([]string(nil), mc.Contact.Emails...),
			Org:    mc.Contact.Organization,
		})
	}
	return out
}

func writeJSON(w io.Writer, contacts []normalizedContact) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(contacts)
}

func writeCSV(w io.Writer, contacts []normalizedContact) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()
	if err := writer.Write([]string{"name", "phone", "email", "org"}); err != nil {
		return err
	}
	for _, c := range contacts {
		if err := writer.Write([]string{
			c.Name,
			strings.Join(c.Phones, "; "),
			strings.Join(c.Emails, "; "),
			c.Org,
		}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func writeVCF(w io.Writer, contacts []normalizedContact, verbose bool, errOut io.Writer) error {
	pr, pw := io.Pipe()

	go func() {
		cw := csv.NewWriter(pw)
		_ = cw.Write([]string{"name", "phone", "email", "org"})
		for _, c := range contacts {
			_ = cw.Write([]string{
				c.Name,
				strings.Join(c.Phones, "; "),
				strings.Join(c.Emails, "; "),
				c.Org,
			})
		}
		cw.Flush()
		_ = pw.CloseWithError(cw.Error())
	}()

	_, err := convert.ConvertCSVToVCF(pr, w, convert.Config{
		Deduplicate: false,
		Verbose:     verbose,
		Stderr:      errOut,
	})
	return err
}
