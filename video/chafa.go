package video

import (
	"fmt"
	"os"

	chafa "github.com/ploMP4/chafa-go"
)

const symbolSelector = "block+border+space"

type ChafaConfig struct {
	WorkFactor float32
	CanvasMode chafa.CanvasMode
	ColorSpace chafa.ColorSpace
	DitherMode chafa.DitherMode
}

var defaultChafaConfig = ChafaConfig{
	WorkFactor: 0.5,
	CanvasMode: chafa.CHAFA_CANVAS_MODE_TRUECOLOR,
	ColorSpace: chafa.CHAFA_COLOR_SPACE_DIN99D,
	DitherMode: chafa.CHAFA_DITHER_MODE_DIFFUSION,
}

// Render converts raw RGBA pixels into terminal ASCII art.
// pixW/pixH are the source pixel dimensions; termW/termH are the target character dimensions.
func (c ChafaConfig) Render(pixels []byte, pixW, pixH, termW, termH int) (string, error) {
	if len(pixels) != pixW*pixH*rgbaChannels {
		return "", fmt.Errorf("pixel buffer size mismatch: got %d, want %d", len(pixels), pixW*pixH*rgbaChannels)
	}

	symbolMap := chafa.SymbolMapNew()
	defer chafa.SymbolMapUnref(symbolMap)
	chafa.SymbolMapApplySelectors(symbolMap, symbolSelector)

	canvasMode := c.CanvasMode
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		canvasMode = chafa.CHAFA_CANVAS_MODE_FGBG_BGFG
	}

	cfg := chafa.CanvasConfigNew()
	defer chafa.CanvasConfigUnref(cfg)
	chafa.CanvasConfigSetGeometry(cfg, int32(termW), int32(termH))
	chafa.CanvasConfigSetSymbolMap(cfg, symbolMap)
	chafa.CanvasConfigSetPixelMode(cfg, chafa.CHAFA_PIXEL_MODE_SYMBOLS)
	chafa.CanvasConfigSetCanvasMode(cfg, canvasMode)
	chafa.CanvasConfigSetColorSpace(cfg, c.ColorSpace)
	chafa.CanvasConfigSetDitherMode(cfg, c.DitherMode)
	chafa.CanvasConfigSetWorkFactor(cfg, c.WorkFactor)

	canvas := chafa.CanvasNew(cfg)
	defer chafa.CanvasUnRef(canvas)

	chafa.CanvasDrawAllPixels(
		canvas,
		chafa.CHAFA_PIXEL_RGBA8_UNASSOCIATED,
		pixels,
		int32(pixW), int32(pixH), int32(pixW*rgbaChannels),
	)

	gs := chafa.CanvasPrint(canvas, nil)
	return gs.String(), nil
}
