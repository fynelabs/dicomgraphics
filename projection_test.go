package dicomgraphics

import (
	"image/color"
	"testing"

	"github.com/suyashkumar/dicom/pkg/frame"
)

// testFrames builds n frames of rows x cols where each pixel value encodes (slice, row, col).
func testFrames(n, rows, cols int) []*frame.NativeFrame {
	frames := make([]*frame.NativeFrame, n)
	for z := range frames {
		data := make([][]int, rows*cols)
		for y := 0; y < rows; y++ {
			for x := 0; x < cols; x++ {
				data[y*cols+x] = []int{z*100 + y*10 + x}
			}
		}
		frames[z] = &frame.NativeFrame{Data: data, Rows: rows, Cols: cols, BitsPerSample: 16}
	}
	return frames
}

// identity window: level 500, width 1000 maps raw 0..999 to 0..0xffff linearly
func rawOf(c color.Color) int {
	g := c.(color.Gray16)
	return int(float32(g.Y)/float32(0xffff)*1000 + 0.5)
}

func TestProjectionBounds(t *testing.T) {
	frames := testFrames(4, 6, 8)

	cor := NewProjection(frames, Coronal, 500, 1000)
	if b := cor.Bounds(); b.Dx() != 8 || b.Dy() != 4 {
		t.Errorf("coronal bounds = %v, want 8x4", b)
	}
	sag := NewProjection(frames, Sagittal, 500, 1000)
	if b := sag.Bounds(); b.Dx() != 6 || b.Dy() != 4 {
		t.Errorf("sagittal bounds = %v, want 6x4", b)
	}

	sag.SetSliceRatio(2.5)
	if b := sag.Bounds(); b.Dy() != 10 {
		t.Errorf("scaled sagittal height = %d, want 10", b.Dy())
	}
	if NewProjection(nil, Coronal, 0, 1).Bounds().Dx() != 0 {
		t.Error("empty projection should have empty bounds")
	}
}

func TestProjectionAt(t *testing.T) {
	frames := testFrames(4, 6, 8)

	cor := NewProjection(frames, Coronal, 500, 1000)
	if cor.Index() != 3 {
		t.Errorf("default coronal row = %d, want 3", cor.Index())
	}
	cor.SetIndex(2)
	// pixel (x=5, y=3) should be slice 3, row 2, col 5
	if got := rawOf(cor.At(5, 3)); got != 325 {
		t.Errorf("coronal At(5,3) = %d, want 325", got)
	}

	sag := NewProjection(frames, Sagittal, 500, 1000)
	if sag.Index() != 4 {
		t.Errorf("default sagittal col = %d, want 4", sag.Index())
	}
	sag.SetIndex(7)
	// pixel (x=4, y=1) should be slice 1, row 4, col 7
	if got := rawOf(sag.At(4, 1)); got != 147 {
		t.Errorf("sagittal At(4,1) = %d, want 147", got)
	}

	sag.SetSliceRatio(2)
	// y=5 maps to slice 2
	if got := rawOf(sag.At(0, 5)); got != 207 {
		t.Errorf("scaled sagittal At(0,5) = %d, want 207", got)
	}

	// out of range is black, not a panic
	for _, c := range []color.Color{cor.At(-1, 0), cor.At(8, 0), cor.At(0, 4), sag.At(6, 0), sag.At(0, 99)} {
		if c.(color.Gray16).Y != 0 {
			t.Errorf("out of range pixel = %v, want black", c)
		}
	}
}

func TestProjectionSetIndexClamps(t *testing.T) {
	frames := testFrames(2, 6, 8)
	cor := NewProjection(frames, Coronal, 500, 1000)
	cor.SetIndex(-5)
	if cor.Index() != 0 {
		t.Errorf("negative index = %d, want 0", cor.Index())
	}
	cor.SetIndex(50)
	if cor.Index() != 5 {
		t.Errorf("oversized row index = %d, want 5", cor.Index())
	}
	sag := NewProjection(frames, Sagittal, 500, 1000)
	sag.SetIndex(50)
	if sag.Index() != 7 {
		t.Errorf("oversized col index = %d, want 7", sag.Index())
	}
}
