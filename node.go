package slb

// Node present a resource.
type Node struct {
	index int
	// Server present server name.
	// e.g. 0.0.0.0:9999
	Server string
}

// NodeList present resource node list.
type NodeList []Node

func (l *NodeList) nodeIndex(server *Node) int {
	if server == nil {
		return -1
	}
	for i, s := range *l {
		if server.index == s.index {
			return i
		}
	}
	return -1
}
