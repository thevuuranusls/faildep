package slb

import (
	"math/rand"
)

type distributor struct {
	metrics    *nodeMetrics
	pickServer func(currentServer *Node, servers nodeList) *Node
}

func newDistributor(addrs []string, metric *nodeMetrics) *distributor {
	dist := &distributor{
		metrics: metric,
	}
	dist.pickServer = dist.p2cPick
	return dist
}

func randomPick(currentServer *Node, servers nodeList) *Node {
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

func (d *distributor) p2cPick(currentServer *Node, servers nodeList) *Node {
	serverLen := len(servers)
	if serverLen == 0 {
		return nil
	}
	if serverLen == 1 {
		return &servers[0]
	}
	si1 := rand.Intn(serverLen)
	delta := 1
	if serverLen != 2 {
		delta = rand.Intn((serverLen - 1)) + 1
	}
	si2 := (si1 + delta) % serverLen
	s1 := servers[si1]
	s2 := servers[si2]
	if d.metrics.takeMetric(s1).takeActiveReqCount() > d.metrics.takeMetric(s2).takeActiveReqCount() {
		return &s2
	}
	return &s1
}
