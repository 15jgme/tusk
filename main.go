package main

// export DOCKER_API_VERSION=1.41

import (
	"context"
	"fmt"
	"os"

	"github.com/15jgme/tusk/whaleFacts"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

var (
	// Available spinners
	spinners = []spinner.Spinner{
		spinner.Line,
		spinner.Dot,
		spinner.MiniDot,
		spinner.Jump,
		spinner.Pulse,
		spinner.Points,
		spinner.Globe,
		spinner.Moon,
		spinner.Monkey,
	}
	textStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Render
	spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render
)

type container struct {
	name         string
	repository   string
	tag          string
	exposesPorts bool
	ports        []uint16
	outdated     bool
	spinner      spinner.Model
	imageID      string
}

type model struct {
	cursor     int
	processing bool
	containers []container
	selected   map[int]struct{}
	fact       string
}

func initialModel() model {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err)
	}

	containers_api, err := cli.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		panic(err)
	}

	containers := []container{}

	// types.ImagePullOptions

	for _, container_api := range containers_api {
		// fmt.Printf("container_api: %+v\n", container_api)

		ports := []uint16{0, 0}
		exposesPorts := false
		if len(container_api.Ports) > 0 {
			ports = []uint16{container_api.Ports[0].PublicPort, container_api.Ports[0].PrivatePort}
			exposesPorts = true
		}

		defaultSpinner := spinner.New()
		defaultSpinner.Style = spinnerStyle
		defaultSpinner.Spinner = spinner.MiniDot
		// defaultSpinner.Spinner.FPS = 5

		containers = append(containers, container{
			name:         container_api.Names[0],
			repository:   container_api.Image,
			tag:          container_api.Command,
			exposesPorts: exposesPorts,
			ports:        ports,
			outdated:     false,
			spinner:      defaultSpinner,
			imageID:      container_api.ImageID,
		})
	}

	return model{
		containers: containers,
		cursor:     0,
		processing: false,
		selected:   make(map[int]struct{}),
		fact:       whaleFacts.GenerateWhaleFact(),
	}
}

func (c container) update() {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}

	// Pull new image
	reader, err := cli.ImagePull(ctx, c.name, types.ImagePullOptions{})
	if err != nil {
		panic(err)
	}

	// Check if the image is new (hash is different)
	fmt.Sprintln(reader)

}

func (m model) tickSpinner() tea.Msg {
	if m.processing {
		return m.containers[m.cursor].spinner.Tick()
	} else {
		return nil
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+q", "q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 && !m.processing {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.containers)-1 && !m.processing {
				m.cursor++
			}
		case "enter", " ":
			if !m.processing {
				_, ok := m.selected[m.cursor]
				if ok {
					delete(m.selected, m.cursor)
				} else {
					m.selected[m.cursor] = struct{}{}
				}
			}
		case "r":
			if len(m.selected) > 0 {
				for i := range m.selected {
					m.containers[i].update()
				}
			}

		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.containers[m.cursor].spinner, cmd = m.containers[m.cursor].spinner.Update(msg)
		return m, cmd
	}
	return m, m.tickSpinner

}

func (m model) View() string {
	s := "Containers\n\n"

	for i, container := range m.containers {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}

		checked := " "

		if _, ok := m.selected[i]; ok {
			checked = "x"
		}

		spin_view := "" // Spinner view

		if m.processing && m.cursor == i {
			spin_view = container.spinner.View()
		}

		if container.exposesPorts {
			s += fmt.Sprintf("%s [%s] %s %s [%d, %d]  %s\n", cursor, checked, container.name, container.repository, container.ports[0], container.ports[1], spin_view)
		} else {
			s += fmt.Sprintf("%s [%s] %s %s %s\n", cursor, checked, container.name, container.repository, spin_view)
		}
	}

	s += "\n"
	if len(m.selected) > 0 {
		s += textStyle("Containers selected, press r to update")
	}

	s += helpStyle("\nPress q or ctrl + c to quit.")
	if m.processing {
		s += helpStyle(fmt.Sprintf("\nDid you know? : %s", m.fact))
	} else {
		s += "\n"
	}

	return s
}

func (m model) Init() tea.Cmd {
	return nil
}

func main() {

	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error -> %v", err)
		os.Exit(0)
	}

}
