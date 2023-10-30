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
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"github.com/fynelabs/dicomgraphics"
	"github.com/suyashkumar/dicom"
	"github.com/suyashkumar/dicom/pkg/frame"
	"github.com/suyashkumar/dicom/pkg/tag"
)

type viewer struct {
	dicom                  *dicomgraphics.DICOMImage
	frames                 []*frame.Frame
	currentFrame           int
	image                  *canvas.Image
	study, name, id, frame *widget.Label
	level, width           *widget.Entry

	win fyne.Window
}

func (v *viewer) loadDir(dir fyne.ListableURI) {
	var (
		data   dicom.Dataset
		frames []*frame.Frame
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

func (v *viewer) nextFrame() {
	v.setFrame(v.currentFrame + 1)
}

func (v *viewer) previousFrame() {
	v.setFrame(v.currentFrame - 1)
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
