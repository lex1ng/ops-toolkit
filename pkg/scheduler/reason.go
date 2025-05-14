package scheduler

type Report struct {
	NodeSelectorReason     string
	NodeAffinityReason     string
	NodeUnschedulable      string
	ResourceReason         string
	TolerationReason       string
	PersistentVolumeReason string
	AffinityReason         string
}
