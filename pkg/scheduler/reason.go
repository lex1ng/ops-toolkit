package scheduler

type Report struct {
	NodeSelectorReason     []string
	NodeAffinityReason     string
	ResourceReason         string
	TolerationReason       string
	PersistentVolumeReason string
}
