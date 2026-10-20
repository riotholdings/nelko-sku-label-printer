package imaging

import (
	"fmt"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/math/fixed"
	"image"
	"image/color"
	"image/draw"
)

// RenderSKU places the date above a large sequence on the long edge of a
// narrow P21 label, then rotates it into the printer's native feed direction.
func RenderSKU(date string, sequence int, width, height int) (image.Image, error) {
	if width < 40 || height < 100 || sequence < 1 {
		return nil, fmt.Errorf("invalid SKU label dimensions or sequence")
	}
	f, err := truetype.Parse(gobold.TTF)
	if err != nil {
		return nil, err
	}
	img := image.NewRGBA(image.Rect(0, 0, height, width))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	texts := []string{date, fmt.Sprintf("%04d", sequence)}
	for i, text := range texts {
		top, bottom := 4, width/3
		if i == 1 {
			top = width / 3
			bottom = width - 4
		}
		for size := float64(26); size >= 3; size -= 0.5 {
			face := truetype.NewFace(f, &truetype.Options{Size: size, DPI: 203, Hinting: font.HintingFull})
			metrics := face.Metrics()
			w := font.MeasureString(face, text).Ceil()
			h := (metrics.Ascent + metrics.Descent).Ceil()
			if w <= height-12 && h <= bottom-top {
				d := font.Drawer{Dst: img, Src: image.NewUniform(color.Black), Face: face, Dot: fixed.P((height-w)/2, top+(bottom-top-h)/2+metrics.Ascent.Ceil())}
				d.DrawString(text)
				face.Close()
				break
			}
			face.Close()
			if size == 3 {
				return nil, fmt.Errorf("SKU text does not fit this label")
			}
		}
	}
	return rotate90CW(img), nil
}
