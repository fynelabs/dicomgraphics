package main

import (
	"image"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// volumeShaderBody ray casts the three orthogonal planes of a volume, each textured
// with its slice image, meeting at the crosshair point that the camera orbits.
// The volume is a box of half extents (halfX, halfY, halfZ) in world units where
// x runs along the columns, y up through the slices (slice 0 at the top) and z along
// the rows. The cut uniforms give the plane positions as a fraction of each axis.
const volumeShaderBody = `
uniform vec2 frame;
uniform vec4 bounds; //x1 [0], y1 [1], x2 [2], y2 [3]; coords in the frame

uniform float halfX;
uniform float halfY;
uniform float halfZ;
uniform float cutX;
uniform float cutY;
uniform float cutZ;
uniform float yaw;
uniform float pitch;
uniform float dist;
uniform float lineR;
uniform float lineG;
uniform float lineB;

uniform sampler2D axial;
uniform sampler2D coronal;
uniform sampler2D sagittal;

mat3 rotX(float a) {
    float c = cos(a);
    float s = sin(a);
    return mat3(1.0, 0.0, 0.0, 0.0, c, s, 0.0, -s, c);
}

mat3 rotY(float a) {
    float c = cos(a);
    float s = sin(a);
    return mat3(c, 0.0, -s, 0.0, 1.0, 0.0, s, 0.0, c);
}

void main() {
    vec2 size = vec2(bounds[2] - bounds[0], bounds[3] - bounds[1]);
    vec2 center = vec2((bounds[0] + bounds[2]) * 0.5, frame.y - (bounds[1] + bounds[3]) * 0.5);
    vec2 uv = (gl_FragCoord.xy - center) / (0.5 * min(size.x, size.y));

    vec3 target = vec3(-halfX + cutX * 2.0 * halfX, halfY - cutY * 2.0 * halfY, -halfZ + cutZ * 2.0 * halfZ);
    mat3 rot = rotY(yaw) * rotX(pitch);
    vec3 ro = target + rot * vec3(0.0, 0.0, -dist);
    vec3 rd = normalize(rot * vec3(uv, 2.0));

    float best = 1e9;
    int which = -1;
    vec3 hp = vec3(0.0);

    if (abs(rd.x) > 1e-6) {
        float t = (target.x - ro.x) / rd.x;
        vec3 p = ro + rd * t;
        if (t > 0.0 && t < best && abs(p.y) <= halfY && abs(p.z) <= halfZ) {
            best = t; which = 0; hp = p;
        }
    }
    if (abs(rd.y) > 1e-6) {
        float t = (target.y - ro.y) / rd.y;
        vec3 p = ro + rd * t;
        if (t > 0.0 && t < best && abs(p.x) <= halfX && abs(p.z) <= halfZ) {
            best = t; which = 1; hp = p;
        }
    }
    if (abs(rd.z) > 1e-6) {
        float t = (target.z - ro.z) / rd.z;
        vec3 p = ro + rd * t;
        if (t > 0.0 && t < best && abs(p.x) <= halfX && abs(p.y) <= halfY) {
            best = t; which = 2; hp = p;
        }
    }
    if (which < 0) {
        discard;
    }

    vec3 col;
    vec3 n;
    if (which == 0) {
        col = texture2D(sagittal, vec2((hp.z + halfZ) / (2.0 * halfZ), (halfY - hp.y) / (2.0 * halfY))).rgb;
        n = vec3(1.0, 0.0, 0.0);
    } else if (which == 1) {
        col = texture2D(axial, vec2((hp.x + halfX) / (2.0 * halfX), (hp.z + halfZ) / (2.0 * halfZ))).rgb;
        n = vec3(0.0, 1.0, 0.0);
    } else {
        col = texture2D(coronal, vec2((hp.x + halfX) / (2.0 * halfX), (halfY - hp.y) / (2.0 * halfY))).rgb;
        n = vec3(0.0, 0.0, 1.0);
    }
    col *= 0.45 + 0.55 * abs(dot(rd, n));

    float lw = 0.005;
    if ((which != 0 && abs(hp.x - target.x) < lw) ||
        (which != 1 && abs(hp.y - target.y) < lw) ||
        (which != 2 && abs(hp.z - target.z) < lw)) {
        col = vec3(lineR, lineG, lineB);
    }
    gl_FragColor = vec4(col, 1.0);
}
`

const volumeShaderSource = "#version 110\n" + volumeShaderBody

const volumeShaderSourceES = `#version 100

#ifdef GL_ES
# ifdef GL_FRAGMENT_PRECISION_HIGH
precision highp float;
# else
precision mediump float;
#endif
precision mediump int;
#endif
` + volumeShaderBody

// volumeView draws the three planes of the loaded volume in 3D, meeting at the
// crosshair point. Dragging orbits the camera and scrolling zooms.
type volumeView struct {
	widget.BaseWidget

	shader *canvas.Shader
}

func newVolumeView() *volumeView {
	v := &volumeView{shader: canvas.NewShader("dicomVolume", []byte(volumeShaderSource), []byte(volumeShaderSourceES))}
	v.shader.Uniforms = map[string]float32{
		"halfX": 0.5, "halfY": 0.5, "halfZ": 0.5,
		"cutX": 0.5, "cutY": 0.5, "cutZ": 0.5,
		"yaw": 0.6, "pitch": 0.45, "dist": 1.6,
		"lineR": 1, "lineG": 0.75, "lineB": 0.5,
	}
	r, g, b, _ := theme.Color(theme.ColorNamePrimary).RGBA()
	v.shader.Uniforms["lineR"] = float32(r) / 0xffff
	v.shader.Uniforms["lineG"] = float32(g) / 0xffff
	v.shader.Uniforms["lineB"] = float32(b) / 0xffff
	v.shader.Textures = map[string]image.Image{}
	v.ExtendBaseWidget(v)
	return v
}

func (v *volumeView) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(v.shader)
}

