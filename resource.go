package faildep

// Resource present a resource. - -
type Resource struct {
	index int
	// Server present server name.
	// e.g. 0.0.0.0:9999
	Server string
}

// ResourceList present resource node list.
type ResourceList []Resource

func (l *ResourceList) nodeIndex(res *Resource) int {
	if res == nil {
		return -1
	}
	for i, s := range *l {
		if res.index == s.index {
			return i
		}
	}
	return -1
}
