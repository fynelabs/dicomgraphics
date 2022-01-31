package main

import (
	"fmt"
	"strconv"

	"github.com/fynelabs/dicomgraphics"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var (
	presetNames = []string{
		"Abdomen",
		"Bone",
		"Brain",
		"Lungs",
		"Mediastinum",
	}

	presetValues = map[string]struct{ level, width int }{
		"Abdomen":     {40, 400},
		"Bone":        {400, 1800},
		"Brain":       {40, 80},
		"Lungs":       {600, 1500},
		"Mediastinum": {50, 350},
	}
)

func (v *viewer) fullScreen() {
	v.win.SetFullScreen(!v.win.FullScreen())
}

func (v *viewer) openFile() {
	d := dialog.NewFileOpen(func(f fyne.URIReadCloser, err error) {
		if f == nil || err != nil {
			return
		}

		v.loadFile(f, fileLength(f.URI().Path())) // TODO work with library upstream to not do this
	}, v.win)
	d.SetFilter(storage.NewExtensionFileFilter([]string{".dcm"}))
	d.Show()
}

func (v *viewer) openFolder() {
	d := dialog.NewFolderOpen(func(f fyne.ListableURI, err error) {
		if f == nil || err != nil {
			return
		}

		v.loadDir(f)
	}, v.win)
	d.Show()
}
func (v *viewer) setupForm(dicomImg *dicomgraphics.DICOMImage, img *canvas.Image) fyne.CanvasObject {
	values := widget.NewForm()

	v.id = widget.NewLabel("anon")
	values.Append("ID", v.id)
	v.name = widget.NewLabel("anon")
	values.Append("Name", v.name)
	v.study = widget.NewLabel("ANON")
	values.Append("Study", v.study)

	v.level = widget.NewEntry()
	v.level.SetText(fmt.Sprintf("%d", dicomImg.WindowLevel()))
	v.level.OnChanged = func(val string) {
		l, _ := strconv.Atoi(val)
		dicomImg.SetWindowLevel(int16(l))

		canvas.Refresh(img)
	}

	v.width = widget.NewEntry()
	v.width.SetText(fmt.Sprintf("%d", dicomImg.WindowWidth()))
	v.width.OnChanged = func(val string) {
		w, _ := strconv.Atoi(val)
		dicomImg.SetWindowWidth(int16(w))

		canvas.Refresh(img)
	}

	presets := widget.NewSelect(presetNames, func(name string) {
		val := presetValues[name]
		v.level.SetText(strconv.Itoa(val.level))
		v.width.SetText(strconv.Itoa(val.width))
	})
	return container.NewVBox(values, widget.NewCard("Window", "", widget.NewForm(
		widget.NewFormItem("Level", v.level),
		widget.NewFormItem("Width", v.width),
		widget.NewFormItem("Preset", presets))))
}

func (v *viewer) setupNavigation() []fyne.CanvasObject {
	next := widget.NewButtonWithIcon("", theme.MoveUpIcon(), func() {
		v.nextFrame()
	})
	prev := widget.NewButtonWithIcon("", theme.MoveDownIcon(), func() {
		v.previousFrame()
	})
	full := widget.NewButtonWithIcon("Full Screen", theme.ViewFullScreenIcon(), func() {
		v.fullScreen()
	})

	v.frame = widget.NewLabel("1/1")
	return []fyne.CanvasObject{
		container.NewGridWithColumns(1, next, container.NewCenter(
			widget.NewForm(&widget.FormItem{Text: "Slice", Widget: v.frame})),
			prev),
		layout.NewSpacer(),
		full,
	}
}

func makeUI(a fyne.App) *viewer {
	win := a.NewWindow("DICOM Viewer")
	dicomImg := dicomgraphics.NewDICOMImage(nil, 40, 380)

	img := canvas.NewImageFromImage(dicomImg)
	img.FillMode = canvas.ImageFillContain

	view := &viewer{dicom: dicomImg, image: img, win: win}
	form := view.setupForm(dicomImg, img)
	items := []fyne.CanvasObject{view.makeToolbar(), form}
	items = append(items, view.setupNavigation()...)
	bar := container.NewVBox(items...)

	win.SetContent(container.NewBorder(nil, nil, bar, nil, img))
	win.Resize(fyne.NewSize(600, 400))

	return view
}
