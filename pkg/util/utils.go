package util

import (
	"encoding/json"
	"github.com/fatih/color"
	"github.com/jedib0t/go-pretty/v6/table"
)

func ToJSON(obj interface{}) string {
	if str, err := json.Marshal(obj); err == nil {
		return string(str)
	}

	return ""
}
func ToJSONIndent(obj interface{}) string {
	if str, err := json.MarshalIndent(obj, "", "	"); err == nil {
		return string(str)
	}
	return ""
}

func ListToRow(toTransfer []string) table.Row {

	var row table.Row

	for _, v := range toTransfer {
		row = append(row, v)
	}

	return row
}

type ColorText struct {
	Color color.Attribute
	Text  string
}

type ColorTextList []ColorText

var colorMaps map[string]color.Attribute = map[string]color.Attribute{
	"green": color.FgGreen,
	"red":   color.FgRed,
}

func (c ColorText) String() string {
	if c.Text == "" {
		return ""
	}

	return color.New(c.Color).SprintFunc()(c.Text)
}

func NewGreenText(text string) ColorText {
	return ColorText{
		Color: color.FgGreen,
		Text:  text,
	}
}

func NewRedText(text string) ColorText {
	return ColorText{
		Color: color.FgRed,
		Text:  text,
	}
}

func (ctl ColorTextList) String() string {

	result := ""
	if ctl == nil {
		return result
	}
	for _, ct := range ctl {
		if ct.String() == "" {
			continue
		}
		result += ct.String() + "\n"
	}
	return result[:len(result)-1]
}

func StringListToColorTextList(input []string, color string) ColorTextList {

	var colorTextList ColorTextList
	for _, v := range input {
		colorTextList = append(colorTextList, ColorText{
			Color: colorMaps[color],
			Text:  v,
		})
	}
	return colorTextList
}
