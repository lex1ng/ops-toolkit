package scheduler

import (
	"github.com/ops-tool/pkg/util"
)

type Report struct {
	NodeName               string
	NodeSelectorReason     util.ColorTextList
	NodeAffinityReason     util.ColorTextList
	NodeUnschedulable      util.ColorTextList
	ResourceReason         util.ColorTextList
	TolerationReason       util.ColorTextList
	PersistentVolumeReason util.ColorTextList
	PodAffinityReason      util.ColorTextList
}

func (r *Report) ToStringList() []string {

	return []string{r.NodeName, r.NodeUnschedulable.String(), r.NodeSelectorReason.String(),
		r.NodeAffinityReason.String(), r.PodAffinityReason.String(), r.TolerationReason.String(), r.ResourceReason.String(), r.PersistentVolumeReason.String()}
}
