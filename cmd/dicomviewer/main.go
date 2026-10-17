//go:generate fyne bundle -o bundle.go ../../icon.png

package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
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

// loadDir parses every DICOM file directly inside dir and shows them as one series.
// Files that cannot be parsed are skipped, it is an error if none could be read.
func (v *viewer) loadDir(dir fyne.ListableURI) {
	files, err := dir.List()
	if err != nil {
		dialog.ShowError(err, v.win)
		return
	}

	var sets []dicom.Dataset
	failed := 0
	for _, file := range files {
		if isDir, _ := storage.CanList(file); isDir {
			continue
		}

		r, err := storage.Reader(file)
		if err != nil {
			failed++
			fyne.LogError("Could not open file "+file.Name()+" in folder", err)
			continue
		}
		d, err := dicom.Parse(r, fileLength(file.Path()), nil)
		_ = r.Close()
		if err != nil {
			failed++
			fyne.LogError("Skipping file "+file.Name()+" in folder", err)
			continue
		}
		sets = append(sets, d)
	}

	if len(sets) == 0 {
		dialog.ShowError(fmt.Errorf("no DICOM files could be read from %s (%d failed)", dir.Name(), failed), v.win)
		return
	}
	v.loadSeries(sets)
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

// loadMetadata reads the patient, study and windowing details from a dataset.
func (v *viewer) loadMetadata(data dicom.Dataset) {
	for _, elem := range data.Elements {
		if elem.Tag == tag.PatientName {
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

// loadImage shows the frames of a single DICOM file.
func (v *viewer) loadImage(data dicom.Dataset) {
	frames := framesOf(data)
	if len(frames) == 0 {
		dialog.ShowError(errors.New("no images found"), v.win)
		return
	}

	v.loadMetadata(data)
	v.frames = frames
	v.setFrame(0)
}

// loadSeries shows a set of single-slice DICOM files as one multi-frame series.
// The slices are ordered by their Instance Number as filenames are not reliable.
func (v *viewer) loadSeries(sets []dicom.Dataset) {
	type slice struct {
		order  int
		frames []*frame.Frame
	}

	var slices []slice
	for i, data := range sets {
		fs := framesOf(data)
		if len(fs) == 0 {
			continue
		}
		slices = append(slices, slice{order: instanceNumber(data, i), frames: fs})
	}
	sort.SliceStable(slices, func(a, b int) bool {
		return slices[a].order < slices[b].order
	})

	var frames []*frame.Frame
	for _, s := range slices {
		frames = append(frames, s.frames...)
	}
	if len(frames) == 0 {
		dialog.ShowError(errors.New("no images found"), v.win)
		return
	}

	v.loadMetadata(sets[0])
	v.frames = frames
	v.setFrame(0)
}

// framesOf returns the pixel data frames of a dataset, or nil if it has none.
func framesOf(data dicom.Dataset) []*frame.Frame {
	elem, err := data.FindElementByTag(tag.PixelData)
	if err != nil {
		return nil
	}
	info, ok := elem.Value.GetValue().(dicom.PixelDataInfo)
	if !ok {
		return nil
	}
	return info.Frames
}

// instanceNumber returns the Instance Number of a dataset, or fallback if it has none.
func instanceNumber(data dicom.Dataset, fallback int) int {
	elem, err := data.FindElementByTag(tag.InstanceNumber)
	if err != nil {
		return fallback
	}
	strs, ok := elem.Value.GetValue().([]string)
	if !ok || len(strs) == 0 {
		return fallback
	}
	n, err := strconv.Atoi(strs[0])
	if err != nil {
		return fallback
	}
	return n
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
