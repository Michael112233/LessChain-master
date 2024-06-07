package messageHub

import (
	"net"
	"sync"
)

// ConnectionsMap is a safe map for concurrent use.
type ConnectionsMap struct {
	sync.RWMutex
	connections map[string]net.Conn
}

// NewConnectionsMap creates a new ConnectionsMap.
func NewConnectionsMap() *ConnectionsMap {
	return &ConnectionsMap{
		connections: make(map[string]net.Conn),
	}
}

// Add adds a connection to the map.
func (cm *ConnectionsMap) Add(key string, conn net.Conn) {
	cm.Lock()
	cm.connections[key] = conn
	cm.Unlock()
}

// Get retrieves a connection by key.
func (cm *ConnectionsMap) Get(key string) (net.Conn, bool) {
	cm.RLock()
	conn, ok := cm.connections[key]
	cm.RUnlock()
	return conn, ok
}

// Remove removes a connection by key.
func (cm *ConnectionsMap) Remove(key string) {
	cm.Lock()
	delete(cm.connections, key)
	cm.Unlock()
}
