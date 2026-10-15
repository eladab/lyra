package preview

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"strings"
)

// Theme changes only the rendered preview. The engraved PDF remains untouched.
type Theme string

const (
	ThemeWarm  Theme = "warm"
	ThemeNight Theme = "night"
	ThemePaper Theme = "paper"
)

// ParseTheme accepts an empty value as the default warm paper.
func ParseTheme(value string) (Theme, error) {
	switch theme := Theme(strings.ToLower(strings.TrimSpace(value))); theme {
	case "", ThemeWarm:
		return ThemeWarm, nil
	case ThemeNight, ThemePaper:
		return theme, nil
	default:
		return "", fmt.Errorf("unknown preview theme %q: use warm, night or paper", value)
	}
}

func (t Theme) Next() Theme {
	switch t {
	case "", ThemeWarm:
		return ThemeNight
	case ThemeNight:
		return ThemePaper
	default:
		return ThemeWarm
	}
}

func (t Theme) Label() string {
	switch t {
	case ThemeNight:
		return "Night"
	case ThemePaper:
		return "Paper"
	default:
		return "Warm"
	}
}

func (t Theme) paper() color.RGBA {
	switch t {
	case ThemeNight:
		return color.RGBA{36, 42, 50, 255}
	case ThemePaper:
		return color.RGBA{255, 255, 255, 255}
	default:
		return color.RGBA{230, 223, 207, 255}
	}
}

// themeRaster runs on the raster worker after measuring the original page's
// margins. Poppler normally decodes into RGBA, which can be updated in place;
// other image types need one bounded replacement bitmap.
func themeRaster(ctx context.Context, src image.Image, theme Theme) (image.Image, error) {
	dst, ok := src.(*image.RGBA)
	if !ok {
		dst = image.NewRGBA(src.Bounds())
	}
	paper := theme.paper()
	for y := src.Bounds().Min.Y; y < src.Bounds().Max.Y; y++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for x := src.Bounds().Min.X; x < src.Bounds().Max.X; x++ {
			r, g, b, a := src.At(x, y).RGBA()
			// RGBA returns premultiplied channels. Composite onto white before
			// remapping so transparent ink and antialiasing keep their coverage.
			r8, g8, b8 := int((r+0xffff-a+128)/257), int((g+0xffff-a+128)/257), int((b+0xffff-a+128)/257)
			pixel := color.RGBA{uint8(r8), uint8(g8), uint8(b8), 255}
			switch theme {
			case ThemeWarm:
				pixel = color.RGBA{
					uint8(36 + (int(paper.R)-36)*r8/255),
					uint8(42 + (int(paper.G)-42)*g8/255),
					uint8(50 + (int(paper.B)-50)*b8/255), 255,
				}
			case ThemeNight:
				pixel = nightPixel(r8, g8, b8, paper)
			}
			i := dst.PixOffset(x, y)
			dst.Pix[i], dst.Pix[i+1], dst.Pix[i+2], dst.Pix[i+3] = pixel.R, pixel.G, pixel.B, pixel.A
		}
	}
	return dst, nil
}

func nightPixel(r, g, b int, paper color.RGBA) color.RGBA {
	// Separate an ink color from its coverage over white. Unlike RGB inversion,
	// this keeps a red annotation red and gives partially covered pixels the
	// same smooth edge against dark paper. Lift saturated colors for legibility.
	minimum := min(r, g, b)
	coverage := 255 - minimum
	if coverage == 0 {
		return paper
	}
	r, g, b = (r-minimum)*255/coverage, (g-minimum)*255/coverage, (b-minimum)*255/coverage
	chroma := max(r, g, b)
	channel := func(value, neutral, background int) uint8 {
		colored := 107 + (148*value+127)/255
		ink := (neutral*(255-chroma) + colored*chroma + 127) / 255
		return uint8((background*(255-coverage) + ink*coverage + 127) / 255)
	}
	return color.RGBA{channel(r, 220, int(paper.R)), channel(g, 226, int(paper.G)), channel(b, 222, int(paper.B)), 255}
}
