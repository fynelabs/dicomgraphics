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
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
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
	coronal, sagittal      *dicomgraphics.Projection
	frames                 []*frame.Frame
	currentFrame           int
	slices                 *markedImage
	coronalView            *markedImage
	sagittalView           *markedImage
	volume                 *volumeView
	view                   *fyne.Container
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
			v.setWindowLevel(int16(l))
			v.level.SetText(str)
		} else if elem.Tag == tag.WindowWidth {
			str := fmt.Sprintf("%v", elem.Value.GetValue().([]string)[0])
			l, _ := strconv.Atoi(str)
			v.setWindowWidth(int16(l))
			v.width.SetText(str)
		}
	}
}

func (v *viewer) setWindowLevel(l int16) {
	v.dicom.SetWindowLevel(l)
	v.coronal.SetWindowLevel(l)
	v.sagittal.SetWindowLevel(l)
}

func (v *viewer) setWindowWidth(w int16) {
	v.dicom.SetWindowWidth(w)
	v.coronal.SetWindowWidth(w)
	v.sagittal.SetWindowWidth(w)
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
	v.setVolume(data)
	v.setFrame(len(v.frames) / 2)
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
	v.setVolume(sets[0])
	v.setFrame(len(v.frames) / 2)
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

// decimalStrings returns the decimal string values of a tag, or nil if it is missing.
func decimalStrings(data dicom.Dataset, t tag.Tag) []string {
	elem, err := data.FindElementByTag(t)
	if err != nil {
		return nil
	}
	strs, ok := elem.Value.GetValue().([]string)
	if !ok {
		return nil
	}
	return strs
}

// spacingOf returns the distance between pixel rows, pixel columns and slices of a
// dataset in millimetres. Any value that is missing or invalid is returned as 1.
func spacingOf(data dicom.Dataset) (row, col, slice float64) {
	row, col, slice = 1, 1, 1

	if pixel := decimalStrings(data, tag.PixelSpacing); len(pixel) >= 2 {
		if r, err := strconv.ParseFloat(strings.TrimSpace(pixel[0]), 64); err == nil && r > 0 {
			row = r
		}
		if c, err := strconv.ParseFloat(strings.TrimSpace(pixel[1]), 64); err == nil && c > 0 {
			col = c
		}
	}

	between := decimalStrings(data, tag.SpacingBetweenSlices)
	if len(between) == 0 {
		between = decimalStrings(data, tag.SliceThickness)
	}
	if len(between) > 0 {
		if s, err := strconv.ParseFloat(strings.TrimSpace(between[0]), 64); err == nil && s > 0 {
			slice = s
		}
	}
	return row, col, slice
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
	v.updateVolume(true, false, false)
	v.refreshImages()
	v.frame.SetText(fmt.Sprintf("%d/%d", id+1, count))
}

// setCrosshair chooses the column shown by the sagittal projection and the row
// shown by the coronal projection, then redraws them.
func (v *viewer) setCrosshair(col, row int) {
	v.sagittal.SetIndex(col)
	v.coronal.SetIndex(row)
	v.updateVolume(false, true, true)
	v.refreshImages()
}

// updateVolume copies the chosen planes into the 3D view and moves its cut point to
// the crosshair. It does nothing when there is no volume to show.
func (v *viewer) updateVolume(axial, coronal, sagittal bool) {
	if len(v.frames) <= 1 {
		return
	}
	if axial {
		v.volume.setPlane("axial", v.dicom)
	}
	if coronal {
		v.volume.setPlane("coronal", v.coronal)
	}
	if sagittal {
		v.volume.setPlane("sagittal", v.sagittal)
	}
	first := &v.frames[0].NativeData
	v.volume.setCut(v.sagittal.Index(), v.coronal.Index(), v.currentFrame, first.Cols, first.Rows, len(v.frames))
}

// refreshImages redraws every view and its marker lines.
func (v *viewer) refreshImages() {
	v.slices.Refresh()
	v.coronalView.Refresh()
	v.sagittalView.Refresh()
	v.volume.Refresh()
}

// crosshairMarks returns the centre of the column and row of the main image that the
// sagittal and coronal projections cut through, or hides them for a single slice.
func (v *viewer) crosshairMarks() (col, row float32) {
	if len(v.frames) <= 1 {
		return -1, -1
	}
	return float32(v.sagittal.Index()) + 0.5, float32(v.coronal.Index()) + 0.5
}

// sliceMarks returns a function giving the vertical centre, within p, of the slice
// currently shown in the main image. No column is marked.
func (v *viewer) sliceMarks(p *dicomgraphics.Projection) func() (col, row float32) {
	return func() (col, row float32) {
		return -1, float32((float64(v.currentFrame) + 0.5) * p.SliceRatio())
	}
}

// frameAt returns which slice is drawn at the given row of the projection p.
func (v *viewer) frameAt(p *dicomgraphics.Projection, row int) int {
	return max(0, min(int(float64(row)/p.SliceRatio()), len(v.frames)-1))
}

// setVolume builds the coronal and sagittal projections from the loaded frames,
// and lays the window out as a 2x2 grid if there is more than one slice.
func (v *viewer) setVolume(data dicom.Dataset) {
	natives := make([]*frame.NativeFrame, len(v.frames))
	for i := range v.frames {
		natives[i] = &v.frames[i].NativeData
	}
	v.coronal.SetFrames(natives)
	v.sagittal.SetFrames(natives)

	rowSpacing, colSpacing, sliceSpacing := spacingOf(data)
	v.coronal.SetSliceRatio(sliceSpacing / colSpacing)
	v.sagittal.SetSliceRatio(sliceSpacing / rowSpacing)
	v.volume.setExtents(natives[0].Cols, natives[0].Rows, len(natives), rowSpacing, colSpacing, sliceSpacing)
	v.updateVolume(false, true, true)

	if len(v.frames) > 1 {
		v.view.Objects = []fyne.CanvasObject{container.NewGridWithColumns(2,
			v.slices, v.coronalView,
			v.sagittalView, v.volume)}
	} else {
		v.view.Objects = []fyne.CanvasObject{v.slices}
	}
	v.view.Refresh()
	v.refreshImages()
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
