//go:generate fyne bundle -o bundle.go ../../icon.png

package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/fynelabs/dicomgraphics"
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
	level, width           *widget.Entry

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
			str := fmt.Sprintf("%v", elem.Value.GetValue().([]string)[0])
			l, _ := strconv.Atoi(str)
			v.dicom.SetWindowLevel(int16(l))
			v.level.SetText(str)
		} else if elem.Tag == tag.WindowWidth {
			str := fmt.Sprintf("%v", elem.Value.GetValue().([]string)[0])
			l, _ := strconv.Atoi(str)
			v.dicom.SetWindowWidth(int16(l))
			v.width.SetText(str)
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

func (v *viewer) loadDir(dir fyne.ListableURI) {
	var (
		data   dicom.Dataset
		frames []frame.Frame
	)

	files, _ := dir.List()
	for i, file := range files {
		r, _ := storage.Reader(file)
		d, err := dicom.Parse(r, fileLength(file.Path()), nil)
		if i == 0 {
			if err != nil {
				fyne.LogError("First file in dir was not DICOM", err)
				return
			}
			data = d
		}
		if err != nil {
			fyne.LogError("Could not open dicom file "+file.Name()+" in folder", err)
			continue
		}

		t, err := d.FindElementByTag(tag.PixelData)
		if err == nil {
			frames = append(frames, t.Value.GetValue().(dicom.PixelDataInfo).Frames...)
		}
		_ = r.Close()
	}

	t, err := data.FindElementByTag(tag.PixelData)
	if err == nil {
		info := t.Value.GetValue().(dicom.PixelDataInfo)
		info.Frames = frames
		v, _ := dicom.NewValue(info)
		t.Value = v
	}

	v.loadImage(data)
}

func (v *viewer) loadFile(r io.ReadCloser, length int64) {
	data, err := dicom.Parse(r, length, nil)
	if err != nil {
		dialog.ShowError(err, v.win)
		return
	}

	err = r.Close()

	v.loadImage(data)
}

func (v *viewer) nextFrame() {
	v.setFrame(v.currentFrame + 1)
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

	v.level = widget.NewEntry()
	v.level.SetText(fmt.Sprintf("%d", dicomImg.WindowLevel()))
	v.level.OnChanged = func(val string) {
		l, _ := strconv.Atoi(val)
		dicomImg.SetWindowLevel(int16(l))

		canvas.Refresh(img)
	}
	values.Append("Window Level", v.level)

	v.width = widget.NewEntry()
	v.width.SetText(fmt.Sprintf("%d", dicomImg.WindowWidth()))
	v.width.OnChanged = func(val string) {
		w, _ := strconv.Atoi(val)
		dicomImg.SetWindowWidth(int16(w))

		canvas.Refresh(img)
	}
	values.Append("Window Width", v.width)

	return values
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
	toolbar := widget.NewToolbar(widget.NewToolbarAction(theme.FolderOpenIcon(), view.openFile))

	form := view.setupForm(dicomImg, img)
	items := []fyne.CanvasObject{toolbar, form}
	items = append(items, view.setupNavigation()...)
	bar := container.NewVBox(items...)

	win.SetContent(container.NewBorder(nil, nil, bar, nil, img))
	win.Resize(fyne.NewSize(600, 400))

	return view
}

func fileLength(path string) int64 {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return 0
	}

	return info.Size()
}

func main() {
	a := app.New()
	a.SetIcon(resourceIconPng)

	ui := makeUI(a)
	if len(os.Args) > 1 {
		path := os.Args[1]

		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			dir, err := storage.ListerForURI(storage.NewFileURI(path))
			if err != nil {
				log.Println("Failed to open folder at path:", path)
				return
			}
			ui.loadDir(dir)
		} else {
			r, err := os.Open(path)
			if err != nil {
				log.Println("Failed to load file at path:", path)
				return
			}
			ui.loadFile(r, fileLength(path))
		}
	}

	ui.loadKeys()
	ui.win.ShowAndRun()
}
