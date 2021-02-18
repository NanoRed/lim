package lim

import "sync"

type group struct {
	connections sync.Map
}

func newGroup() *group {
	return &group{}
}

func (g *group) add(conn *connection) {
	g.connections.Store(conn, struct{}{})
}

func (g *group) remove(conn *connection) {
	g.connections.Delete(conn)
}

func (g *group) do(f func(conn *connection) bool) {
	g.connections.Range(func(key, value interface{}) bool {
		return f(key.(*connection))
	})
}

var groups struct {
	sync.Map
	sync.Mutex
}

// Label label a connection
func Label(conn *connection, label string) {
	conn.m.Lock()
	conn.label[label] = struct{}{}
	conn.m.Unlock()
	if val, ok := groups.Load(label); ok {
		val.(*group).add(conn)
		return
	}
	groups.Lock()
	defer groups.Unlock()
	if val, ok := groups.Load(label); ok { // double check
		val.(*group).add(conn)
		return
	}
	group := newGroup()
	group.add(conn)
	groups.Store(label, group)
	return
}

func Message(label string, content []byte) error {
	return nil
	// TODO()
}
