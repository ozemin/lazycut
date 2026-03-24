package video

/*
#cgo pkg-config: chafa
#include <chafa.h>
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"os"
	"unsafe"
)

const symbolSelector = "block+border+space"

type ChafaConfig struct {
	WorkFactor float32
	CanvasMode C.ChafaCanvasMode
	ColorSpace C.ChafaColorSpace
	DitherMode C.ChafaDitherMode
}

var defaultChafaConfig = ChafaConfig{
	WorkFactor: 0.5,
	CanvasMode: C.CHAFA_CANVAS_MODE_TRUECOLOR,
	ColorSpace: C.CHAFA_COLOR_SPACE_DIN99D,
	DitherMode: C.CHAFA_DITHER_MODE_DIFFUSION,
}

func (c ChafaConfig) Render(pixels []byte, pixW, pixH, termW, termH int) (string, error) {
	if len(pixels) != pixW*pixH*rgbaChannels {
		return "", fmt.Errorf("pixel buffer size mismatch: got %d, want %d", len(pixels), pixW*pixH*rgbaChannels)
	}

	sm := C.chafa_symbol_map_new()
	defer C.chafa_symbol_map_unref(sm)
	selectors := C.CString(symbolSelector)
	defer C.free(unsafe.Pointer(selectors))
	C.chafa_symbol_map_apply_selectors(sm, selectors, nil)

	canvasMode := c.CanvasMode
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		canvasMode = C.CHAFA_CANVAS_MODE_FGBG_BGFG
	}

	cfg := C.chafa_canvas_config_new()
	defer C.chafa_canvas_config_unref(cfg)
	C.chafa_canvas_config_set_geometry(cfg, C.gint(termW), C.gint(termH))
	C.chafa_canvas_config_set_symbol_map(cfg, sm)
	C.chafa_canvas_config_set_pixel_mode(cfg, C.CHAFA_PIXEL_MODE_SYMBOLS)
	C.chafa_canvas_config_set_canvas_mode(cfg, canvasMode)
	C.chafa_canvas_config_set_color_space(cfg, c.ColorSpace)
	C.chafa_canvas_config_set_dither_mode(cfg, c.DitherMode)
	C.chafa_canvas_config_set_work_factor(cfg, C.gfloat(c.WorkFactor))

	cv := C.chafa_canvas_new(cfg)
	defer C.chafa_canvas_unref(cv)

	C.chafa_canvas_draw_all_pixels(
		cv,
		C.CHAFA_PIXEL_RGBA8_UNASSOCIATED,
		(*C.guint8)(unsafe.Pointer(&pixels[0])),
		C.gint(pixW), C.gint(pixH), C.gint(pixW*rgbaChannels),
	)

	gs := C.chafa_canvas_print(cv, nil)
	defer C.g_string_free(gs, C.TRUE)
	return C.GoString(gs.str), nil
}
