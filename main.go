package main

// export DOCKER_API_VERSION=1.41

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/15jgme/tusk/whaleFacts"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/docker/docker/api/types"
	container_types "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
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
	imageID      string
}

type model struct {
	cursor     int
	processing bool
	containers []container
	selected   map[int]struct{}
	fact       string
	spinner    spinner.Model
}

const (
	processing_finished int = 0
	processing_started      = 1
)

type notification struct {
	note    int
	message string
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

		// defaultSpinner.Spinner.FPS = 5

		containers = append(containers, container{
			name:         container_api.Names[0],
			repository:   container_api.Image,
			tag:          container_api.Command,
			exposesPorts: exposesPorts,
			ports:        ports,
			outdated:     false,
			imageID:      container_api.ImageID,
		})
	}

	defaultSpinner := spinner.New()
	defaultSpinner.Style = spinnerStyle
	defaultSpinner.Spinner = spinner.MiniDot

	return model{
		containers: containers,
		cursor:     0,
		processing: false,
		selected:   make(map[int]struct{}),
		fact:       whaleFacts.GenerateWhaleFact(),
		spinner:    defaultSpinner,
	}
}

func (m model) updateContainers() tea.Msg {
	for i := range m.selected {
		m.containers[i].update()
	}
	return notification{note: processing_finished, message: "Updated containers"}
}

func (c container) update() {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}

	// Pull new image
	_, err = cli.ImagePull(ctx, c.repository, types.ImagePullOptions{})
	if err != nil {
		panic(err)
	}

	// TODO Check if the image is new (hash is different)
	// fmt.Println(reader)

	// fmt.Print("Stopping container ", c.name, "... ")
	noWaitTimeout := 0 // to not wait for the container to exit gracefully
	if err := cli.ContainerStop(ctx, c.name, container_types.StopOptions{Timeout: &noWaitTimeout}); err != nil {
		panic(err)
	}
	// fmt.Print("Stopped container ", c.name, "... ")

	exposed_port, err := nat.NewPort("tcp", strconv.Itoa(int(c.ports[0])))
	if err != nil {
		panic(err)
	}
	host_port := fmt.Sprintf("%d", c.ports[1])

	config := &container_types.Config{
		Image: c.repository,
		ExposedPorts: nat.PortSet{
			exposed_port: struct{}{},
		},
	}

	host_config := &container_types.HostConfig{
		PortBindings: nat.PortMap{
			exposed_port: []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: host_port,
				},
			},
		},
	}

	resp, err := cli.ContainerCreate(ctx, config, host_config, nil, nil, "")
	if err != nil {
		panic(err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		panic(err)
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
				m.processing = true
				return m, tea.Batch(m.updateContainers, m.spinner.Tick)
			}
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case notification:
		switch msg.note {
		case processing_finished:
			m.processing = false
			return initialModel(), nil
		}

	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil

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

		if container.exposesPorts {
			s += fmt.Sprintf("%s [%s] %s %s [%d, %d]\n", cursor, checked, container.name, container.repository, container.ports[0], container.ports[1])
		} else {
			s += fmt.Sprintf("%s [%s] %s %s\n", cursor, checked, container.name, container.repository)
		}
	}

	s += "\n"
	if len(m.selected) > 0 && !m.processing {
		s += textStyle("Containers selected, press r to update")
	} else if m.processing {
		s += textStyle("Processing: ") + m.spinner.View()
	}

	s += helpStyle("\nPress q or ctrl + c to quit.")
	if m.processing {
		s += helpStyle(fmt.Sprintf("\nDid you know? : %s\n", m.fact))
	} else {
		s += "\n"
	}

	return s
}

func (m model) Init() tea.Cmd {
	return m.spinner.Tick
}

func main() {

	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error -> %v", err)
		os.Exit(0)
	}

}
