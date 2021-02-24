package main

import (
	"fmt"
	"os"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/andydotxyz/dicomgraphics"
	"github.com/suyashkumar/dicom"
	"github.com/suyashkumar/dicom/pkg/frame"
	"github.com/suyashkumar/dicom/pkg/tag"
)

type viewer struct {
	dicom                  *dicomgraphics.DICOMImage
	frames                 []frame.Frame
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

func (v *viewer) loadImage(data dicom.Dataset) {
	for _, elem := range data.Elements {
		if elem.Tag == tag.PixelData {
			v.frames = elem.Value.GetValue().(dicom.PixelDataInfo).Frames

			if len(v.frames) == 0 {
				panic("No images found")
			}

			v.setFrame(0)
		} else if elem.Tag == tag.PatientName {
			v.name.SetText(fmt.Sprintf("%v", elem.Value))
		} else if elem.Tag == tag.PatientID {
			v.id.SetText(fmt.Sprintf("%v", elem.Value))
		} else if elem.Tag == tag.StudyDescription {
			v.study.SetText(fmt.Sprintf("%v", elem.Value))
		} else if elem.Tag == tag.WindowCenter {
			l, _ := strconv.Atoi(fmt.Sprintf("%v", elem.Value))
			v.dicom.SetWindowLevel(int16(l))
		} else if elem.Tag == tag.WindowWidth {
			l, _ := strconv.Atoi(fmt.Sprintf("%v", elem.Value))
			v.dicom.SetWindowWidth(int16(l))
		}
	}

}

func (v *viewer) loadKeys() {
	v.win.Canvas().SetOnTypedKey(func(key *fyne.KeyEvent) {
		switch key.Name {
		case fyne.KeyUp:
			v.nextFrame()
		case fyne.KeyDown:
			v.previousFrame()
		case fyne.KeyF:
			v.fullScreen()
		}
	})
}

func (v *viewer) fullScreen() {
	v.win.SetFullScreen(!v.win.FullScreen())
}

func (v *viewer) nextFrame() {
	v.setFrame(v.currentFrame + 1)
}

func (v *viewer) previousFrame() {
	v.setFrame(v.currentFrame - 1)
}

func (v *viewer) setupForm(dicomImg *dicomgraphics.DICOMImage, img *canvas.Image) fyne.Widget {
	values := widget.NewForm()

	v.id = widget.NewLabel("anon")
	values.Append("ID", v.id)
	v.name = widget.NewLabel("anon")
	values.Append("Name", v.name)
	v.study = widget.NewLabel("ANON")
	values.Append("Study", v.study)

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

	return values
}

func (v *viewer) setupNavigation() []fyne.CanvasObject {
	next := widget.NewButtonWithIcon("", theme.MoveUpIcon(), func() {
		v.nextFrame()
	})
	prev := widget.NewButtonWithIcon("", theme.MoveDownIcon(), func() {
		v.previousFrame()
	})
	full := widget.NewButtonWithIcon("", theme.ViewFullScreenIcon(), func() {
		v.fullScreen()
	})

	in := widget.NewButtonWithIcon("", theme.ZoomInIcon(), func() {
		// TODO
	})
	out := widget.NewButtonWithIcon("", theme.ZoomOutIcon(), func() {
		// TODO
	})

	up := widget.NewButtonWithIcon("", theme.MoveUpIcon(), func() {
		// TODO
	})
	down := widget.NewButtonWithIcon("", theme.MoveDownIcon(), func() {
		// TODO
	})
	left := widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
		// TODO
	})
	right := widget.NewButtonWithIcon("", theme.NavigateNextIcon(), func() {
		// TODO
	})

	directions := container.NewGridWithColumns(3,
		out, up, in,
		left, full, right,
		layout.NewSpacer(), down, layout.NewSpacer(),
	)
	v.frame = widget.NewLabel("1/1")
	return []fyne.CanvasObject{
		container.NewGridWithColumns(1, next,
			widget.NewForm(&widget.FormItem{Text: "Frame", Widget: v.frame}), prev),
		layout.NewSpacer(),
		directions,
	}
}

func makeUI(a fyne.App) *viewer {
	win := a.NewWindow("DICOM Viewer")
	dicomImg := dicomgraphics.NewDICOMImage(nil, 40, 380)

	img := canvas.NewImageFromImage(dicomImg)
	img.FillMode = canvas.ImageFillContain

	view := &viewer{dicom: dicomImg, image: img, win: win}
	form := view.setupForm(dicomImg, img)
	items := []fyne.CanvasObject{form}
	items = append(items, view.setupNavigation()...)
	bar := container.NewVBox(items...)

	win.SetContent(container.NewBorder(nil, nil, bar, nil, img))
	win.Resize(fyne.NewSize(600, 400))

	return view
}

func showError(err string, a fyne.App) {
	go func() {
		// TODO return to dialog when Fyne supports parentless dialogs
		d := a.NewWindow("DICOM Viewer Error")
		d.SetContent(widget.NewLabel(err))
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
	data, err := dicom.ParseFile(path, nil)
	if err != nil {
		showError("Error parsing "+path, a)
		return
	}

	ui := makeUI(a)
	ui.loadImage(data)
	ui.loadKeys()
	ui.win.ShowAndRun()
}
