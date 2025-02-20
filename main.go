package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Styles struct {
	BorderColor lipgloss.Color
	InputField  lipgloss.Style
	BubbleTable lipgloss.Style
}

func DefaultStyles() *Styles {
	s := new(Styles)
	s.BorderColor = lipgloss.Color("36")
	s.InputField = lipgloss.NewStyle().BorderForeground(s.BorderColor).BorderStyle(lipgloss.NormalBorder()).Padding(1).Width(100)
	s.BubbleTable = lipgloss.NewStyle().BorderForeground(s.BorderColor).BorderStyle(lipgloss.NormalBorder()).Padding(1)
	return s
}

type model struct {
	index     int
	width     int
	heigth    int
	done      bool
	questions []Question
	styles    *Styles
	table     table.Model
}

type Question struct {
	question string
	answer   string
	input    Input
}

func NewQuestion(question string) Question {
	return Question{question: question}
}

func newShortQuestion(question string) Question {
	q := NewQuestion(question)
	field := NewShortAnswerField()
	q.input = field
	return q
}

func newLongQuestion(question string) Question {
	q := NewQuestion(question)
	field := NewLongAnswerField()
	q.input = field
	return q
}

func New(questions []Question) *model {
	styles := DefaultStyles()

	return &model{
		questions: questions,
		styles:    styles,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	current := &m.questions[m.index]

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.heigth = msg.Height
		if m.width < 80 || m.heigth < 20 {
			return m, nil
		}

		if m.done {
			m.initTable()
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "enter":
			current.answer = current.input.Value()
			m.Next()
			return m, current.input.Blur

		case "esc":
			if m.index >= 1 {
				m.index = m.index - 1
			}
			return m, nil
		}
	}

	if m.done {
		m.table, cmd = m.table.Update(msg)
	} else {
		current.input, cmd = current.input.Update(msg)
	}

	return m, cmd
}

func (m model) View() string {
	if m.width < 80 || m.heigth < 20 {
		return "Please increase the terminmal size to at least 80x20."
	}

	if m.done {
		var output string

		file, err := os.Create("output.txt")
		if err != nil {
			log.Fatal("Error creating file:", err)
		}
		defer file.Close()

		for _, q := range m.questions {
			file.WriteString(fmt.Sprintf("%s: %s\n", q.question, q.answer))
			output += fmt.Sprintf("%s: %s\n", q.question, q.answer)
		}

		return lipgloss.Place(
			m.width,
			m.heigth,
			lipgloss.Center,
			lipgloss.Center,
			lipgloss.JoinVertical(
				lipgloss.Center,
				"Answers:",
				m.styles.BubbleTable.Render(m.table.View()),
				lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("ctrl+c: exit | files saved to output.txt!"),
			),
		)
	}
	current := m.questions[m.index]
	caption := fmt.Sprintf(
		"ctrl+c: exit | esc: back | enter: next | question: %d/%d",
		m.index+1, len(m.questions),
	)
	return lipgloss.Place(
		m.width,
		m.heigth,
		lipgloss.Center,
		lipgloss.Center,
		lipgloss.JoinVertical(
			lipgloss.Center,
			current.question,
			m.styles.InputField.Render(current.input.View()),
			lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(caption),
		),
	)
}

func (m *model) initTable() {
	padding := 6
	questionColWidth := (m.width - padding) / 3
	answerColWidth := ((m.width-padding)/3)*2 - padding

	columns := []table.Column{
		{Title: "Question", Width: questionColWidth},
		{Title: "Answer", Width: answerColWidth},
	}
	var rows []table.Row
	for _, q := range m.questions {
		rows = append(rows, table.Row{q.question, q.answer})
	}
	m.table = table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(m.heigth-padding),
	)
}

func isGoRun() bool {
	exePath, err := os.Executable()
	if err != nil {
		return false
	}
	return filepath.Base(exePath) == "main" || filepath.Dir(exePath) == os.TempDir()
}

func isTerminal() bool {
	if isGoRun() {
		return true
	}
	return os.Stdout.Fd() != os.Stderr.Fd()
}

func openInTerminal() error {
	if isGoRun() {
		return nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("unable to get executable path: %v", err)
	}

	if runtime.GOOS == "windows" {
		cmd := exec.Command("cmd.exe", "/c", "start", "cmd", "/k", exePath)
		return cmd.Start()
	}

	if runtime.GOOS == "linux" {
		cmd := exec.Command("gnome-terminal", "--", exePath)
		return cmd.Start()
	}

	return fmt.Errorf("unsupported OS")
}

func ReadLines(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	return lines, scanner.Err()
}

func (m *model) Next() {
	if m.index < len(m.questions)-1 {
		m.index++
	} else {
		m.done = true
		m.initTable()
	}
}

func ListFiles(directory string, recursive bool) ([]string, error) {
	var files []string

	walkFunc := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		if !recursive && directory != path && info.IsDir() {
			return filepath.SkipDir
		}
		return nil
	}

	err := filepath.Walk(directory, walkFunc)
	return files, err
}

func main() {
	if !isTerminal() {
		err := openInTerminal()
		if err != nil {
			fmt.Println("Error opening terminal:", err)
			return
		}
		return
	}

	var recursive bool
	var listDir string
	var questionsPath string

	flag.StringVar(&listDir, "l", "", "Directory to list files from")
	flag.StringVar(&listDir, "list", "", "Directory to list files from")
	flag.StringVar(&questionsPath, "q", "", "Path to questions.txt")
	flag.StringVar(&questionsPath, "questions", "", "Path to questions.txt")
	flag.BoolVar(&recursive, "r", false, "Enable recursive listing")

	flag.Parse()
	args := flag.Args()

	if len(args) > 0 {
		listDir = args[0]
	}

	if len(args) > 1 {
		questionsPath = args[1]
	}

	if listDir == "" {
		listDir = "."
	}

	if questionsPath == "" {
		questionsPath = filepath.Join(listDir, "questions.txt")
	}

	if _, err := os.Stat(questionsPath); os.IsNotExist(err) {
		file, err := os.Create(questionsPath)
		if err != nil {
			log.Fatal("Error creating file:", err)
		}
		defer file.Close()
		fmt.Println("File 'questions.txt' created successfully at", questionsPath)

		contents := `
Write your questions in questions.txt, okay?
// * in the start of a question, is for a long answer
// > in the start of a question, is for file listing questions
Take a look in the --help command!
		`

		if _, err := file.WriteString(contents); err != nil {
			log.Fatal("Error writing to file:", err)
		}
	}

	files, err := ListFiles(listDir, recursive)
	if err != nil {
		log.Fatal("Error reading directory:", err)
	}

	rawQuestions, err := ReadLines("questions.txt")
	if err != nil {
		log.Fatal(err)
	}

	var questions []Question
	for _, line := range rawQuestions {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		if line[0] == '/' && line[1] == '/' {
			//does nothing, it is a comment!
		} else if line[0] == '*' {
			questions = append(questions, newLongQuestion(line[1:]))
		} else if line[0] == '>' {
			for _, file := range files {
				questionText := fmt.Sprintf("%s %s?", line[1:], filepath.Base(file))
				questions = append(questions, newShortQuestion(questionText))
			}
		} else {
			questions = append(questions, newShortQuestion(line))
		}
	}

	m := New(questions)
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
