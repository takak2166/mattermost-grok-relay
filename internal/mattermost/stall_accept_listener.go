package mattermost

import (
	"net"
	"sync"
)

// stallAcceptListener accepts TCP connections and does not send a WebSocket response.
type stallAcceptListener struct {
	ln       net.Listener
	once     sync.Once
	accepted chan struct{}
}

func newStallAcceptListener(addr string) (*stallAcceptListener, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	s := &stallAcceptListener{
		ln:       ln,
		accepted: make(chan struct{}),
	}
	go s.serve()
	return s, nil
}

func (s *stallAcceptListener) serve() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.once.Do(func() { close(s.accepted) })
		go func(c net.Conn) {
			defer c.Close()
			buf := make([]byte, 4096)
			for {
				if _, err := c.Read(buf); err != nil {
					return
				}
			}
		}(conn)
	}
}

func (s *stallAcceptListener) WSOrigin() string {
	host, port, _ := net.SplitHostPort(s.ln.Addr().String())
	return "ws://" + host + ":" + port
}

func (s *stallAcceptListener) HTTPSiteURL() string {
	host, port, _ := net.SplitHostPort(s.ln.Addr().String())
	return "http://" + host + ":" + port
}

func (s *stallAcceptListener) Close() error {
	return s.ln.Close()
}
