package slb

// Node present a resource.
type Node struct {
	index int
	// Server present server name.
	// e.g. 0.0.0.0:9999
	Server string
}

func (s *Node) accessible() bool {
	return true
}

// NodeList present resource node list.
type NodeList []Node

func (l *NodeList) accessibleNodes() []Node {
	nodes := make([]Node, 0, len(*l))
	for _, node := range *l {
		if node.accessible() {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

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
