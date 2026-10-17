package main

import (
	"fmt"
	"image"
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
func (v *viewer) setupForm() fyne.CanvasObject {
	values := widget.NewForm()

	v.id = widget.NewLabel("anon")
	values.Append("ID", v.id)
	v.name = widget.NewLabel("anon")
	values.Append("Name", v.name)
	v.study = widget.NewLabel("ANON")
	values.Append("Study", v.study)

	v.level = widget.NewEntry()
	v.level.SetText(fmt.Sprintf("%d", v.dicom.WindowLevel()))
	v.level.OnChanged = func(val string) {
		l, _ := strconv.Atoi(val)
		v.setWindowLevel(int16(l))
		v.refreshImages()
	}

	v.width = widget.NewEntry()
	v.width.SetText(fmt.Sprintf("%d", v.dicom.WindowWidth()))
	v.width.OnChanged = func(val string) {
		w, _ := strconv.Atoi(val)
		v.setWindowWidth(int16(w))
		v.refreshImages()
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

// markedImage shows an image scaled to fit and draws a vertical and horizontal line
// over it at image pixel positions chosen by marks. Scrolling steps through the
// slices of the viewer and tapping reports the pixel under the pointer.
type markedImage struct {
	widget.BaseWidget

	image       *canvas.Image
	horiz, vert *canvas.Line
	view        *viewer

	// marks returns the column and row of the image, in pixels, that the lines are drawn at.
	// A negative value hides that line.
	marks func() (col, row float32)
	// onTap is called with the image pixel that was tapped, if it is within the image.
	onTap func(col, row int)
}

func newMarkedImage(img *canvas.Image, view *viewer, marks func() (col, row float32), onTap func(col, row int)) *markedImage {
	m := &markedImage{image: img, view: view, marks: marks, onTap: onTap}
	m.horiz = canvas.NewLine(theme.Color(theme.ColorNamePrimary))
	m.vert = canvas.NewLine(theme.Color(theme.ColorNamePrimary))
	m.ExtendBaseWidget(m)
	return m
}

func (m *markedImage) CreateRenderer() fyne.WidgetRenderer {
	return &markedImageRenderer{m: m}
}

func (m *markedImage) Scrolled(ev *fyne.ScrollEvent) {
	if ev.Scrolled.DY > 0 {
		m.view.previousFrame()
	} else if ev.Scrolled.DY < 0 {
		m.view.nextFrame()
	}
}

func (m *markedImage) Tapped(ev *fyne.PointEvent) {
	pos, scale, ok := m.imageRect(m.Size())
	if !ok {
		return
	}

	col := int((ev.Position.X - pos.X) / scale)
	row := int((ev.Position.Y - pos.Y) / scale)
	b := m.image.Image.Bounds()
	if col < 0 || row < 0 || col >= b.Dx() || row >= b.Dy() {
		return
	}
	m.onTap(col, row)
}

// imageRect returns where the pixels of the image are drawn within a widget of
// the given size when the image is scaled to fit, and the size of each pixel.
func (m *markedImage) imageRect(size fyne.Size) (pos fyne.Position, scale float32, ok bool) {
	if m.image.Image == nil {
		return pos, 0, false
	}
	b := m.image.Image.Bounds()
	if b.Dx() == 0 || b.Dy() == 0 {
		return pos, 0, false
	}

	scale = min(size.Width/float32(b.Dx()), size.Height/float32(b.Dy()))
	w, h := float32(b.Dx())*scale, float32(b.Dy())*scale
	return fyne.NewPos((size.Width-w)/2, (size.Height-h)/2), scale, true
}

// layoutLines places the marker lines over the image pixels returned by marks.
func (m *markedImage) layoutLines(size fyne.Size) {
	pos, scale, ok := m.imageRect(size)
	if !ok {
		m.horiz.Hide()
		m.vert.Hide()
		return
	}

	b := m.image.Image.Bounds()
	w, h := float32(b.Dx())*scale, float32(b.Dy())*scale
	col, row := m.marks()

	if col < 0 {
		m.vert.Hide()
	} else {
		x := pos.X + col*scale
		m.vert.Position1 = fyne.NewPos(x, pos.Y)
		m.vert.Position2 = fyne.NewPos(x, pos.Y+h)
		m.vert.Show()
		m.vert.Refresh()
	}
	if row < 0 {
		m.horiz.Hide()
	} else {
		y := pos.Y + row*scale
		m.horiz.Position1 = fyne.NewPos(pos.X, y)
		m.horiz.Position2 = fyne.NewPos(pos.X+w, y)
		m.horiz.Show()
		m.horiz.Refresh()
	}
}

type markedImageRenderer struct {
	m *markedImage
}

func (r *markedImageRenderer) Destroy() {}

func (r *markedImageRenderer) Layout(size fyne.Size) {
	r.m.image.Resize(size)
	r.m.layoutLines(size)
}

func (r *markedImageRenderer) MinSize() fyne.Size {
	return r.m.image.MinSize()
}

func (r *markedImageRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.m.image, r.m.horiz, r.m.vert}
}

func (r *markedImageRenderer) Refresh() {
	r.m.image.Refresh()
	r.m.layoutLines(r.m.Size())
}

func newFitImage(img image.Image) *canvas.Image {
	c := canvas.NewImageFromImage(img)
	c.FillMode = canvas.ImageFillContain
	return c
}

func makeUI(a fyne.App) *viewer {
	const level, width = 40, 380

	win := a.NewWindow("DICOM Viewer")
	view := &viewer{win: win}
	view.dicom = dicomgraphics.NewDICOMImage(nil, level, width)
	view.coronal = dicomgraphics.NewProjection(nil, dicomgraphics.Coronal, level, width)
	view.sagittal = dicomgraphics.NewProjection(nil, dicomgraphics.Sagittal, level, width)

	view.slices = newMarkedImage(newFitImage(view.dicom), view, view.crosshairMarks, view.setCrosshair)
	view.coronalView = newMarkedImage(newFitImage(view.coronal), view,
		view.sliceMarks(view.coronal), func(col, row int) {
			view.sagittal.SetIndex(col)
			view.setFrame(view.frameAt(view.coronal, row))
		})
	view.sagittalView = newMarkedImage(newFitImage(view.sagittal), view,
		view.sliceMarks(view.sagittal), func(col, row int) {
			view.coronal.SetIndex(col)
			view.setFrame(view.frameAt(view.sagittal, row))
		})
	view.view = container.NewStack(view.slices)

	form := view.setupForm()
	items := []fyne.CanvasObject{view.makeToolbar(), form}
	items = append(items, view.setupNavigation()...)
	bar := container.NewVBox(items...)

	win.SetContent(container.NewBorder(nil, nil, bar, nil, view.view))
	win.Resize(fyne.NewSize(900, 700))

	return view
}
