package main

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"strconv"

	"fyne.io/fyne"
	"fyne.io/fyne/app"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/dialog"
	"fyne.io/fyne/layout"
	"fyne.io/fyne/widget"

	"github.com/gradienthealth/dicom"
	"github.com/gradienthealth/dicom/dicomtag"
)

type viewer struct {
	windowLevel int16
	windowWidth int16
	frame       *dicom.NativeFrame

	image           *canvas.Image
	study, name, id *widget.Label

	win fyne.Window
}

func (v *viewer) loadImage(data *dicom.DataSet) {
	for _, elem := range data.Elements {
		if elem.Tag == dicomtag.PixelData {
			frames := elem.Value[0].(dicom.PixelDataInfo).Frames

			if len(frames) == 0 {
				panic("No images found")
			} else if len(frames) > 1 {
				log.Println("Many images found, displaying only first element")
			}

			v.frame = &frames[0].NativeData
		} else if elem.Tag == dicomtag.PatientName {
			v.name.SetText(fmt.Sprintf("%v", elem.Value[0]))
		} else if elem.Tag == dicomtag.PatientID {
			v.id.SetText(fmt.Sprintf("%v", elem.Value[0]))
		} else if elem.Tag == dicomtag.StudyDescription {
			v.study.SetText(fmt.Sprintf("%v", elem.Value[0]))
		}
	}

}

func (v *viewer) ColorModel() color.Model {
	return color.Gray16Model
}

func (v *viewer) Bounds() image.Rectangle {
	return image.Rect(0, 0, v.frame.Cols, v.frame.Rows)
}

func (v *viewer) At(x, y int) color.Color {
	if v.frame == nil {
		return color.Gray16{0}
	}
	windowMin := v.windowLevel - v.windowWidth/2
	windowMax := windowMin + v.windowWidth

	i := y*v.frame.Rows + x
	raw := int16(v.frame.Data[i][0])

	if raw < windowMin || raw >= windowMax {
		return color.Gray16{0}
	}

	val := float32(raw-windowMin) / float32(v.windowWidth)
	return color.Gray16{uint16(float32(0xffff) * val)}
}

func makeUI(a fyne.App) *viewer {
	win := a.NewWindow("DICOM Viewer")
	view := &viewer{windowLevel: 40, windowWidth: 380, win: win}

	img := canvas.NewImageFromImage(view)
	img.FillMode = canvas.ImageFillContain

	values := widget.NewForm()
	view.id = widget.NewLabel("anon")
	values.Append("ID", view.id)
	view.name = widget.NewLabel("anon")
	values.Append("Name", view.name)
	view.study = widget.NewLabel("ANON")
	values.Append("Study", view.study)

	level := widget.NewEntry()
	level.SetText(fmt.Sprintf("%d", view.windowLevel))
	level.OnChanged = func(val string) {
		l, _ := strconv.Atoi(val)
		view.windowLevel = int16(l)

		canvas.Refresh(img)
	}
	values.Append("Level", level)

	width := widget.NewEntry()
	width.SetText(fmt.Sprintf("%d", view.windowWidth))
	width.OnChanged = func(val string) {
		w, _ := strconv.Atoi(val)
		view.windowWidth = int16(w)

		canvas.Refresh(img)
	}
	values.Append("Width", width)

	win.SetContent(fyne.NewContainerWithLayout(layout.NewBorderLayout(nil, nil, values, nil),
		values, img))
	win.Resize(fyne.NewSize(600, 400))

	return view
}

func showError(err string, a fyne.App) {
	go func() {
		d := dialog.NewInformation("DICOM Viewer Error", err, nil)
		d.Show()
	}()

	a.Run() // run the app so the dialog appears, then we will quit when dismissed
}

func main() {
	a := app.New()

	if len(os.Args) != 2 {
		showError("Must pass a parameter - the file to open", a)
		return
	}

	path := os.Args[1]
	parse, err := dicom.NewParserFromFile(path, nil)
	if err != nil {
		showError("Error loading "+path, a)
		return
	}

	data, err := parse.Parse(dicom.ParseOptions{DropPixelData: false})
	if err != nil {
		showError("Error parsing "+path, a)
		return
	}

	ui := makeUI(a)
	ui.loadImage(data)
	ui.win.ShowAndRun()
}
