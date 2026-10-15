package preview

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A real, self-contained two-page PDF exercises Poppler deterministically.
// The page shapes differ so page navigation cannot pass using a stale raster.
func testPDF(t *testing.T, pageStreams ...string) string {
	t.Helper()
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	stream := []string{"0 0 0 rg 20 180 40 60 re f", "1 0 0 rg 80 100 90 40 re f"}
	copy(stream, pageStreams)
	objects := []string{"<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Kids [3 0 R 5 0 R] /Count 2 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 300] /Resources << >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream[0])+1, stream[0]),
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 300] /Resources << >> /Contents 6 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream[1])+1, stream[1])}
	offsets := []int{0}
	for i, o := range objects {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, n := range offsets[1:] {
		fmt.Fprintf(&b, "%010d 00000 n \n", n)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	path := filepath.Join(t.TempDir(), "score with spaces.pdf")
	if err := os.WriteFile(path, b.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
func toolsAvailable(t *testing.T) {
	t.Helper()
	for _, tool := range []string{"pdfinfo", "pdftoppm"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip("requires Poppler: " + tool)
		}
	}
}
func await(t *testing.T, s *Service, version uint64) State {
	t.Helper()
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-s.Changed():
			state := s.Snapshot()
			if state.Version == version {
				return state
			}
		case <-timer.C:
			t.Fatal("preview worker timed out")
			return State{}
		}
	}
}
func decode(t *testing.T, state State) image.Image {
	t.Helper()
	if state.Err != nil {
		t.Fatal(state.Err)
	}
	data, err := base64.StdEncoding.DecodeString(state.Payload)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return img
}
func TestRealPopplerPagesZoomCropAndDocumentOwnership(t *testing.T) {
	toolsAvailable(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, err := NewService(ctx, "", "")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	source := testPDF(t)
	opens := 0
	r := Request{Version: 1, Generation: 1, Page: 1, Width: 300, Height: 300, Zoom: 1, FullPage: true, Theme: ThemePaper, Open: func() (io.ReadCloser, error) { opens++; return os.Open(source) }}
	s.Submit(r)
	first := await(t, s, 1)
	img := decode(t, first)
	if first.Page != 1 || first.Pages != 2 || img.Bounds().Dx() != 300 || img.Bounds().Dy() != 300 {
		t.Fatalf("bad first page: %+v %v", first, img.Bounds())
	}
	minX, minY, maxX, maxY := 300, 300, 0, 0
	for y := 0; y < 300; y++ {
		for x := 0; x < 300; x++ {
			red, g, b, _ := img.At(x, y).RGBA()
			if red < 10000 && g < 10000 && b < 10000 {
				minX = min(minX, x)
				minY = min(minY, y)
				maxX = max(maxX, x)
				maxY = max(maxY, y)
			}
		}
	}
	if maxX-minX < 38 || maxX-minX > 41 || maxY-minY < 58 || maxY-minY > 61 {
		t.Fatalf("PDF was stretched or empty: %d,%d to %d,%d", minX, minY, maxX, maxY)
	}
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	} // later pages must use owned document
	r.Version = 2
	r.Page = 2
	s.Submit(r)
	second := await(t, s, 2)
	decode(t, second)
	if second.Page != 2 || first.Payload == second.Payload {
		t.Fatal("page change did not produce a new raster")
	}
	r.Version = 3
	r.Zoom = 3
	r.Width = 120
	r.Height = 80
	r.ScrollX = 100000
	r.ScrollY = 100000
	s.Submit(r)
	zoomed := await(t, s, 3)
	img = decode(t, zoomed)
	if img.Bounds() != image.Rect(0, 0, 120, 80) || zoomed.ScrollX <= 0 || zoomed.ScrollY <= 0 || zoomed.ScrollY >= 100000 {
		t.Fatalf("viewport not cropped/clamped: %v %+v", img.Bounds(), zoomed)
	}
	r.Version = 4
	r.Page = 99
	s.Submit(r)
	last := await(t, s, 4)
	decode(t, last)
	if last.Page != 2 || opens != 1 {
		t.Fatalf("document ownership/clamping failed: page=%d opens=%d", last.Page, opens)
	}
	r.Version = 5
	r.Width = 10000
	s.Submit(r)
	if state := await(t, s, 5); state.Err == nil {
		t.Fatal("unbounded viewport accepted")
	}
}
func TestMissingPopplerIsUseful(t *testing.T) {
	_, err := NewService(context.Background(), "/missing/pdfinfo", "")
	if err == nil || !strings.Contains(err.Error(), "make deps") {
		t.Fatal(err)
	}
}

