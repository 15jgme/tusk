package main

// export DOCKER_API_VERSION=1.41

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

type container struct {
	name         string
	repository   string
	tag          string
	exposesPorts bool
	ports        []uint16
	outdated     bool
}

type model struct {
	cursor     int
	containers []container
	selected   map[int]struct{}
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

	for _, container_api := range containers_api {
		fmt.Printf("container_api: %+v\n", container_api)

		ports := []uint16{0, 0}
		exposesPorts := false
		if len(container_api.Ports) > 0 {
			ports = []uint16{container_api.Ports[0].PublicPort, container_api.Ports[0].PrivatePort}
			exposesPorts = true
		}

		containers = append(containers, container{
			name:         container_api.Names[0],
			repository:   container_api.Image,
			tag:          container_api.Command,
			exposesPorts: exposesPorts,
			ports:        ports,
			outdated:     false,
		})
	}

	return model{
		containers: containers,
		cursor:     0,
		selected:   make(map[int]struct{}),
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+q", "q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.containers)-1 {
				m.cursor++
			}
		case "enter", " ":
			_, ok := m.selected[m.cursor]
			if ok {
				delete(m.selected, m.cursor)
			} else {
				m.selected[m.cursor] = struct{}{}
			}
		}
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

	s += "\nPress q or ctrl + c to quit.\n"

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
