//go:build (linux || freebsd) && !novulkan

package vulkan

import (
	"errors"
	"fmt"
	"testing"

	"gioui.org/internal/vk"
)

func TestALostDeviceDoesNotPanic(t *testing.T) {
	panicUnlessLost(fmt.Errorf("vulkan: vkQueueSubmit: %w", vk.ERROR_DEVICE_LOST))
}

func TestAnotherVulkanErrorPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("an error other than a lost device did not panic")
		}
	}()
	panicUnlessLost(errors.New("vulkan: out of memory"))
}
