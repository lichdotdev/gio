// SPDX-License-Identifier: Unlicense OR MIT

package opengl

import (
	"log"
	"strings"
	"sync"
)

var softwareRendererOnce sync.Once

// reportSoftwareRenderer logs, once per process, that the window draws on
// the CPU: a slow window then has an explanation in its log.
func reportSoftwareRenderer(name string) {
	if softwareRenderer(name) {
		softwareRendererOnce.Do(func() {
			log.Printf("gio: no GPU acceleration, the window draws on the CPU (GL_RENDERER %q)", name)
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