func (v *volumeView) Dragged(ev *fyne.DragEvent) {
	v.shader.Uniforms["yaw"] += ev.Dragged.DX * 0.01
	v.shader.Uniforms["pitch"] = max(-1.5, min(1.5, v.shader.Uniforms["pitch"]+ev.Dragged.DY*0.01))
	v.shader.Refresh()
}

func (v *volumeView) DragEnd() {}

func (v *volumeView) Scrolled(ev *fyne.ScrollEvent) {
	dist := v.shader.Uniforms["dist"]
	if ev.Scrolled.DY > 0 {
		dist *= 0.9
	} else if ev.Scrolled.DY < 0 {
		dist /= 0.9
	}
	v.shader.Uniforms["dist"] = max(0.5, min(5, dist))
	v.shader.Refresh()
}

// setExtents sizes the volume from its pixel counts and spacing, scaled so that the
// largest dimension is one world unit.
func (v *volumeView) setExtents(cols, rows, slices int, rowSpacing, colSpacing, sliceSpacing float64) {
	x := float64(cols) * colSpacing
	y := float64(slices) * sliceSpacing
	z := float64(rows) * rowSpacing
	m := max(x, y, z)
	if m <= 0 {
		return
	}
	v.shader.Uniforms["halfX"] = float32(x / m / 2)
	v.shader.Uniforms["halfY"] = float32(y / m / 2)
	v.shader.Uniforms["halfZ"] = float32(z / m / 2)
}

// setCut moves the point where the planes meet to the centre of the given column,
// row and slice.
func (v *volumeView) setCut(col, row, slice, cols, rows, slices int) {
	if cols == 0 || rows == 0 || slices == 0 {
		return
	}
	v.shader.Uniforms["cutX"] = (float32(col) + 0.5) / float32(cols)
	v.shader.Uniforms["cutZ"] = (float32(row) + 0.5) / float32(rows)
	v.shader.Uniforms["cutY"] = (float32(slice) + 0.5) / float32(slices)
}

// setPlane replaces the texture of one plane with a snapshot of img. The plane is
// "axial", "coronal" or "sagittal".
func (v *volumeView) setPlane(plane string, img image.Image) {
	v.shader.Textures[plane] = snapshot(img)
}

// snapshot copies an image into an RGBA buffer so it can be uploaded as a texture.
func snapshot(img image.Image) *image.RGBA {
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			g := color.Gray16Model.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(color.Gray16)
			v := uint8(g.Y >> 8)
			i := out.PixOffset(x, y)
			out.Pix[i], out.Pix[i+1], out.Pix[i+2], out.Pix[i+3] = v, v, v, 0xff
		}
	}
	return out
}
