package slb

type Node struct {
	index  int
	server string
}

func (s *Node) accessible() bool {
	return true
}

type nodeList []Node

func (l *nodeList) accessibleNodes() []Node {
	ss := make([]Node, 0, len(*l))
	for _, server := range *l {
		if server.accessible() {
			ss = append(ss, server)
		}
	}
	return ss
}

func (l *nodeList) nodeIndex(server *Node) int {
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
