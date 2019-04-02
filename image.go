package dicomgraphics

import (
	"github.com/gradienthealth/dicom"
	"image"
	"image/color"
)

type DICOMImage struct {
	level int16
	width int16

	frame *dicom.NativeFrame
}

func (d *DICOMImage) SetFrame(frame *dicom.NativeFrame) {
	d.frame = frame
}

func (d *DICOMImage) WindowLevel() int16 {
	return d.level
}

func (d *DICOMImage) SetWindowLevel(level int16) {
	d.level = level
}

func (d *DICOMImage) WindowWidth() int16 {
	return d.width
}

func (d *DICOMImage) SetWindowWidth(width int16) {
	d.width = width
}

func (d *DICOMImage) ColorModel() color.Model {
	return color.Gray16Model
}

func (d *DICOMImage) Bounds() image.Rectangle {
	return image.Rect(0, 0, d.frame.Cols, d.frame.Rows)
}

func (d *DICOMImage) At(x, y int) color.Color {
	if d.frame == nil {
		return color.Gray16{Y: 0}
	}
	windowMin := d.level - d.width/2
	windowMax := windowMin + d.width

	i := y*d.frame.Rows + x
	raw := int16(d.frame.Data[i][0])

	if raw < windowMin || raw >= windowMax {
		return color.Gray16{Y: 0}
	}

	val := float32(raw-windowMin) / float32(d.width)
	return color.Gray16{Y: uint16(float32(0xffff) * val)}
}

func NewDICOMImage(frame *dicom.NativeFrame, level, width int16) *DICOMImage {
	return &DICOMImage{frame: frame, width: width, level: level}
}
