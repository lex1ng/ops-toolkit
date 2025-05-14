package scheduler

type Report struct {
	NodeName               string
	NodeSelectorReason     string
	NodeAffinityReason     string
	NodeUnschedulable      string
	ResourceReason         string
	TolerationReason       string
	PersistentVolumeReason string
	PodAffinityReason      string
}

func (r *Report) ToStringList() []string {

	return []string{r.NodeName, r.NodeUnschedulable, r.NodeSelectorReason, r.NodeAffinityReason, r.PodAffinityReason, r.TolerationReason, r.ResourceReason, r.PersistentVolumeReason}
}
