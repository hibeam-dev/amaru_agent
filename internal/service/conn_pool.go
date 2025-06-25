package service

import (
	"net"
	"sync"
	"time"

	"erlang-solutions.com/amaru_agent/internal/config"
	"erlang-solutions.com/amaru_agent/internal/i18n"
	"erlang-solutions.com/amaru_agent/internal/util"
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
			util.Debug(i18n.T("pool_reusing_slot", map[string]any{
				"type":  "pool",
				"Index": i,
			}))
			conn, err := p.createConn(cfg)
			if err != nil {
				util.Debug(i18n.T("pool_connection_creation_failed", map[string]any{
					"type":  "pool",
					"Index": i,
					"Error": err,
				}))
				continue
			}

			pc.conn = conn
			pc.inUse = true
			pc.lastUsed = time.Now()
			util.Debug(i18n.T("pool_connection_created", map[string]any{
				"type":  "pool",
				"Index": i,
			}))
			return conn, i
		}
	}

	// Try to find an idle live connection
	for i, pc := range p.pool {
		if pc != nil && pc.conn != nil && !pc.inUse {
			if isConnAlive(pc.conn) {
				util.Debug(i18n.T("pool_connection_reused", map[string]any{
					"type":  "pool",
					"Index": i,
				}))
				pc.inUse = true
				return pc.conn, i
			}

			util.Debug(i18n.T("pool_connection_dead", map[string]any{
				"type":  "pool",
				"Index": i,
			}))
			_ = pc.conn.Close()
			pc.conn = nil
		}
	}

	// Try to use an empty slot
	for i, pc := range p.pool {
		if pc == nil {
			util.Debug(i18n.T("pool_using_empty_slot", map[string]any{
				"type":  "pool",
				"Index": i,
			}))
			conn, err := p.createConn(cfg)
			if err != nil {
				util.Debug(i18n.T("pool_connection_creation_failed", map[string]any{
					"type":  "pool",
					"Index": i,
					"Error": err,
				}))
				return nil, -1
			}

			p.pool[i] = &pooledConnection{
				conn:     conn,
				inUse:    true,
				lastUsed: time.Now(),
			}
			util.Debug(i18n.T("pool_connection_created", map[string]any{
				"type":  "pool",
				"Index": i,
			}))
			return conn, i
		}
	}

	// Pool is full, create a one-off connection
	util.Debug(i18n.T("pool_full_creating_oneoff", map[string]any{
		"type": "pool",
	}))
	conn, err := p.createConn(cfg)
	if err != nil {
		util.Debug(i18n.T("pool_oneoff_creation_failed", map[string]any{
			"type":  "pool",
			"Error": err,
		}))
		return nil, -1
	}

	util.Debug(i18n.T("pool_oneoff_connection_created", map[string]any{
		"type": "pool",
	}))
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

	util.Debug(i18n.T("pool_connection_releasing", map[string]any{
		"type":  "pool",
		"Index": idx,
	}))

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

	util.Info(i18n.T("pool_cleanup_starting", map[string]any{
		"type":  "pool",
		"Count": len(p.pool),
	}))

	for i, conn := range p.pool {
		if conn != nil && conn.conn != nil {
			util.Debug(i18n.T("pool_connection_closing", map[string]any{
				"type":  "pool",
				"Index": i,
			}))
			_ = conn.conn.Close()
			p.pool[i] = nil
		}
	}

	util.Info(i18n.T("pool_cleanup_completed", map[string]any{
		"type": "pool",
	}))
}

func (p *ConnectionPool) CleanIdleConnections() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	cleaned := 0

	for i, pc := range p.pool {
		if pc == nil {
			continue
		}

		if pc.inUse {
			continue
		}

		if pc.conn == nil {
			if now.Sub(pc.lastUsed) > 60*time.Second {
				util.Debug(i18n.T("pool_slot_cleaned", map[string]any{
					"type":  "pool",
					"Index": i,
				}))
				p.pool[i] = nil
				cleaned++
			}
			continue
		}

		if now.Sub(pc.lastUsed) > 60*time.Second {
			util.Debug(i18n.T("pool_idle_connection_closing", map[string]any{
				"type":  "pool",
				"Index": i,
			}))
			_ = pc.conn.Close()
			pc.conn = nil
			cleaned++
		}
	}

	if cleaned > 0 {
		util.Debug(i18n.T("pool_idle_cleanup_completed", map[string]any{
			"type":    "pool",
			"Cleaned": cleaned,
		}))
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
