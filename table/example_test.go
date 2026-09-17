package table_test

import (
	"strconv"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/patterns/table"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// ExampleTable builds a Table from a fixed column set and a row stream: a
// table's whole data is Columns plus an Items observable, with sort and
// filter left to the consumer — Table renders whatever Items emits, in
// that order.
func ExampleTable() {
	type row struct {
		ID    int
		Name  string
		Value float64
	}

	shaper := tokens.DefaultTypography.DeterministicShaper()
	textCell := func(s string) layout.Widget {
		return table.RenderTextCell(shaper, tokens.PlatformLight, tokens.DefaultTypography.BodyMedium, s)
	}

	columns := []table.Column[row]{
		{Header: "ID", Width: unit.Dp(60), Cell: func(r row) layout.Widget { return textCell(strconv.Itoa(r.ID)) }},
		{Header: "Name", Cell: func(r row) layout.Widget { return textCell(r.Name) }},
		{Header: "Value", Width: unit.Dp(120), Cell: func(r row) layout.Widget { return textCell(strconv.FormatFloat(r.Value, 'f', 2, 64)) }},
	}

	rows := []row{
		{ID: 1, Name: "Alpha", Value: 1.5},
		{ID: 2, Name: "Beta", Value: 3},
		{ID: 3, Name: "Gamma", Value: 4.5},
		{ID: 4, Name: "Delta", Value: 6},
	}

	_ = table.Table(rx.Of(theme.Default()), table.Props[row]{
		Columns: columns,
		Items:   rx.Of(rows),
	})
}
