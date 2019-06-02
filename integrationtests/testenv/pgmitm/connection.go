package pgmitm

import (
	"io"
	"log"
	"net"
	"sync"

	"github.com/jackc/pgx/pgproto3"

	"github.com/onederx/bitcoin-processing/integrationtests/util"
)

type Connection struct {
	clientConn net.Conn
	serverConn net.Conn
	frontend   *pgproto3.Frontend
	backend    *pgproto3.Backend
	wg         *sync.WaitGroup
	exiting    util.AtomicFlag

	clientMsgHandlers   *[]func(pgproto3.FrontendMessage, *Connection)
	clientMsgHandlersMu *sync.Mutex

	serverMsgHandlers   *[]func(pgproto3.BackendMessage, *Connection)
	serverMsgHandlersMu *sync.Mutex
}

func (c *Connection) proxyServerToClient() {
	defer c.wg.Done()

	for {
		msg, err := c.frontend.Receive()

		switch {
		case err == nil:
		case err == io.EOF && c.exiting.Get():
			return
		case err == io.EOF && !c.exiting.Get():
			log.Printf("Connection unexpectedly closed by server")
			c.exiting.Set(true)
			c.clientConn.(*net.TCPConn).CloseWrite()
			return
		case err != nil && err != io.EOF:
			panic(err)
		}

		c.runServerMsgHandlers(msg)

		err = c.backend.Send(msg)

		if err != nil && !c.exiting.Get() {
			panic(err)
		}
	}
}

func (c *Connection) proxyClientToServer() {
	defer c.wg.Done()

	start, err := c.backend.ReceiveStartupMessage()

	if err != nil {
		panic(err)
	}

	err = c.frontend.Send(start)

	if err != nil {
		panic(err)
	}

	for {
		msg, err := c.backend.Receive()

		switch {
		case err == nil:
		case err == io.EOF && c.exiting.Get():
			return
		case err == io.EOF && !c.exiting.Get():
			log.Printf("Connection unexpectedly closed by client")
			c.exiting.Set(true)
			c.serverConn.(*net.TCPConn).CloseWrite()
			return
		case err != nil && err != io.EOF:
			panic(err)
		}

		if _, ok := msg.(*pgproto3.Terminate); ok {
			c.exiting.Set(true)
		}

		c.runClientMsgHandlers(msg)

		err = c.frontend.Send(msg)

		if err != nil && !c.exiting.Get() {
			panic(err)
		}
	}
}

func (c *Connection) runClientMsgHandlers(msg pgproto3.FrontendMessage) {
	c.clientMsgHandlersMu.Lock()
	defer c.clientMsgHandlersMu.Unlock()
	for _, handler := range *c.clientMsgHandlers {
		handler(msg, c)
	}
}

func (c *Connection) runServerMsgHandlers(msg pgproto3.BackendMessage) {
	c.serverMsgHandlersMu.Lock()
	defer c.serverMsgHandlersMu.Unlock()
	for _, handler := range *c.serverMsgHandlers {
		handler(msg, c)
	}
}

func (c *Connection) Shutdown() {
	wasNotShuttingDownAlready := c.exiting.SetIf(false, true)

	if !wasNotShuttingDownAlready {
		return
	}

	c.serverConn.(*net.TCPConn).CloseWrite()
	c.clientConn.(*net.TCPConn).CloseWrite()
}
