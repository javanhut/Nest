package ssh

import (
	"fmt"
	"log"

	"golang.org/x/crypto/ssh"
)

type SSHInformation struct {
	username  string
	ipAddress string
	password  string
	port      string
}

func (si *SSHInformation) SetUsername(username string) {
	si.username = username
}

func (si *SSHInformation) SetIPAddress(ipaddress string) {
	si.ipAddress = ipaddress
}

func (si *SSHInformation) SetPassword(password string) {
	si.password = password
}

func (si *SSHInformation) GetUsername() string {
	return si.username
}

func (si *SSHInformation) GetIPAddress() string {
	return si.ipAddress
}

func (si *SSHInformation) GetPassword() string {
	return si.password
}

func (si *SSHInformation) SetPort(port string) {
	if port == "" {
		port = "22"
	}
	si.port = port
}

func (si *SSHInformation) GetPort() string {
	return si.port
}

type SSHConnection struct {
	SourceConnection SSHInformation
	NestedConnection []SSHInformation
}

func dialHost(info SSHInformation) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User: info.GetUsername(),
		Auth: []ssh.AuthMethod{
			ssh.Password(info.GetPassword()),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	addr := info.GetIPAddress() + ":" + info.GetPort()
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", addr, err)
	}
	return client, nil
}

// Connect establishes an SSH connection, tunneling through the nested hop if set.
// Returns the final client and a cleanup function that closes all clients in the chain.
func (sh *SSHConnection) Connect() (*ssh.Client, func(), error) {
	addr := sh.SourceConnection.GetIPAddress() + ":" + sh.SourceConnection.GetPort()
	log.Printf("[Nest] Connecting to %s@%s ...",
		sh.SourceConnection.GetUsername(), addr)

	client, err := dialHost(sh.SourceConnection)
	if err != nil {
		return nil, nil, err
	}
	log.Printf("[Nest] Connected to %s", addr)

	allClients := []*ssh.Client{client}

	for _, nested := range sh.NestedConnection {
		nestedAddr := nested.GetIPAddress() + ":" + nested.GetPort()
		log.Printf("[Nest] Tunneling to %s@%s ...",
			nested.GetUsername(), nestedAddr)

		tunnelConn, err := client.Dial("tcp", nestedAddr)
		if err != nil {
			closeAll(allClients)
			return nil, nil, fmt.Errorf("failed to tunnel to %s: %w", nestedAddr, err)
		}

		nestedConfig := &ssh.ClientConfig{
			User: nested.GetUsername(),
			Auth: []ssh.AuthMethod{
				ssh.Password(nested.GetPassword()),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}

		ncc, chans, reqs, err := ssh.NewClientConn(tunnelConn, nestedAddr, nestedConfig)
		if err != nil {
			tunnelConn.Close()
			closeAll(allClients)
			return nil, nil, fmt.Errorf("failed to authenticate to %s: %w", nestedAddr, err)
		}

		client = ssh.NewClient(ncc, chans, reqs)
		allClients = append(allClients, client)
		log.Printf("[Nest] Connected to nested host %s", nestedAddr)
	}

	cleanup := func() {
		closeAll(allClients)
	}
	return client, cleanup, nil
}

func closeAll(clients []*ssh.Client) {
	for i := len(clients) - 1; i >= 0; i-- {
		clients[i].Close()
	}
}
