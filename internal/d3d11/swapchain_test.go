// SPDX-License-Identifier: Unlicense OR MIT

package d3d11

import (
	"errors"
	"testing"
)

func TestFlipConfigsAreValidForFlipModel(t *testing.T) {
	flips := 0
	for _, cfg := range swapChainConfigs {
		if cfg.effect != DXGI_SWAP_EFFECT_FLIP_DISCARD {
			continue
		}
		flips++
		if cfg.bufferCount < 2 {
			t.Errorf("flip model needs at least 2 buffers, got %d", cfg.bufferCount)
		}
		if cfg.format == DXGI_FORMAT_R8G8B8A8_UNORM_SRGB {
			t.Error("flip-model buffers cannot have an _SRGB format")
		}
	}
	if flips == 0 {
		t.Error("no flip-model config")
	}
}

func TestFirstSwapChainPrefersFlip(t *testing.T) {
	var tried []uint32
	err := firstSwapChain(func(cfg swapChainConfig) error {
		tried = append(tried, cfg.effect)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(tried) != 1 || tried[0] != DXGI_SWAP_EFFECT_FLIP_DISCARD {
		t.Fatalf("tried %v, want only the flip model", tried)
	}
}

func TestFirstSwapChainFallsBackToDiscard(t *testing.T) {
	var last swapChainConfig
	err := firstSwapChain(func(cfg swapChainConfig) error {
		last = cfg
		if cfg.effect == DXGI_SWAP_EFFECT_FLIP_DISCARD {
			return errors.New("no flip model here")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := swapChainConfig{DXGI_SWAP_EFFECT_DISCARD, DXGI_FORMAT_R8G8B8A8_UNORM_SRGB, 1}
	if last != want {
		t.Fatalf("fell back to %+v, want %+v", last, want)
	}
}

func TestFirstSwapChainReportsLastError(t *testing.T) {
	want := errors.New("no swap chain")
	err := firstSwapChain(func(swapChainConfig) error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("got %v, want %v", err, want)
	}
}
