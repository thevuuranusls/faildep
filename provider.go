package faildep

type (
	NodeProvider     func() ([]string, chan struct{})
	ResourceProvider func() (func() ResourceList, chan struct{})
)

func nodeToResource(n NodeProvider) ResourceProvider {
	return func() (func() ResourceList, chan struct{}) {
		nodes, c := n()
		return func() ResourceList {
			resources := make(ResourceList, 0, len(nodes))
			for idx, node := range nodes {
				resources = append(resources, Resource{
					index:  idx,
					Server: node,
				})
			}
			return resources
		}, c
	}
}
