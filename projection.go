package dicomgraphics

import (
	"image"
	"image/color"
	"math"

	"github.com/suyashkumar/dicom/pkg/frame"
)

// Plane is the direction of a view reconstructed through a stack of frames.
type Plane int

const (
	// Coronal fixes a row and shows every column of that row from each frame.
	Coronal Plane = iota
	// Sagittal fixes a column and shows every row of that column from each frame.
	Sagittal
)

// Projection is an image.Image reconstructed through a stack of frames (slices).
// It takes a single row (Coronal) or column (Sagittal) from every frame and
// stacks them in frame order, with frame 0 at the top of the image.
type Projection struct {
	window

	frames []*frame.NativeFrame
	plane  Plane
	index  int
	ratio  float64
}

// NewProjection creates a projection of the given plane through frames.
// The projected row or column starts at the centre of the frames.
func NewProjection(frames []*frame.NativeFrame, plane Plane, level, width int16) *Projection {
	p := &Projection{plane: plane, ratio: 1, window: window{level: level, width: width}}
	p.SetFrames(frames)
	return p
}

// Plane returns the direction of this projection.
func (p *Projection) Plane() Plane {
	return p.plane
}

// SetFrames replaces the stack of frames that this projection is built from
// and moves the projected row or column back to the centre.
func (p *Projection) SetFrames(frames []*frame.NativeFrame) {
	p.frames = frames
	if len(frames) == 0 {
		p.index = 0
		return
	}
	if p.plane == Coronal {
		p.index = frames[0].Rows / 2
	} else {
		p.index = frames[0].Cols / 2
	}
}

// Index returns the row (Coronal) or column (Sagittal) of each frame that is shown.
func (p *Projection) Index() int {
	return p.index
}

// SetIndex chooses the row (Coronal) or column (Sagittal) of each frame to show.
// The value is clamped to the size of the frames.
func (p *Projection) SetIndex(i int) {
	if len(p.frames) == 0 {
		p.index = 0
		return
	}
	limit := p.frames[0].Rows
	if p.plane == Sagittal {
		limit = p.frames[0].Cols
	}
	p.index = max(0, min(i, limit-1))
}

// SliceRatio returns how many pixels tall each frame is drawn in the projection.
func (p *Projection) SliceRatio() float64 {
	return p.ratio
}

// SetSliceRatio sets how tall each frame is drawn relative to a pixel within a frame,
// so that the projection has the right aspect ratio. This is normally the spacing between
// slices divided by the pixel spacing. Values of zero or less are treated as one.
func (p *Projection) SetSliceRatio(r float64) {
	if r <= 0 || math.IsNaN(r) || math.IsInf(r, 0) {
		r = 1
	}
	p.ratio = r
}

func (p *Projection) ColorModel() color.Model {
	return color.Gray16Model
}

func (p *Projection) Bounds() image.Rectangle {
	if len(p.frames) == 0 {
		return image.Rectangle{}
	}
	w := p.frames[0].Cols
	if p.plane == Sagittal {
		w = p.frames[0].Rows
	}
	h := int(math.Round(float64(len(p.frames)) * p.ratio))
	return image.Rect(0, 0, w, max(h, 1))
}

func (p *Projection) At(x, y int) color.Color {
	if len(p.frames) == 0 || x < 0 || y < 0 {
		return color.Gray16{Y: 0}
	}

	z := int(float64(y) / p.ratio)
	if z >= len(p.frames) {
		return color.Gray16{Y: 0}
	}
	f := p.frames[z]

	var i int
	if p.plane == Coronal {
		if x >= f.Cols {
			return color.Gray16{Y: 0}
		}
		i = p.index*f.Cols + x
	} else {
		if x >= f.Rows {
			return color.Gray16{Y: 0}
		}
		i = x*f.Cols + p.index
	}
	if i < 0 || i >= len(f.Data) {
		return color.Gray16{Y: 0}
	}
	return p.gray(int16(f.Data[i][0]))
}
