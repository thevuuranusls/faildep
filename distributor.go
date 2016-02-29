package slb

import (
	"math/rand"
)

type distributor struct {
	pickServer PickServer
}

func newDistributor(addrs []string, metric *nodeMetrics) *distributor {
	dist := &distributor{}
	dist.pickServer = P2CPick
	return dist
}

// PickServer present pick server logic.
// NewPick server logic must use this contract.
type PickServer func(metrics *nodeMetrics, currentServer *Node, servers NodeList) *Node

// RandomPick picks server using random index.
func RandomPick(metrics *nodeMetrics, currentServer *Node, servers NodeList) *Node {
	if len(servers) == 0 {
		return nil
	}
	currentIdx := servers.nodeIndex(currentServer)
	if currentIdx == -1 {
		return &servers[rand.Intn(len(servers))]
	}
	nextIdx := currentIdx + 1
	return &servers[nextIdx%len(servers)]
}

// P2CPick picks server using P2C
// https://www.eecs.harvard.edu/~michaelm/postscripts/tpds2001.pdf
func P2CPick(metrics *nodeMetrics, currentServer *Node, allNodes NodeList) *Node {
	nodes := excludeCurrent(currentServer, allNodes)
	serverLen := len(nodes)
	if serverLen == 0 {
		return currentServer
	}
	if serverLen == 1 {
		return &nodes[0]
	}
	si1 := rand.Intn(serverLen)
	delta := 1
	if serverLen != 2 {
		delta = rand.Intn((serverLen - 1)) + 1
	}
	si2 := (si1 + delta) % serverLen
	s1 := nodes[si1]
	s2 := nodes[si2]
	if metrics.takeMetric(s1).takeActiveReqCount() > metrics.takeMetric(s2).takeActiveReqCount() {
		return &s2
	}
	return &s1
}

func excludeCurrent(current *Node, nodes NodeList) NodeList {
	if current == nil {
		return nodes
	}
	size := len(nodes) - 1
	if size < 0 {
		size = 0
	}
	excludedNodes := make(NodeList, 0, size)
	for _, node := range nodes {
		if *current != node {
			excludedNodes = append(excludedNodes, node)
		}
	}
	return excludedNodes
}