func TestContentFitEnlargesMusicAndKeepsFooterReachable(t *testing.T) {
	toolsAvailable(t)
	s, err := NewService(context.Background(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// Black music near the top and a red footer far below. Bounding all ink
	// and fitting its height would reproduce the user's oversized margins.
	source := testPDF(t, "0 0 0 rg 40 220 100 25 re f 1 0 0 rg 90 10 20 3 re f")
	r := Request{Version: 1, Generation: 1, Page: 1, Width: 300, Height: 220, Zoom: 1, FullPage: true, Theme: ThemePaper, Open: func() (io.ReadCloser, error) { return os.Open(source) }}
	s.Submit(r)
	full := decode(t, await(t, s, 1))
	paperInk, _, _ := printedPixels(full)
	r.Version = 2
	r.FullPage = false
	s.Submit(r)
	fit := await(t, s, 2)
	content := decode(t, fit)
	music, black, red := printedPixels(content)
	if fit.FullPage || black == 0 || red != 0 || music.Dx() < 270 || music.Min.Y > 20 || music.Dx() < paperInk.Dx()*2 {
		t.Fatalf("content was not enlarged/top-aligned: music=%v paper=%v black=%d red=%d", music, paperInk, black, red)
	}
	if ratio := float64(music.Dx()) / float64(music.Dy()); ratio < 3.8 || ratio > 4.2 {
		t.Fatalf("content fit stretched the PDF: %v", music)
	}
	r.Version = 3
	r.ScrollY = 100000
	s.Submit(r)
	bottom := await(t, s, 3)
	_, black, red = printedPixels(decode(t, bottom))
	if bottom.ScrollY <= 0 || red == 0 || black != 0 {
		t.Fatalf("footer was discarded or scrolling escaped crop: y=%d black=%d red=%d", bottom.ScrollY, black, red)
	}
	// A new generation moves the content to a different corner. It must be
	// measured again instead of reusing the prior PDF's crop.
	next := testPDF(t, "0 0 0 rg 120 20 40 30 re f")
	r.Version = 4
	r.Generation = 2
	r.ScrollY = 0
	r.Open = func() (io.ReadCloser, error) { return os.Open(next) }
	s.Submit(r)
	music, black, _ = printedPixels(decode(t, await(t, s, 4)))
	if black == 0 || music.Dx() < 270 || music.Min.Y > 20 {
		t.Fatalf("new generation reused stale margins: %v", music)
	}
}

func printedPixels(img image.Image) (image.Rectangle, int, int) {
	bounds := img.Bounds()
	ink := image.Rectangle{Min: bounds.Max, Max: bounds.Min}
	black, red := 0, 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if r < 10000 && g < 10000 && b < 10000 {
				black++
				ink.Min.X = min(ink.Min.X, x)
				ink.Min.Y = min(ink.Min.Y, y)
				ink.Max.X = max(ink.Max.X, x+1)
				ink.Max.Y = max(ink.Max.Y, y+1)
			}
			if r > 50000 && g < 10000 && b < 10000 {
				red++
			}
		}
	}
	return ink, black, red
}

func TestBlankPageContentFit(t *testing.T) {
	toolsAvailable(t)
	s, err := NewService(context.Background(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	source := testPDF(t, "")
	s.Submit(Request{Version: 1, Generation: 1, Page: 1, Width: 200, Height: 300, Zoom: 1, Open: func() (io.ReadCloser, error) { return os.Open(source) }})
	img := decode(t, await(t, s, 1))
	_, black, red := printedPixels(img)
	if img.Bounds() != image.Rect(0, 0, 200, 300) || black != 0 || red != 0 {
		t.Fatal("blank-page fallback failed")
	}
}

func TestSupersededRasterIsCanceledAndCannotPublish(t *testing.T) {
	toolsAvailable(t)
	dir := t.TempDir()
	realRaster, err := exec.LookPath("pdftoppm")
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "started")
	wrapper := filepath.Join(dir, "raster")
	// The first-page worker blocks in one owned executable; the second request
	// must cancel it and finish without waiting for its artificial delay.
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
	script := "#!/bin/sh\nif [ \"$2\" = 1 ]; then touch " + quote(marker) + "; exec /bin/sleep 30; fi\nexec " + quote(realRaster) + " \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	s, err := NewService(context.Background(), "", wrapper)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	file := testPDF(t)
	r := Request{Version: 1, Generation: 1, Page: 1, Width: 200, Height: 300, Zoom: 1, Open: func() (io.ReadCloser, error) { return os.Open(file) }}
	s.Submit(r)
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("rasterizer did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	r.Version = 2
	r.Page = 2
	s.Submit(r)
	state := await(t, s, 2)
	decode(t, state)
	if state.Page != 2 {
		t.Fatal("stale page replaced current preview")
	}
}
func TestRealEngravedPDF(t *testing.T) {
	toolsAvailable(t)
	exe := os.Getenv("LYRA_TEST_LILYPOND")
	if exe == "" {
		t.Skip("set LYRA_TEST_LILYPOND for real engraving integration")
	}
	dir := t.TempDir()
	source, err := filepath.Abs("../testdata/scale.ly")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "--pdf", "--output="+filepath.Join(dir, "score"), source)
	cmd.Env = append(os.Environ(), "HOME="+dir, "XDG_CACHE_HOME="+filepath.Join(dir, "cache"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("real LilyPond: %v %s", err, out)
	}
	s, err := NewService(ctx, "", "")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.Submit(Request{Version: 1, Generation: 1, Page: 1, Width: 640, Height: 800, Zoom: 1, Open: func() (io.ReadCloser, error) { return os.Open(filepath.Join(dir, "score.pdf")) }})
	img := decode(t, await(t, s, 1))
	ink := 0
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if r < 20000 && g < 20000 && b < 20000 {
				ink++
			}
		}
	}
	if ink < 500 {
		t.Fatalf("engraved score raster has too little ink: %d", ink)
	}
}
