package slb

import (
	"math/rand"
)

type distributor struct {
	pickServer func(metrics *nodeMetrics, currentServer *Node, servers nodeList) *Node
}

func newDistributor(addrs []string, metric *nodeMetrics) *distributor {
	dist := &distributor{}
	dist.pickServer = p2cPick
	return dist
}

func randomPick(metrics *nodeMetrics, currentServer *Node, servers nodeList) *Node {
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

func p2cPick(metrics *nodeMetrics, currentServer *Node, allNodes nodeList) *Node {
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

func excludeCurrent(current *Node, nodes nodeList) nodeList {
	if current == nil {
		return nodes
	}
	excludedNodes := make(nodeList, 0, len(nodes)-1)
	for _, node := range nodes {
		if *current != node {
			excludedNodes = append(excludedNodes, node)
		}
	}
	return excludedNodes
}
