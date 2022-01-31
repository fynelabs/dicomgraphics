package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type labelledAction struct {
	label  string
	icon   fyne.Resource
	tapped func()
}

func newLabelledAction(label string, icon fyne.Resource, tapped func()) widget.ToolbarItem {
	return &labelledAction{label: label, icon: icon, tapped: tapped}
}

func (l *labelledAction) ToolbarObject() fyne.CanvasObject {
	b := widget.NewButtonWithIcon(l.label, l.icon, l.tapped)
	b.Importance = widget.LowImportance
	return b
}

func (v *viewer) makeToolbar() *widget.Toolbar {
	return widget.NewToolbar(
		newLabelledAction("Open File", theme.FolderOpenIcon(), v.openFile),
		newLabelledAction("Open Folder", theme.FolderOpenIcon(), v.openFolder),
		widget.NewToolbarAction(theme.ViewFullScreenIcon(), v.fullScreen))
}
