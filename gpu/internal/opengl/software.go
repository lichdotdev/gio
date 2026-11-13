// SPDX-License-Identifier: Unlicense OR MIT

package opengl

import (
	"log"
	"strings"
	"sync"

	"gioui.org/gpu/internal/driver"
)

var softwareRendererOnce sync.Once

// ReportSoftwareRenderer logs, once per process, that the window draws on
// the CPU: a slow window then has an explanation in its log. Only a window's
// device is reported: a headless one opens EGL_DEFAULT_DISPLAY, which glvnd
// can hand to Mesa's llvmpipe while the window itself runs on an NVIDIA GPU.
func ReportSoftwareRenderer(d driver.Device) {
	if b, ok := d.(*Backend); ok && softwareRenderer(b.renderer) {
		softwareRendererOnce.Do(func() {
			log.Printf("gio: no GPU acceleration, the window draws on the CPU (GL_RENDERER %q)", b.renderer)
		})
	}
}

func softwareRenderer(name string) bool {
	n := strings.ToLower(name)
	for _, s := range []string{"llvmpipe", "softpipe", "swrast", "software rasterizer", "swiftshader"} {
		if strings.Contains(n, s) {
			return true
		}
	}
	return false
}
