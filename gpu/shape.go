// SPDX-License-Identifier: Unlicense OR MIT

package gpu

import (
	_ "embed"
	"unsafe"

	"gioui.org/gpu/internal/driver"
	"gioui.org/internal/byteslice"
	"gioui.org/internal/f32"
	"gioui.org/internal/f32color"
	"gioui.org/internal/ops"
	"gioui.org/shader"
	"gioui.org/shader/gio"
)

// shapeKind is how a clip that clip.RRect or clip.Arc made is drawn as one
// quad whose fragment shader computes the coverage, with no stencil pass.
type shapeKind uint8

const (
	shapeNone shapeKind = iota
	shapeFill
	shapeBorder
	shapeArc
)

// shapeOp is a shape in window pixels, laid out as shape.frag reads it.
type shapeOp struct {
	kind shapeKind
	// geom is a rounded rectangle's center and half size, or an arc's
	// center, radius and half width.
	geom [4]float32
	// radii are a rounded rectangle's SE, NE, SW and NW radii, or an arc's
	// start and sweep.
	radii [4]float32
	// halfWidth is a border's.
	halfWidth float32
}

// shaper draws shapeOps; nil where the device cannot run shape.frag.
type shaper struct {
	pipelines [2]*pipeline
	uniforms  *shapeUniforms
}

type shapeUniforms struct {
	blitUniforms
	color f32color.RGBA
	geom  [4]float32
	radii [4]float32
	style [4]float32
}

var (
	//go:embed shape.frag.glsl150
	shapeFragGLSL150 string
	//go:embed shape.frag.glsl100es
	shapeFragGLSL100ES string
)

// shapeFrag ships only the GLSL variants: the other backends draw these
// clips as paths.
var shapeFrag = shader.Sources{
	Name: "shape.frag",
	Inputs: []shader.InputLocation{
		{Name: "vUV", Location: 0, Semantic: "TEXCOORD", SemanticIndex: 0, Type: shader.DataTypeFloat, Size: 2},
		{Name: "opacity", Location: 1, Semantic: "TEXCOORD", SemanticIndex: 1, Type: shader.DataTypeFloat, Size: 1},
	},
	Uniforms: shader.UniformsReflection{
		Locations: []shader.UniformLocation{
			{Name: "_shape.color", Type: shader.DataTypeFloat, Size: 4, Offset: int(unsafe.Offsetof(shapeUniforms{}.color))},
			{Name: "_shape.geom", Type: shader.DataTypeFloat, Size: 4, Offset: int(unsafe.Offsetof(shapeUniforms{}.geom))},
			{Name: "_shape.radii", Type: shader.DataTypeFloat, Size: 4, Offset: int(unsafe.Offsetof(shapeUniforms{}.radii))},
			{Name: "_shape.style", Type: shader.DataTypeFloat, Size: 4, Offset: int(unsafe.Offsetof(shapeUniforms{}.style))},
		},
		Size: int(unsafe.Sizeof(shapeUniforms{}) - unsafe.Sizeof(blitUniforms{})),
	},
	GLSL150:   shapeFragGLSL150,
	GLSL100ES: shapeFragGLSL100ES,
}

// newShaper returns nil off OpenGL, the one backend shapeFrag has source
// for; Caps tells OpenGL apart by its bottom left origin.
func newShaper(ctx driver.Device) *shaper {
	if !ctx.Caps().BottomLeftOrigin {
		return nil
	}
	s := &shaper{uniforms: new(shapeUniforms)}
	vsh, fsh, err := newShaders(ctx, gio.Shader_blit_vert, shapeFrag)
	if err != nil {
		return nil
	}
	defer vsh.Release()
	defer fsh.Release()
	for i, format := range []driver.TextureFormat{driver.TextureFormatOutput, driver.TextureFormatSRGBA} {
		pipe, err := ctx.NewPipeline(driver.PipelineDesc{
			VertexShader:   vsh,
			FragmentShader: fsh,
			BlendDesc:      driver.BlendDesc{SrcFactor: driver.BlendFactorOne, DstFactor: driver.BlendFactorOneMinusSrcAlpha},
			VertexLayout:   blitLayout,
			PixelFormat:    format,
			Topology:       driver.TopologyTriangleStrip,
		})
		if err != nil {
			s.release()
			return nil
		}
		s.pipelines[i] = &pipeline{pipe, newUniformBuffer(ctx, byteslice.View(s.uniforms))}
	}
	return s
}

