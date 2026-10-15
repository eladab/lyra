package preview

import (
	"context"
	"image"
	"math"
)

// contentArea measures a low-resolution page once, then maps those bounds to
// sharper rasters at the requested zoom. Keep all printed content, including
// footers, lyrics and dynamics; fitting its WIDTH avoids a footer shrinking the
// music to fit an entire sheet. Vertical content remains reachable by scrolling.
type contentArea struct {
	ink, page image.Rectangle
}

func measureContent(ctx context.Context, img image.Image) (contentArea, error) {
	page := img.Bounds()
	ink := image.Rectangle{Min: page.Max, Max: page.Min}
	for y := page.Min.Y; y < page.Max.Y; y++ {
		if err := ctx.Err(); err != nil {
			return contentArea{}, err
		}
		for x := page.Min.X; x < page.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			// Composite transparency over white before deciding whether this
			// pixel is ink. Near-white antialiasing fringes are covered by padding.
			if r+0xffff-a < 0xfafa || g+0xffff-a < 0xfafa || b+0xffff-a < 0xfafa {
				ink.Min.X, ink.Min.Y = min(ink.Min.X, x), min(ink.Min.Y, y)
				ink.Max.X, ink.Max.Y = max(ink.Max.X, x+1), max(ink.Max.Y, y+1)
			}
		}
	}
	if ink.Empty() {
		return contentArea{ink: page, page: page}, nil
	}
	// A small breathing room scales with the score rather than the paper size.
	padding := max(2, int(math.Ceil(float64(ink.Dx())*.025)))
	return contentArea{ink: ink.Inset(-padding).Intersect(page), page: page}, nil
}

func (a contentArea) bounds(page image.Rectangle) image.Rectangle {
	xscale := float64(page.Dx()) / float64(a.page.Dx())
	yscale := float64(page.Dy()) / float64(a.page.Dy())
	return image.Rect(
		page.Min.X+int(math.Floor(float64(a.ink.Min.X-a.page.Min.X)*xscale)),
		page.Min.Y+int(math.Floor(float64(a.ink.Min.Y-a.page.Min.Y)*yscale)),
		page.Min.X+int(math.Ceil(float64(a.ink.Max.X-a.page.Min.X)*xscale)),
		page.Min.Y+int(math.Ceil(float64(a.ink.Max.Y-a.page.Min.Y)*yscale)),
	).Intersect(page)
}
