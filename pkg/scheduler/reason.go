package scheduler

import (
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"golang.org/x/term"

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

func getTerminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 120 // 默认 fallback 宽度
	}
	return width
}
func printReport(report []*Report) {

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(util.ListToRow(ReportHeader))

	for _, r := range report {
		t.AppendRow(util.ListToRow(r.ToStringList()))
	}
	t.SetStyle(table.Style{
		Name: "MyStyle",
		Box:  table.StyleBoxRounded, // 圆角边框
		Options: table.Options{
			DrawBorder:      true, // 启用外边框
			SeparateColumns: true, // 列分隔线
			SeparateRows:    true, // 行分隔线（核心配置）
			SeparateFooter:  true,
			SeparateHeader:  true,
		},
		Color: table.ColorOptions{
			Separator: text.Colors{text.FgHiCyan}, // 行线颜色
			Border:    text.Colors{text.FgHiCyan},
		},
	})

	termWidth := getTerminalWidth()
	numCols := len(ReportHeader)

	margin := numCols + 1 // 估计边框 + padding + 总体留白
	availableWidth := termWidth - margin
	if availableWidth <= 0 {
		availableWidth = 30 // fallback 宽度
	}

	// 3. 每列最大宽度 = 总宽度 / 列数
	widthPerCol := availableWidth / numCols
	if widthPerCol > 30 {
		widthPerCol = 30 // 限制最大单列宽度
	}
	//fmt.Printf("calculated widthPerCol: %d", widthPerCol)

	columnConfigs := make([]table.ColumnConfig, len(ReportHeader))
	for i := range ReportHeader {
		columnConfigs[i] = table.ColumnConfig{
			Number: i + 1, // 列号从 1 开始
			//Align:       text.AlignLeft, // 设置居中对齐
			//AlignHeader: text.AlignLeft, // 设置居中对齐
			WidthMax: 33,
		}

	}

	t.SetColumnConfigs(columnConfigs)
	//style := table.StyleDefault
	//style.Format.Header = 0
	//t.SetStyle(style)
	fmt.Println()
	t.Render()
}
