package dicomgraphics

import (
	"github.com/suyashkumar/dicom/pkg/frame"

	"image"
	"image/color"
)

// window holds the level and width that map raw pixel values to grey levels.
type window struct {
	level int16
	width int16
}

func (w *window) WindowLevel() int16 {
	return w.level
}

func (w *window) SetWindowLevel(level int16) {
	w.level = level
}

func (w *window) WindowWidth() int16 {
	return w.width
}

func (w *window) SetWindowWidth(width int16) {
	w.width = width
}

// gray converts a raw pixel value to a grey level using the current window.
func (w *window) gray(raw int16) color.Gray16 {
	windowMin := w.level - w.width/2
	windowMax := windowMin + w.width

	if raw < windowMin {
		return color.Gray16{Y: 0}
	} else if raw >= windowMax {
		return color.Gray16{Y: 0xffff}
	}

	val := float32(raw-windowMin) / float32(w.width)
	return color.Gray16{Y: uint16(float32(0xffff) * val)}
}

// DICOMImage is an image.Image that shows a single frame with window level and width applied.
type DICOMImage struct {
	window

	frame *frame.NativeFrame
}

func (d *DICOMImage) SetFrame(frame *frame.NativeFrame) {
	d.frame = frame
}

func (d *DICOMImage) ColorModel() color.Model {
	return color.Gray16Model
}

func (d *DICOMImage) Bounds() image.Rectangle {
	if d.frame == nil {
		return image.Rectangle{}
	}
	return image.Rect(0, 0, d.frame.Cols, d.frame.Rows)
}

func (d *DICOMImage) At(x, y int) color.Color {
	if d.frame == nil {
		return color.Gray16{Y: 0}
	}

	i := y*d.frame.Cols + x
	if i < 0 || i >= len(d.frame.Data) {
		return color.Gray16{Y: 0}
	}
	return d.gray(int16(d.frame.Data[i][0]))
}

func NewDICOMImage(frame *frame.NativeFrame, level, width int16) *DICOMImage {
	return &DICOMImage{frame: frame, window: window{width: width, level: level}}
}
