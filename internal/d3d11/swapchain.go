// SPDX-License-Identifier: Unlicense OR MIT

package d3d11

const (
	DXGI_FORMAT_R8G8B8A8_UNORM      = 28
	DXGI_FORMAT_R8G8B8A8_UNORM_SRGB = 29

	DXGI_SWAP_EFFECT_DISCARD      = 0
	DXGI_SWAP_EFFECT_FLIP_DISCARD = 4

	maxFrameLatency = 1
)

type swapChainConfig struct {
	effect, format, bufferCount uint32
}

// swapChainConfigs are tried in order. Windows before 10, and some VMs and
// drivers, cannot create a flip-model swap chain.
var swapChainConfigs = [...]swapChainConfig{
	{DXGI_SWAP_EFFECT_FLIP_DISCARD, DXGI_FORMAT_R8G8B8A8_UNORM, 2},
	{DXGI_SWAP_EFFECT_DISCARD, DXGI_FORMAT_R8G8B8A8_UNORM_SRGB, 1},
}

func firstSwapChain(create func(swapChainConfig) error) error {
	var err error
	for _, cfg := range swapChainConfigs {
		if err = create(cfg); err == nil {
			return nil
		}
	}
	return err
}
