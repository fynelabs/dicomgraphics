package main

import (
	"fmt"
	"os"
	"strconv"

	"fyne.io/fyne"
	"fyne.io/fyne/app"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/dialog"
	"fyne.io/fyne/layout"
	"fyne.io/fyne/widget"

	"github.com/andydotxyz/dicomgraphics"
	"github.com/gradienthealth/dicom"
	"github.com/gradienthealth/dicom/dicomtag"
)

type viewer struct {
	dicom                  *dicomgraphics.DICOMImage
	frames                 []dicom.Frame
	currentFrame           int
	image                  *canvas.Image
	study, name, id, frame *widget.Label

	win fyne.Window
}

func (v *viewer) setFrame(id int) {
	count := len(v.frames)
	if id > count-1 {
		id = 0
	} else if id < 0 {
		id = count - 1
	}
	v.currentFrame = id

	v.dicom.SetFrame(&v.frames[id].NativeData)
	canvas.Refresh(v.image)
	v.frame.SetText(fmt.Sprintf("%d/%d", id+1, count))
}

func (v *viewer) loadImage(data *dicom.DataSet) {
	for _, elem := range data.Elements {
		if elem.Tag == dicomtag.PixelData {
			v.frames = elem.Value[0].(dicom.PixelDataInfo).Frames

			if len(v.frames) == 0 {
				panic("No images found")
			}

			v.setFrame(0)
		} else if elem.Tag == dicomtag.PatientName {
			v.name.SetText(fmt.Sprintf("%v", elem.Value[0]))
		} else if elem.Tag == dicomtag.PatientID {
			v.id.SetText(fmt.Sprintf("%v", elem.Value[0]))
		} else if elem.Tag == dicomtag.StudyDescription {
			v.study.SetText(fmt.Sprintf("%v", elem.Value[0]))
		} else if elem.Tag == dicomtag.WindowCenter {
			l, _ := strconv.Atoi(fmt.Sprintf("%v", elem.Value[0]))
			v.dicom.SetWindowLevel(int16(l))
		} else if elem.Tag == dicomtag.WindowWidth {
			l, _ := strconv.Atoi(fmt.Sprintf("%v", elem.Value[0]))
			v.dicom.SetWindowWidth(int16(l))
		}
	}

}

func (v *viewer) loadKeys() {
	v.win.Canvas().SetOnTypedKey(func(key *fyne.KeyEvent) {
		switch key.Name {
		case fyne.KeyDown:
			v.setFrame(v.currentFrame + 1)
		case fyne.KeyUp:
			v.setFrame(v.currentFrame - 1)
		}
	})
}

func makeUI(a fyne.App) *viewer {
	win := a.NewWindow("DICOM Viewer")
	dicomImg := dicomgraphics.NewDICOMImage(nil, 40, 380)

	img := canvas.NewImageFromImage(dicomImg)
	img.FillMode = canvas.ImageFillContain

	view := &viewer{dicom: dicomImg, image: img, win: win}
	values := widget.NewForm()
	view.id = widget.NewLabel("anon")
	values.Append("ID", view.id)
	view.name = widget.NewLabel("anon")
	values.Append("Name", view.name)
	view.study = widget.NewLabel("ANON")
	values.Append("Study", view.study)
	view.frame = widget.NewLabel("1/1")
	values.Append("Frame", view.frame)

	level := widget.NewEntry()
	level.SetText(fmt.Sprintf("%d", dicomImg.WindowLevel()))
	level.OnChanged = func(val string) {
		l, _ := strconv.Atoi(val)
		dicomImg.SetWindowLevel(int16(l))

		canvas.Refresh(img)
	}
	values.Append("Level", level)

	width := widget.NewEntry()
	width.SetText(fmt.Sprintf("%d", dicomImg.WindowWidth()))
	width.OnChanged = func(val string) {
		w, _ := strconv.Atoi(val)
		dicomImg.SetWindowWidth(int16(w))

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
	ui.loadKeys()
	ui.win.ShowAndRun()
}