func (s *shaper) release() {
	if s == nil {
		return
	}
	for _, p := range s.pipelines {
		if p != nil {
			p.Release()
		}
	}
}

// draw covers img's clip, offset by opOff, with img's shape in its color.
func (s *shaper) draw(ctx driver.Device, quad driver.Buffer, img imageOp, fbo bool, scale, off f32.Point) {
	fboIdx := 0
	if fbo {
		fboIdx = 1
	}
	p := s.pipelines[fboIdx]
	ctx.BindPipeline(p.pipeline)
	ctx.BindVertexBuffer(quad, 0)
	u := s.uniforms
	u.transform = [4]float32{scale.X, scale.Y, off.X, off.Y}
	// The quad's uv runs over the clip, so the shader sees window pixels.
	min, size := img.clip.Min, img.clip.Size()
	u.uvTransformR1 = [4]float32{float32(size.X), 0, float32(min.X), 0}
	u.uvTransformR2 = [4]float32{0, float32(size.Y), float32(min.Y), 0}
	u.opacity = img.material.opacity
	u.fbo = 0
	if fbo {
		u.fbo = 1
	}
	u.color = img.material.color
	u.geom, u.radii = img.shape.geom, img.shape.radii
	u.style = [4]float32{float32(img.shape.kind - shapeFill), img.shape.halfWidth}
	p.UploadUniforms(ctx)
	ctx.DrawArrays(0, 4)
}

// shapeFor returns the shape a clip of the given shape and args covers
// under the transform t, plus its bounds in t's space. It reports false
// where the quad would not draw what the path draws: a transform that
// rotates, shears or stretches, or radii the rectangle cannot hold.
func shapeFor(shape ops.Shape, args [ops.ShapeArgs]float32, outline bool, width float32, t f32.Affine2D) (shapeOp, f32.Rectangle, bool) {
	sx, hx, ox, hy, sy, oy := t.Elems()
	if hx != 0 || hy != 0 || sx != sy || sx <= 0 {
		return shapeOp{}, f32.Rectangle{}, false
	}
	at := func(x, y float32) f32.Point { return f32.Pt(x*sx+ox, y*sx+oy) }
	hw := width * .5 * sx
	switch {
	case shape == ops.RRect && (outline || width > 0):
		w, n, e, s, se, sw, nw, ne := args[0], args[1], args[2], args[3], args[4], args[5], args[6], args[7]
		if !(e > w && s > n && se >= 0 && sw >= 0 && nw >= 0 && ne >= 0 &&
			nw+ne <= e-w && sw+se <= e-w && nw+sw <= s-n && ne+se <= s-n) {
			return shapeOp{}, f32.Rectangle{}, false
		}
		r := f32.Rectangle{Min: at(w, n), Max: at(e, s)}
		c, half := r.Min.Add(r.Max).Mul(.5), r.Size().Mul(.5)
		op := shapeOp{kind: shapeFill, geom: [4]float32{c.X, c.Y, half.X, half.Y}, radii: [4]float32{se * sx, ne * sx, sw * sx, nw * sx}}
		if width > 0 {
			op.kind, op.halfWidth = shapeBorder, hw
			r = outset(r, hw)
		}
		return op, r, true
	case shape == ops.Arc && !outline && width > 0:
		radius, start, sweep := args[2]*sx, args[3], args[4]
		if sweep < 0 {
			start, sweep = start+sweep, -sweep
		}
		if sweep == 0 || radius <= 0 {
			return shapeOp{}, f32.Rectangle{}, false
		}
		c := at(args[0], args[1])
		op := shapeOp{kind: shapeArc, geom: [4]float32{c.X, c.Y, radius, hw}, radii: [4]float32{start, sweep}}
		return op, outset(f32.Rectangle{Min: c, Max: c}, radius+hw), true
	}
	return shapeOp{}, f32.Rectangle{}, false
}

func outset(r f32.Rectangle, d float32) f32.Rectangle {
	return f32.Rectangle{Min: r.Min.Sub(f32.Pt(d, d)), Max: r.Max.Add(f32.Pt(d, d))}
}
