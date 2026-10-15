package preview

import (
	"context"
	"image"
	"image/color"
	"io"
	"os"
	"testing"
)

func TestThemeParsingAndCycle(t *testing.T) {
	for input, want := range map[string]Theme{"": ThemeWarm, "warm": ThemeWarm, " NIGHT ": ThemeNight, "paper": ThemePaper} {
		got, err := ParseTheme(input)
		if err != nil || got != want {
			t.Fatalf("ParseTheme(%q) = %q, %v", input, got, err)
		}
	}
	if _, err := ParseTheme("sepia"); err == nil {
		t.Fatal("invalid theme accepted")
	}
	if ThemeWarm.Next() != ThemeNight || ThemeNight.Next() != ThemePaper || ThemePaper.Next() != ThemeWarm || Theme("").Next() != ThemeNight {
		t.Fatal("theme cycle does not include all three backgrounds")
	}
}

func TestThemeGrayscaleCoverageAndTransparency(t *testing.T) {
	for _, theme := range []Theme{ThemePaper, ThemeWarm, ThemeNight} {
		t.Run(string(theme), func(t *testing.T) {
			// A non-zero origin and unassociated alpha also exercise the generic
			// image path. Half-transparent black should match opaque middle gray.
			src := image.NewNRGBA(image.Rect(7, 9, 12, 10))
			for i, pixel := range []color.NRGBA{{0, 0, 0, 255}, {127, 127, 127, 255}, {255, 255, 255, 255}, {0, 0, 0, 128}, {231, 12, 17, 0}} {
				src.SetNRGBA(7+i, 9, pixel)
			}
			img, err := themeRaster(context.Background(), src, theme)
			if err != nil {
				t.Fatal(err)
			}
			ink := color.RGBAModel.Convert(img.At(7, 9)).(color.RGBA)
			middle := color.RGBAModel.Convert(img.At(8, 9)).(color.RGBA)
			paper := color.RGBAModel.Convert(img.At(9, 9)).(color.RGBA)
			if paper != theme.paper() || color.RGBAModel.Convert(img.At(11, 9)) != paper {
				t.Fatal("transparent/white pixels did not become the themed paper")
			}
			if middle != color.RGBAModel.Convert(img.At(10, 9)) || middle == ink || middle == paper {
				t.Fatal("antialias coverage was lost or transparent ink was mishandled")
			}
			if theme == ThemeNight {
				if !(ink.R > middle.R && middle.R > paper.R && ink.G > middle.G && middle.G > paper.G && ink.B > middle.B && middle.B > paper.B) {
					t.Fatal("night antialiasing is not between pale ink and dark paper")
				}
			} else if !(ink.R < middle.R && middle.R < paper.R) {
				t.Fatal("light-paper antialiasing is not monotonic")
			}
		})
	}
}

func TestNightKeepsColoredInkAndAntialiasing(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 1))
	src.SetRGBA(0, 0, color.RGBA{255, 0, 0, 255})
	src.SetRGBA(1, 0, color.RGBA{0, 255, 0, 255})
	src.SetRGBA(2, 0, color.RGBA{0, 0, 255, 255})
	src.SetRGBA(3, 0, color.RGBA{255, 128, 128, 255})
	img, err := themeRaster(context.Background(), src, ThemeNight)
	if err != nil {
		t.Fatal(err)
	}
	if img != src {
		t.Fatal("RGBA raster unnecessarily allocated another full bitmap")
	}
	for x := 0; x < 3; x++ {
		pixel := src.RGBAAt(x, 0)
		channels := []uint8{pixel.R, pixel.G, pixel.B}
		if channels[x] < 230 || channels[(x+1)%3] > 150 || channels[(x+2)%3] > 150 || pixel.A != 255 {
			t.Fatalf("colored annotation changed hue or lost contrast: %v", pixel)
		}
	}
	red, edge, paper := src.RGBAAt(0, 0), src.RGBAAt(3, 0), ThemeNight.paper()
	if !(paper.R < edge.R && edge.R < red.R && paper.G < edge.G && edge.G < red.G && paper.B < edge.B && edge.B < red.B) {
		t.Fatalf("colored antialiasing is not blended against paper: %v", edge)
	}
}

func TestCanceledThemeRaster(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := themeRaster(ctx, image.NewRGBA(image.Rect(0, 0, 8, 8)), ThemeNight); err != context.Canceled {
		t.Fatalf("theme worker did not respect cancellation: %v", err)
	}
}

func TestRealPopplerThemeChangesAndCache(t *testing.T) {
	toolsAvailable(t)
	s, err := NewService(context.Background(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	source := testPDF(t, "0 0 0 rg 20 180 40 60 re f 1 0 0 rg 80 180 30 60 re f 0 0 1 rg 130 180 30 60 re f")
	original, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	opens := 0
	r := Request{Generation: 1, Page: 1, Width: 300, Height: 300, Zoom: 1, FullPage: true, Open: func() (io.ReadCloser, error) { opens++; return os.Open(source) }}
	payloads := make(map[Theme]string)
	for i, theme := range []Theme{"", ThemeNight, ThemePaper, ThemeWarm} {
		r.Version, r.Theme = uint64(i+1), theme
		s.Submit(r)
		state := await(t, s, r.Version)
		img := decode(t, state)
		canonical, _ := ParseTheme(string(theme))
		if state.Theme != canonical || color.RGBAModel.Convert(img.At(0, 0)) != canonical.paper() {
			t.Fatalf("stale/unreported theme: %q background=%v", state.Theme, img.At(0, 0))
		}
		ink := color.RGBAModel.Convert(img.At(80, 80)).(color.RGBA)
		red := color.RGBAModel.Convert(img.At(145, 80)).(color.RGBA)
		blue := color.RGBAModel.Convert(img.At(195, 80)).(color.RGBA)
		if red.R <= red.G || red.R <= red.B || blue.B <= blue.R || blue.B <= blue.G || ink == canonical.paper() {
			t.Fatal("themed Poppler image lost music or annotation color")
		}
		if prior, exists := payloads[canonical]; exists && prior != state.Payload {
			t.Fatal("returning to a cached theme changed its pixels")
		}
		for other, payload := range payloads {
			if other != canonical && payload == state.Payload {
				t.Fatal("theme change reused another theme's cached raster")
			}
		}
		payloads[canonical] = state.Payload
	}
	if opens != 1 {
		t.Fatal("theme changes reopened the source instead of using the owned PDF")
	}
	after, err := os.ReadFile(source)
	if err != nil || string(original) != string(after) {
		t.Fatal("preview theme modified the source PDF")
	}
	// A theme change must not turn the whole dark page into content or change
	// scroll limits: measuring white paper is independent of recoloring.
	var position image.Point
	for i, theme := range []Theme{ThemeWarm, ThemeNight, ThemePaper} {
		r.Version, r.Theme = uint64(i+10), theme
		r.FullPage, r.ScrollX, r.ScrollY = false, 10000, 10000
		s.Submit(r)
		state := await(t, s, r.Version)
		decode(t, state)
		point := image.Pt(state.ScrollX, state.ScrollY)
		if i == 0 {
			position = point
		} else if point != position {
			t.Fatalf("theme changed content-fit geometry: %v versus %v", point, position)
		}
	}
	r.Version, r.Theme = 20, Theme("invalid")
	s.Submit(r)
	if state := await(t, s, r.Version); state.Err == nil || state.Payload != "" {
		t.Fatal("invalid theme rendered a misleading successful preview")
	}
}
