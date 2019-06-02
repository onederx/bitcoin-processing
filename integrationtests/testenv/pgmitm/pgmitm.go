package pgmitm

import (
	"errors"
	"log"
	"net"
	"sync"

	"github.com/onederx/bitcoin-processing/integrationtests/util"
	"github.com/jackc/pgx/pgproto3"
)

type PgMITM struct {
	BindAddr     string
	UpstreamAddr string

	listener      net.Listener
	connections   map[*Connection]struct{}
	connectionsMu sync.Mutex
	shuttingDown  util.AtomicFlag

	clientMsgHandlers   []func(pgproto3.FrontendMessage, *Connection)
	clientMsgHandlersMu sync.Mutex

	serverMsgHandlers   []func(pgproto3.BackendMessage, *Connection)
	serverMsgHandlersMu sync.Mutex
}

func NewPgMITM(bindAddr, upstreamAddr string) (*PgMITM, error) {
	if upstreamAddr == "" {
		return nil, errors.New("UpstreamAddr can't be empty")
	}
	return &PgMITM{
		BindAddr:     bindAddr,
		UpstreamAddr: upstreamAddr,
		connections:  make(map[*Connection]struct{}),
	}, nil
}

func (p *PgMITM) Start() (err error) {
	p.listener, err = net.Listen("tcp", p.BindAddr)
	if err != nil {
		return
	}
	go p.handleConnections()
	return
}

func (p *PgMITM) handleConnections() {
	for {
		conn, err := p.listener.Accept()

		if err != nil {
			if _, ok := err.(*net.OpError); ok && p.shuttingDown.Get() {
				return
			}
			panic(err)
		}

		go p.runProxyConnection(conn)
	}
}

func (p *PgMITM) runProxyConnection(clientConn net.Conn) {
	defer clientConn.Close()

	backend, err := pgproto3.NewBackend(clientConn, clientConn)
	if err != nil {
		panic(err)
	}
	serverConn, err := net.Dial("tcp", p.UpstreamAddr)

	if err != nil {
		panic(err)
	}
	defer serverConn.Close()

	frontend, err := pgproto3.NewFrontend(serverConn, serverConn)

	if err != nil {
		panic(err)
	}

	conn := &Connection{
		clientConn: clientConn,
		serverConn: serverConn,
		backend:    backend,
		frontend:   frontend,
		wg:         &sync.WaitGroup{},

		clientMsgHandlers:   &p.clientMsgHandlers,
		clientMsgHandlersMu: &p.clientMsgHandlersMu,
		serverMsgHandlers:   &p.serverMsgHandlers,
		serverMsgHandlersMu: &p.serverMsgHandlersMu,
	}

	p.connectionsMu.Lock()
	p.connections[conn] = struct{}{}
	p.connectionsMu.Unlock()

	conn.wg.Add(2)

	log.Printf("New proxy connection started")

	go conn.proxyServerToClient()
	go conn.proxyClientToServer()

	conn.wg.Wait()
	log.Printf("Proxy connection done")

	p.connectionsMu.Lock()
	delete(p.connections, conn)
	p.connectionsMu.Unlock()
}

func (p *PgMITM) Shutdown() {
	wasNotShuttingDownAlready := p.shuttingDown.SetIf(false, true)

	if !wasNotShuttingDownAlready {
		return
	}
	p.listener.Close()

	p.connectionsMu.Lock()
	defer p.connectionsMu.Unlock()

	for conn := range p.connections {
		conn.Shutdown()
	}
}

func (p *PgMITM) AddClientMsgHandler(f func(pgproto3.FrontendMessage, *Connection)) {
	p.clientMsgHandlersMu.Lock()
	defer p.clientMsgHandlersMu.Unlock()

	p.clientMsgHandlers = append(p.clientMsgHandlers, f)
}

func (p *PgMITM) AddServerMsgHandler(f func(pgproto3.BackendMessage, *Connection)) {
	p.serverMsgHandlersMu.Lock()
	defer p.serverMsgHandlersMu.Unlock()

	p.serverMsgHandlers = append(p.serverMsgHandlers, f)
}
