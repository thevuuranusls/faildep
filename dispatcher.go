package faildep

import (
	"math/rand"
)

type dispatcher struct {
	srvPicker ServerPicker
}

func newDispatcher() *dispatcher {
	dist := &dispatcher{}
	dist.srvPicker = P2CPick
	return dist
}

// PickServer present pick server logic.
// NewPick server logic must use this contract.
type ServerPicker func(metrics *resourceMetrics, currentServer *Resource, servers ResourceList) *Resource

// RandomPick picks server using random index.
func RandomPick(metrics *resourceMetrics, currentServer *Resource, servers ResourceList) *Resource {
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
func P2CPick(metrics *resourceMetrics, currentServer *Resource, allNodes ResourceList) *Resource {
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

func excludeCurrent(current *Resource, nodes ResourceList) ResourceList {
	if current == nil {
		return nodes
	}
	size := len(nodes) - 1
	if size < 0 {
		size = 0
	}
	excludedNodes := make(ResourceList, 0, size)
	for _, node := range nodes {
		if *current != node {
			excludedNodes = append(excludedNodes, node)
		}
	}
	return excludedNodes
}
