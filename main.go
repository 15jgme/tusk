package main

// export DOCKER_API_VERSION=1.41

import (
	"context"
	"fmt"
	"os"

	"github.com/15jgme/tusk/whaleFacts"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/docker/docker/api/types"
	container_types "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

var (
	initial_load = true // On first load hide whale fact
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
	titleStyle   = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
		// Background(lipgloss.Color("#7D56F4")).
		PaddingTop(1).
		PaddingBottom(0).
		PaddingLeft(1)
	subTitleStyle = lipgloss.NewStyle().
			Italic(true).
			Foreground(lipgloss.Color("#FAFAFA")).
		// Background(lipgloss.Color("#7D56F4")).
		PaddingTop(0).
		PaddingBottom(0).
		PaddingLeft(1)
	baseStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240"))
)

type container_port struct {
	cont_port uint16
	host_port uint16
	host_IP   string
	protocol  string
}

type container struct {
	name         string
	repository   string
	tag          string
	exposesPorts bool
	ports        []container_port
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
	table      table.Model
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
	rows := []table.Row{} // Rows need to be initialized in function scope

	// Fetch containers from docker API
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}

	containers_api, err := cli.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		panic(err)
	}

	containers := []container{}

	for _, container_api := range containers_api {

		exposesPorts := false

		s_port := ""
		ports := make([]container_port, len(container_api.Ports))
		for i := range ports {
			exposesPorts = true
			// fmt.Sprintln("%s", container_api.Ports[i])
			ports[i] = container_port{
				host_port: container_api.Ports[i].PublicPort,
				host_IP:   container_api.Ports[i].IP,
				cont_port: container_api.Ports[i].PrivatePort,
				protocol:  container_api.Ports[i].Type,
			}

			// Try to avoid using the :: blank container IPs
			if ports[i].host_IP == "::" {
				ports[i].host_IP = ""
			}

			// Note: if the port is a wildcard '0' it could be that a host port
			// has not been specified. If this is the case, we will try to match it
			// with the container port, but if that port is taken we will try a wild card

			if s_port != "" {
				s_port += ", "
			}
			s_port += fmt.Sprintf("%s:%d->%d/%s", container_api.Ports[i].IP, ports[0].host_port, ports[0].cont_port, container_api.Ports[i].Type)
		}

		current_container := container{
			name:         container_api.Names[0],
			repository:   container_api.Image,
			tag:          container_api.Command,
			exposesPorts: exposesPorts,
			ports:        ports,
			outdated:     false,
			imageID:      container_api.ImageID,
		}

		rows = append(rows, []string{"", current_container.name, current_container.repository, s_port})

		containers = append(containers, current_container)
	}

	// rows = append(rows, []string{current_container.name, current_container.repository})

	defaultSpinner := spinner.New()
	defaultSpinner.Style = spinnerStyle
	defaultSpinner.Spinner = spinner.MiniDot

	// Initialize table component
	columns := []table.Column{
		{Title: "", Width: 2},
		{Title: "Name", Width: 15},
		{Title: "Image", Width: 30},
		{Title: "Ports", Width: 40},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(7),
	)

	style := table.DefaultStyles()
	style.Header = style.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	style.Selected = style.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(style)
	// End of table initalization

	return model{
		containers: containers,
		cursor:     0,
		processing: false,
		selected:   make(map[int]struct{}),
		fact:       whaleFacts.GenerateWhaleFact(),
		spinner:    defaultSpinner,
		table:      t,
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
	noWaitTimeout := 10 // to not wait for the container to exit gracefully
	if err := cli.ContainerStop(ctx, c.name, container_types.StopOptions{Timeout: &noWaitTimeout}); err != nil {
		panic(err)
	}
	// fmt.Print("Stopped container ", c.name, "... ")

	cont_bindings := nat.PortSet{}
	port_map := nat.PortMap{}

	for _, port := range c.ports {

		// Check if the host port is a wildcard '0'
		if port.host_port == 0 && isHostPortAvailable(port.cont_port) {
			port.host_port = port.cont_port
		} else if !isHostPortAvailable(port.host_port) {
			// If the port is already occupied then we need to assign a wild card
			// Docker can run multiple containers with the same port exposed somehow
			// Not sure yet how to replicate that here
			port.host_port = 0
		} // else, keep as wildcard and let the system find a port

		// Add host machine port and IP to map
		host_port_i := []nat.PortBinding{{
			HostIP:   port.host_IP,
			HostPort: fmt.Sprintf("%d", port.host_port),
		}}

		// Add container exposed port to map
		exposed_port, err := nat.NewPort(port.protocol, fmt.Sprintf("%d", port.cont_port)) // Port
		if err != nil {
			panic(err)
		}
		cont_bindings[exposed_port] = struct{}{} // Add to map

		// Add to container -> host mapping
		port_map[exposed_port] = host_port_i
	}

	config := &container_types.Config{
		Image:        c.repository,
		ExposedPorts: cont_bindings,
	}

	host_config := &container_types.HostConfig{
		PortBindings: port_map,
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
		case "esc":
			if m.table.Focused() {
				m.table.Blur()
			} else {
				m.table.Focus()
			}
		case "ctrl+q", "q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 && !m.processing {
				m.table.MoveUp(1)
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.containers)-1 && !m.processing {
				m.table.MoveDown(1)
				m.cursor++
			}
		case "enter", " ":
			if !m.processing {
				_, ok := m.selected[m.cursor]
				if ok {
					delete(m.selected, m.cursor)
					rows := m.table.Rows()
					rows[m.cursor][0] = ""
					m.table.SetRows(rows)
				} else {
					m.selected[m.cursor] = struct{}{}
					rows := m.table.Rows()
					rows[m.cursor][0] = "x"
					m.table.SetRows(rows)
				}
			}
		case "r":
			if len(m.selected) > 0 {
				m.processing = true
				initial_load = false
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
		// m.spinner, cmd = m.spinner.Update(msg)
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	}
	return m, nil

}

func (m model) View() string {

	s := titleStyle.Render("Tusk")
	s += subTitleStyle.Render("\ncontainer updates done quick ⚡")
	s += "\n\n"

	s += baseStyle.Render(m.table.View()) + "\n"

	s += "\n"
	if len(m.selected) > 0 && !m.processing {
		s += textStyle(fmt.Sprintf("%d containers selected, press r to update", len(m.selected)))
	} else if m.processing {
		s += textStyle("Processing: ") + m.spinner.View()
	}

	s += helpStyle("\nPress q or ctrl + c to quit.")
	if !initial_load {
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
