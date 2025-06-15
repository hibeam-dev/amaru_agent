package service

import (
	"net"
	"sync"
	"time"

	"erlang-solutions.com/amaru_agent/internal/config"
)

const (
	poolSize        = 5
	connIdleTimeout = 60 * time.Second
	connTimeout     = 5 * time.Second
)

type pooledConnection struct {
	conn     net.Conn
	inUse    bool
	lastUsed time.Time
}

type ConnectionPool struct {
	mu   sync.Mutex
	pool []*pooledConnection

	createConn func(config.Config) (net.Conn, error)
}

func (p *ConnectionPool) GetPoolSize() int {
	return len(p.pool)
}

func (p *ConnectionPool) GetPoolStatus() (total, inUse, empty int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	total = len(p.pool)

	for _, pc := range p.pool {
		if pc == nil {
			empty++
			continue
		}

		if pc.inUse {
			inUse++
		}
	}

	return total, inUse, empty
}

func NewConnectionPool(createFn func(config.Config) (net.Conn, error)) *ConnectionPool {
	return &ConnectionPool{
		pool:       make([]*pooledConnection, poolSize),
		createConn: createFn,
	}
}

func (p *ConnectionPool) SetPoolConnection(idx int, conn net.Conn, inUse bool) {
	if idx < 0 || idx >= len(p.pool) {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.pool[idx] = &pooledConnection{
		conn:     conn,
		inUse:    inUse,
		lastUsed: time.Now(),
	}
}

func (p *ConnectionPool) GetConnection(cfg config.Config) (net.Conn, int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Try to reuse a pooled entry with no connection
	for i, pc := range p.pool {
		if pc != nil && pc.conn == nil && !pc.inUse {
			conn, err := p.createConn(cfg)
			if err != nil {
				continue
			}

			pc.conn = conn
			pc.inUse = true
			pc.lastUsed = time.Now()
			return conn, i
		}
	}

	// Try to find an idle live connection
	for i, pc := range p.pool {
		if pc != nil && pc.conn != nil && !pc.inUse {
			if isConnAlive(pc.conn) {
				pc.inUse = true
				return pc.conn, i
			}

			_ = pc.conn.Close()
			pc.conn = nil
		}
	}

	// Try to use an empty slot
	for i, pc := range p.pool {
		if pc == nil {
			conn, err := p.createConn(cfg)
			if err != nil {
				return nil, -1
			}

			p.pool[i] = &pooledConnection{
				conn:     conn,
				inUse:    true,
				lastUsed: time.Now(),
			}
			return conn, i
		}
	}

	// Pool is full, create a one-off connection
	conn, err := p.createConn(cfg)
	if err != nil {
		return nil, -1
	}

	return conn, -1
}

func (p *ConnectionPool) ReleaseConnection(idx int) {
	if idx < 0 {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if idx >= len(p.pool) || p.pool[idx] == nil {
		return
	}

	// Ensuring we don't reuse connections that might have buffered responses
	if p.pool[idx].conn != nil {
		_ = p.pool[idx].conn.Close()
		p.pool[idx].conn = nil
	}

	p.pool[idx].inUse = false
	p.pool[idx].lastUsed = time.Now()
}

func (p *ConnectionPool) RemoveConnection(idx int) {
	if idx < 0 {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if idx >= len(p.pool) || p.pool[idx] == nil {
		return
	}

	if p.pool[idx].conn != nil {
		_ = p.pool[idx].conn.Close()
	}
	p.pool[idx] = nil
}

func (p *ConnectionPool) CleanupPool() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, conn := range p.pool {
		if conn != nil && conn.conn != nil {
			_ = conn.conn.Close()
			p.pool[i] = nil
		}
	}
}

func (p *ConnectionPool) CleanIdleConnections() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()

	for i, pc := range p.pool {
		if pc == nil {
			continue
		}

		if pc.inUse {
			continue
		}

		if pc.conn == nil {
			if now.Sub(pc.lastUsed) > 10*time.Second {
				p.pool[i] = nil
			}
			continue
		}

		if now.Sub(pc.lastUsed) > 10*time.Second {
			_ = pc.conn.Close()
			pc.conn = nil
		}
	}
}

func isConnAlive(conn net.Conn) bool {
	if conn == nil {
		return false
	}

	err := conn.SetDeadline(time.Now().Add(100 * time.Millisecond))
	if err != nil {
		return false
	}

	defer func() { _ = conn.SetDeadline(time.Time{}) }()

	_, err = conn.Write([]byte{})
	return err == nil
}
