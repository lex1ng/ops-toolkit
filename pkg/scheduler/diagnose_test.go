package scheduler

import (
	"github.com/ops-tool/pkg/scheduler/framework"
	"github.com/ops-tool/pkg/scheduler/framework/interpodaffinity"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"testing"
)

func TestAnalyzer_checkUnSchedulableNode(t *testing.T) {
	type fields struct {
		ClientSet              *kubernetes.Clientset
		targetPod              *corev1.Pod
		Namespace              string
		PodName                string
		TargetConditions       *Conditions
		NodeReport             NodeReport
		allNodes               []corev1.Node
		interPodAffinityPlugin *interpodaffinity.InterPodAffinity
	}
	type args struct {
		node *corev1.Node
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		{
			name: "Node Unschedulable",
			fields: fields{
				TargetConditions: &Conditions{
					Toleration: []corev1.Toleration{},
				},
			},
			args: args{
				node: &corev1.Node{
					Spec: corev1.NodeSpec{
						Unschedulable: true,
					},
				},
			},
			want: "node unSchedulable",
		},
		{
			name: "pod tolerate Unschedulable Node",
			fields: fields{
				TargetConditions: &Conditions{
					Toleration: []corev1.Toleration{
						{
							Key:    corev1.TaintNodeUnschedulable,
							Effect: corev1.TaintEffectNoSchedule,
						},
					},
				},
			},
			args: args{
				node: &corev1.Node{
					Spec: corev1.NodeSpec{
						Unschedulable: true,
					},
				},
			},
			want: "",
		},
		{
			name: "pod has unschedulable Key",
			fields: fields{
				TargetConditions: &Conditions{
					Toleration: []corev1.Toleration{
						{
							Key: corev1.TaintNodeUnschedulable,
						},
					},
				},
			},
			args: args{
				node: &corev1.Node{
					Spec: corev1.NodeSpec{
						Unschedulable: true,
					},
				},
			},
			want: "",
		},
		{
			name: "pod tolerate noschedule effect",
			fields: fields{
				TargetConditions: &Conditions{
					Toleration: []corev1.Toleration{
						{
							Effect: corev1.TaintEffectNoSchedule,
						},
					},
				},
			},
			args: args{
				node: &corev1.Node{
					Spec: corev1.NodeSpec{
						Unschedulable: true,
					},
				},
			},
			want: "",
		},
		{
			name: "pod don't have unschedulable Key",
			fields: fields{
				TargetConditions: &Conditions{
					Toleration: []corev1.Toleration{
						{
							Key: "123",
						},
					},
				},
			},
			args: args{
				node: &corev1.Node{
					Spec: corev1.NodeSpec{
						Unschedulable: true,
					},
				},
			},
			want: "node unSchedulable",
		},
		{
			name: "pod don't have unschedulable Key",
			fields: fields{
				TargetConditions: &Conditions{
					Toleration: []corev1.Toleration{
						{
							Effect: "NoExecute",
						},
					},
				},
			},
			args: args{
				node: &corev1.Node{
					Spec: corev1.NodeSpec{
						Unschedulable: true,
					},
				},
			},
			want: "node unSchedulable",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Analyzer{
				ClientSet:              tt.fields.ClientSet,
				targetPod:              tt.fields.targetPod,
				Namespace:              tt.fields.Namespace,
				PodName:                tt.fields.PodName,
				TargetConditions:       tt.fields.TargetConditions,
				allNodes:               tt.fields.allNodes,
				interPodAffinityPlugin: tt.fields.interPodAffinityPlugin,
			}
			if got := a.checkUnSchedulableNode(tt.args.node); got != tt.want {
				t.Errorf("checkUnSchedulableNode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnalyzer_checkNodeSelector(t *testing.T) {
	type fields struct {
		ClientSet              *kubernetes.Clientset
		targetPod              *corev1.Pod
		Namespace              string
		PodName                string
		TargetConditions       *Conditions
		NodeReport             NodeReport
		allNodes               []corev1.Node
		interPodAffinityPlugin *interpodaffinity.InterPodAffinity
	}
	type args struct {
		nodeLabels map[string]string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		{
			name: "empty",
			fields: fields{
				TargetConditions: &Conditions{},
			},
			args: args{
				nodeLabels: map[string]string{},
			},
			want: "",
		},
		{
			name: "meet",
			fields: fields{
				TargetConditions: &Conditions{
					NodeSelector: map[string]string{
						"foo": "bar",
					},
				},
			},
			args: args{
				nodeLabels: map[string]string{
					"foo": "bar",
				},
			},
			want: "",
		},
		{
			name: "not meet",
			fields: fields{
				TargetConditions: &Conditions{
					NodeSelector: map[string]string{
						"foo": "bar",
					},
				},
			},
			args: args{
				nodeLabels: map[string]string{
					"foo": "bar1",
				},
			},
			want: "foo:bar",
		},
		{
			name: "not meet multiple",
			fields: fields{
				TargetConditions: &Conditions{
					NodeSelector: map[string]string{
						"foo":  "bar",
						"foo2": "bar2",
					},
				},
			},
			args: args{
				nodeLabels: map[string]string{
					"foo": "bar1",
				},
			},
			want: "foo:bar\nfoo2:bar2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Analyzer{
				ClientSet:              tt.fields.ClientSet,
				targetPod:              tt.fields.targetPod,
				Namespace:              tt.fields.Namespace,
				PodName:                tt.fields.PodName,
				TargetConditions:       tt.fields.TargetConditions,
				allNodes:               tt.fields.allNodes,
				interPodAffinityPlugin: tt.fields.interPodAffinityPlugin,
			}
			if got := a.checkNodeSelector(tt.args.nodeLabels); got != tt.want {
				t.Errorf("checkNodeSelector() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnalyzer_checkTaints(t *testing.T) {
	tolerationExists := corev1.Toleration{
		Key:      "disk",
		Operator: corev1.TolerationOpExists,
		Effect:   corev1.TaintEffectNoSchedule,
	}

	tolerationEqual := corev1.Toleration{
		Key:      "gpu",
		Operator: corev1.TolerationOpEqual,
		Value:    "nvidia",
		Effect:   corev1.TaintEffectNoExecute,
	}

	type fields struct {
		ClientSet              *kubernetes.Clientset
		targetPod              *corev1.Pod
		Namespace              string
		PodName                string
		TargetConditions       *Conditions
		NodeReport             NodeReport
		allNodes               []corev1.Node
		interPodAffinityPlugin *interpodaffinity.InterPodAffinity
	}
	type args struct {
		taints []corev1.Taint
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		{
			name: "empty",
			fields: fields{
				TargetConditions: &Conditions{},
			},
			args: args{
				taints: []corev1.Taint{},
			},
			want: "",
		},
		{
			name: "no taints",
			fields: fields{
				TargetConditions: &Conditions{},
			},
			args: args{
				taints: nil,
			},
			want: "",
		},
		{
			name: "no taints",
			fields: fields{
				TargetConditions: &Conditions{},
			},
			args: args{
				taints: nil,
			},
			want: "",
		},
		{
			name: "untolerated taint",
			fields: fields{
				TargetConditions: &Conditions{
					Toleration: []corev1.Toleration{}, // 空容忍列表
				},
			},
			args: args{
				taints: []corev1.Taint{
					{Key: "disk", Value: "ssd", Effect: corev1.TaintEffectNoSchedule},
				},
			},
			want: "{\n\t\"key\": \"disk\",\n\t\"value\": \"ssd\",\n\t\"effect\": \"NoSchedule\"\n}",
		},
		{
			name: "部分容忍",
			fields: fields{
				TargetConditions: &Conditions{
					Toleration: []corev1.Toleration{tolerationExists},
				},
			},
			args: args{
				taints: []corev1.Taint{
					{Key: "disk", Value: "ssd", Effect: corev1.TaintEffectNoSchedule},  // 可被容忍
					{Key: "gpu", Value: "nvidia", Effect: corev1.TaintEffectNoExecute}, // 未容忍
				},
			},
			want: "{\n\t\"key\": \"gpu\",\n\t\"value\": \"nvidia\",\n\t\"effect\": \"NoExecute\"\n}",
		},
		{
			name: "全容忍",
			fields: fields{
				TargetConditions: &Conditions{
					Toleration: []corev1.Toleration{tolerationExists, tolerationEqual},
				},
			},
			args: args{
				taints: []corev1.Taint{
					{Key: "disk", Value: "anyvalue", Effect: corev1.TaintEffectNoSchedule},
					{Key: "gpu", Value: "nvidia", Effect: corev1.TaintEffectNoExecute},
				},
			},
			want: "",
		},
		{
			name: "复杂匹配规则",
			fields: fields{
				TargetConditions: &Conditions{
					Toleration: []corev1.Toleration{
						{
							Key:      "special",
							Operator: corev1.TolerationOpEqual,
							Value:    "true",
							Effect:   corev1.TaintEffectPreferNoSchedule,
						},
					},
				},
			},
			args: args{
				taints: []corev1.Taint{
					{Key: "special", Value: "true", Effect: corev1.TaintEffectPreferNoSchedule},  // 精确匹配
					{Key: "special", Value: "false", Effect: corev1.TaintEffectPreferNoSchedule}, // 值不匹配
				},
			},
			want: "{\n\t\"key\": \"special\",\n\t\"value\": \"false\",\n\t\"effect\": \"PreferNoSchedule\"\n}",
		},
		{
			name: "Effect 不匹配",
			fields: fields{
				TargetConditions: &Conditions{
					Toleration: []corev1.Toleration{
						{Key: "network", Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule},
					},
				},
			},
			args: args{
				taints: []corev1.Taint{
					{Key: "network", Value: "unstable", Effect: corev1.TaintEffectNoExecute}, // Effect 不同
				},
			},
			want: "{\n\t\"key\": \"network\",\n\t\"value\": \"unstable\",\n\t\"effect\": \"NoExecute\"\n}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Analyzer{
				ClientSet:              tt.fields.ClientSet,
				targetPod:              tt.fields.targetPod,
				Namespace:              tt.fields.Namespace,
				PodName:                tt.fields.PodName,
				TargetConditions:       tt.fields.TargetConditions,
				allNodes:               tt.fields.allNodes,
				interPodAffinityPlugin: tt.fields.interPodAffinityPlugin,
			}
			if got := a.checkTaints(tt.args.taints); got != tt.want {
				t.Errorf("checkTaints() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnalyzer_checkVolumeNodeAffinity(t *testing.T) {
	type fields struct {
		ClientSet              *kubernetes.Clientset
		targetPod              *corev1.Pod
		Namespace              string
		PodName                string
		TargetConditions       *Conditions
		NodeReport             NodeReport
		allNodes               []corev1.Node
		interPodAffinityPlugin *interpodaffinity.InterPodAffinity
	}
	type args struct {
		nodeLabels map[string]string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		{
			name: "nil",
			fields: fields{
				TargetConditions: &Conditions{
					PersistentVolumeAffinity: nil,
				},
			},
			args: args{
				nodeLabels: map[string]string{},
			},
			want: "",
		},
		{
			name: "empty",
			fields: fields{
				TargetConditions: &Conditions{
					PersistentVolumeAffinity: []*corev1.VolumeNodeAffinity{},
				},
			},
			args: args{
				nodeLabels: map[string]string{},
			},
			want: "",
		},
		{
			name: "standard",
			fields: fields{
				TargetConditions: &Conditions{
					PersistentVolumeAffinity: []*corev1.VolumeNodeAffinity{
						{
							Required: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      corev1.LabelHostname,
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"base1-xakd.dev6.abcstackint.com"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			args: args{
				nodeLabels: map[string]string{
					"kubernetes.io/hostname": "base1-xakd.dev6.abcstackint.com",
				},
			},
			want: "",
		},
		{
			name: "standard not meet",
			fields: fields{
				TargetConditions: &Conditions{
					PersistentVolumeAffinity: []*corev1.VolumeNodeAffinity{
						{
							Required: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      corev1.LabelHostname,
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"base1-xakd.dev6.abcstackint.com"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			args: args{
				nodeLabels: map[string]string{
					"kubernetes.io/hostname": "base2-xakd.dev6.abcstackint.com",
				},
			},
			want: "{\n\t\"nodeSelectorTerms\": [\n\t\t{\n\t\t\t\"matchExpressions\": [\n\t\t\t\t{\n\t\t\t\t\t\"key\": \"kubernetes.io/hostname\",\n\t\t\t\t\t\"operator\": \"In\",\n\t\t\t\t\t\"values\": [\n\t\t\t\t\t\t\"base1-xakd.dev6.abcstackint.com\"\n\t\t\t\t\t]\n\t\t\t\t}\n\t\t\t]\n\t\t}\n\t]\n}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Analyzer{
				ClientSet:              tt.fields.ClientSet,
				targetPod:              tt.fields.targetPod,
				Namespace:              tt.fields.Namespace,
				PodName:                tt.fields.PodName,
				TargetConditions:       tt.fields.TargetConditions,
				allNodes:               tt.fields.allNodes,
				interPodAffinityPlugin: tt.fields.interPodAffinityPlugin,
			}
			if got := a.checkVolumeNodeAffinity(tt.args.nodeLabels); got != tt.want {
				t.Errorf("checkVolumeNodeAffinity() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnalyzer_doCheckResource(t *testing.T) {
	type fields struct {
		ClientSet              *kubernetes.Clientset
		targetPod              *corev1.Pod
		Namespace              string
		PodName                string
		TargetConditions       *Conditions
		NodeReport             NodeReport
		allNodes               []corev1.Node
		interPodAffinityPlugin *interpodaffinity.InterPodAffinity
	}
	type args struct {
		want framework.ResourceList
		have framework.ResourceList
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		{
			name:   "nil",
			fields: fields{},
			args: args{
				want: framework.ResourceList{},
				have: framework.ResourceList{},
			},
			want: "",
		},
		{
			name:   "standard meet",
			fields: fields{},
			args: args{
				want: framework.ResourceList{
					"cpu": &framework.Resource{
						Name:     "cpu",
						Requests: 123,
					},
				},
				have: framework.ResourceList{
					"cpu": &framework.Resource{
						Name:     "cpu",
						Requests: 2,
						Capacity: 130,
					},
				},
			},
			want: "",
		},
		{
			name:   "standard not meet",
			fields: fields{},
			args: args{
				want: framework.ResourceList{
					"cpu": &framework.Resource{
						Name:     "cpu",
						Requests: 123,
					},
				},
				have: framework.ResourceList{
					"cpu": &framework.Resource{
						Name:     "cpu",
						Requests: 2,
						Capacity: 100,
					},
				},
			},
			want: "cpu: want 123, have 98 left",
		},
		{
			name:   "standard not exist",
			fields: fields{},
			args: args{
				want: framework.ResourceList{
					"cpu": &framework.Resource{
						Name:     "cpu",
						Requests: 123,
					},
				},
				have: framework.ResourceList{},
			},
			want: "cpu: want 123, have 0",
		},
		{
			name:   "standard not meet multiple",
			fields: fields{},
			args: args{
				want: framework.ResourceList{
					"cpu": &framework.Resource{
						Name:     "cpu",
						Requests: 123,
					},
					"netdevice": &framework.Resource{
						Name:     "netdevice",
						Requests: 2,
					},
				},
				have: framework.ResourceList{
					"cpu": &framework.Resource{
						Name:     "cpu",
						Requests: 123,
						Capacity: 130,
					},
					"netdevice": &framework.Resource{
						Name:     "netdevice",
						Requests: 1,
						Capacity: 2,
					},
				},
			},
			want: "cpu: want 123, have 7 left\nnetdevice: want 2, have 1 left",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Analyzer{
				ClientSet:              tt.fields.ClientSet,
				targetPod:              tt.fields.targetPod,
				Namespace:              tt.fields.Namespace,
				PodName:                tt.fields.PodName,
				TargetConditions:       tt.fields.TargetConditions,
				allNodes:               tt.fields.allNodes,
				interPodAffinityPlugin: tt.fields.interPodAffinityPlugin,
			}
			if got := a.doCheckResource(tt.args.want, tt.args.have); got != tt.want {
				t.Errorf("doCheckResource() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnalyzer_checkNodeAffinity(t *testing.T) {
	type fields struct {
		ClientSet              *kubernetes.Clientset
		targetPod              *corev1.Pod
		Namespace              string
		PodName                string
		TargetConditions       *Conditions
		NodeReport             NodeReport
		allNodes               []corev1.Node
		interPodAffinityPlugin *interpodaffinity.InterPodAffinity
	}
	type args struct {
		node *corev1.Node
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		{
			name: "nil",
			fields: fields{
				TargetConditions: &Conditions{
					Affinity: &corev1.Affinity{},
				},
			},
			args: args{
				node: &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
				},
			},
			want: "",
		},
		{
			name: "nil2",
			fields: fields{
				TargetConditions: &Conditions{
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{},
					},
				},
			},
			args: args{
				node: &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
				},
			},
			want: "",
		},
		{
			name: "standard meet",
			fields: fields{
				TargetConditions: &Conditions{
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "topology.kubernetes.io/subnet",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"podcidr-10-10-84-0-mask-22"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			args: args{
				node: &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
						Labels: map[string]string{
							"topology.kubernetes.io/subnet": "podcidr-10-10-84-0-mask-22",
						},
					},
				},
			},
			want: "",
		},
		{
			name: "standard not meet",
			fields: fields{
				TargetConditions: &Conditions{
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "topology.kubernetes.io/subnet",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"podcidr-10-10-84-0-mask-22"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			args: args{
				node: &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
						Labels: map[string]string{
							"topology.kubernetes.io/subnet": "podcidr-10-10-86-0-mask-22",
						},
					},
				},
			},
			want: "don't match node affinity: {\"nodeSelectorTerms\":[{\"matchExpressions\":[{\"key\":\"topology.kubernetes.io/subnet\",\"operator\":\"In\",\"values\":[\"podcidr-10-10-84-0-mask-22\"]}]}]}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Analyzer{
				ClientSet:              tt.fields.ClientSet,
				targetPod:              tt.fields.targetPod,
				Namespace:              tt.fields.Namespace,
				PodName:                tt.fields.PodName,
				TargetConditions:       tt.fields.TargetConditions,
				allNodes:               tt.fields.allNodes,
				interPodAffinityPlugin: tt.fields.interPodAffinityPlugin,
			}
			if got := a.checkNodeAffinity(tt.args.node); got != tt.want {
				t.Errorf("checkNodeAffinity() = %v, want %v", got, tt.want)
			}
		})
	}
}
