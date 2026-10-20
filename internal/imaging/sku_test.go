package imaging

import (
	"image/color"
	"nelko-print/internal/tspl"
	"testing"
)

func TestSKURenderSizes(t *testing.T) {
	for _, size := range tspl.AllSizes {
		img, err := RenderSKU("2026-10-20", 10000, size.PixelW, size.PixelH)
		if err != nil {
			t.Fatal(err)
		}
		if img.Bounds().Dx() != size.PixelW || img.Bounds().Dy() != size.PixelH {
			t.Fatal(img.Bounds())
		}
		dark := 0
		for y := 0; y < size.PixelH; y++ {
			for x := 0; x < size.PixelW; x++ {
				if color.GrayModel.Convert(img.At(x, y)).(color.Gray).Y < 128 {
					dark++
				}
			}
		}
		if dark < 100 || dark > size.PixelW*size.PixelH/2 {
			t.Fatal("blank or inverted label", dark)
		}
		bitmap := ToMonochrome(img, size.PixelW, size.PixelH, 128, true)
		if len(bitmap) != size.PixelW/8*size.PixelH {
			t.Fatal("invalid bitmap size")
		}
	}
}
