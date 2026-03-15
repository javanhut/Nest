package main

import (
	"log"

	"github.com/javanhut/Nest/gui"
	"github.com/javanhut/Nest/ssh"
	"github.com/javanhut/Nest/tui"
)

func main() {
	log.Println("Initializing Nest Connection")
	m := tui.RunTui()
	connections := m.GetConnections()
	if len(connections) == 0 {
		log.Fatal("No connection details provided")
	}

	connect := ssh.SSHConnection{}
	connect.SourceConnection.SetUsername(connections[0].Username)
	connect.SourceConnection.SetIPAddress(connections[0].IPAddress)
	connect.SourceConnection.SetPassword(connections[0].Password)
	connect.SourceConnection.SetPort(connections[0].Port)

	for _, c := range connections[1:] {
		var info ssh.SSHInformation
		info.SetUsername(c.Username)
		info.SetIPAddress(c.IPAddress)
		info.SetPassword(c.Password)
		info.SetPort(c.Port)
		connect.NestedConnection = append(connect.NestedConnection, info)
	}

	client, cleanup, err := connect.Connect()
	if err != nil {
		log.Fatalf("Connection failed: %v", err)
	}
	if err := gui.OpenTerminal(client, cleanup); err != nil {
		log.Fatalf("Terminal failed: %v", err)
	}
}
