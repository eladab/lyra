package preview

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"time"
)

type Request struct {
	Version, Generation                   uint64
	Open                                  func() (io.ReadCloser, error)
	Page, Width, Height, ScrollX, ScrollY int
	Zoom                                  float64
	FullPage                              bool  // default: crop margins and fit printed width
	Theme                                 Theme // default: warm paper
}
type State struct {
	Version, Generation           uint64
	Page, Pages, ScrollX, ScrollY int
	Payload                       string // viewport PNG, already base64 encoded off the UI thread
	Err                           error
	FullPage                      bool
	Theme                         Theme
}
type Service struct {
	root, info, raster string
	mu                 sync.Mutex
	pending            *Request
	active             context.CancelFunc
	state              State
	requests, changed  chan struct{}
	done               chan struct{}
	cancel             context.CancelFunc
}

func NewService(parent context.Context, info, raster string) (*Service, error) {
	if info == "" {
		info = "pdfinfo"
	}
	if raster == "" {
		raster = "pdftoppm"
	}
	var err error
	if info, err = exec.LookPath(info); err != nil {
		return nil, fmt.Errorf("PDF preview needs Poppler: run make deps or use --pdfinfo and --pdftoppm")
	}
	if raster, err = exec.LookPath(raster); err != nil {
		return nil, fmt.Errorf("PDF preview needs Poppler: run make deps or use --pdftoppm")
	}
	root, err := os.MkdirTemp("", "lyra-preview-")
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	s := &Service{root: root, info: info, raster: raster, requests: make(chan struct{}, 1), changed: make(chan struct{}, 1), done: make(chan struct{}), cancel: cancel}
	go s.run(ctx)
	return s, nil
}
func (s *Service) Submit(r Request) {
	s.mu.Lock()
	s.pending = &r
	if s.active != nil {
		s.active()
	}
	s.mu.Unlock()
	select {
	case s.requests <- struct{}{}:
	default:
	}
}
func (s *Service) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending = nil
	if s.active != nil {
		s.active()
	}
}
func (s *Service) Changed() <-chan struct{} { return s.changed }
func (s *Service) Done() <-chan struct{}    { return s.done }
func (s *Service) Snapshot() State          { s.mu.Lock(); defer s.mu.Unlock(); return s.state }
func (s *Service) Close()                   { s.cancel(); <-s.done; _ = os.RemoveAll(s.root) }

var pagesPattern = regexp.MustCompile(`(?m)^Pages:\s+(\d+)`)
var sizePattern = regexp.MustCompile(`(?m)^Page(?:\s+\d+)?\s+size:\s+([\d.]+)\s+x\s+([\d.]+)`)

// Cache is limited to four page rasters and 64 MiB of decoded pixels. A single
// raster has at most 16 million pixels; the viewport is capped at 4 million.
type cached struct {
	key   string
	img   image.Image
	bytes int
}

func (s *Service) run(ctx context.Context) {
	defer close(s.done)
	var docGen uint64
	var pages int
	var cache []cached
	sizes := make(map[int][2]float64)
	content := make(map[int]contentArea)
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.requests:
		}
		s.mu.Lock()
		r := s.pending
		s.pending = nil
		if r == nil {
			s.mu.Unlock()
			continue
		}
		job, cancel := context.WithTimeout(ctx, 20*time.Second)
		s.active = cancel
		s.mu.Unlock()
		state := State{Version: r.Version, Generation: r.Generation, Page: r.Page, Pages: pages, FullPage: r.FullPage}
		func() {
			theme, err := ParseTheme(string(r.Theme))
			if err != nil {
				state.Err = err
				return
			}
			state.Theme = theme
			doc := filepath.Join(s.root, "document.pdf")
			if r.Generation != docGen {
				if r.Open == nil {
					state.Err = fmt.Errorf("no engraved PDF")
					return
				}
				in, err := r.Open()
				if err != nil {
					state.Err = err
					return
				}
				defer in.Close()
				candidate := filepath.Join(s.root, "incoming.pdf")
				out, err := os.Create(candidate)
				if err != nil {
					state.Err = err
					return
				}
				n, err := io.Copy(out, io.LimitReader(in, (64<<20)+1))
				closeErr := out.Close()
				if err != nil {
					state.Err = err
					return
				}
				if closeErr != nil {
					state.Err = closeErr
					return
				}
				if n > 64<<20 {
					state.Err = fmt.Errorf("PDF exceeds 64 MiB preview limit")
					return
				}
				data, err := s.command(job, s.info, candidate)
				if err != nil {
					state.Err = fmt.Errorf("inspect PDF: %w", err)
					return
				}
				match := pagesPattern.FindSubmatch(data)
				if len(match) != 2 {
					state.Err = fmt.Errorf("Poppler did not report a PDF page count")
					return
				}
				newPages, _ := strconv.Atoi(string(match[1]))
				if newPages < 1 {
					state.Err = fmt.Errorf("PDF has no pages")
					return
				}
				if err := os.Rename(candidate, doc); err != nil {
					state.Err = err
					return
				}
				pages = newPages
				docGen = r.Generation
				cache = nil
				sizes = make(map[int][2]float64)
				content = make(map[int]contentArea)
			}
			state.Pages = pages
			state.Page = max(1, min(pages, r.Page))
			w, h := max(1, r.Width), max(1, r.Height)
			if w > 4096 || h > 4096 || int64(w)*int64(h) > 4_000_000 {
				state.Err = fmt.Errorf("preview viewport is too large; reduce terminal size")
				return
			}
			zoom := max(.25, min(4, r.Zoom))
			pageSize, ok := sizes[state.Page]
			if !ok {
				page := strconv.Itoa(state.Page)
				data, err := s.command(job, s.info, "-f", page, "-l", page, doc)
				if err != nil {
					state.Err = err
					return
				}
				match := sizePattern.FindSubmatch(data)
				if len(match) != 3 {
					state.Err = fmt.Errorf("Poppler did not report page dimensions")
					return
				}
				pageSize[0], _ = strconv.ParseFloat(string(match[1]), 64)
				pageSize[1], _ = strconv.ParseFloat(string(match[2]), 64)
				if pageSize[0] <= 0 || pageSize[1] <= 0 {
					state.Err = fmt.Errorf("invalid PDF page dimensions")
					return
				}
				if len(sizes) >= 16 {
					clear(sizes)
				}
				sizes[state.Page] = pageSize
			}
			scale := math.Min(float64(w)/pageSize[0], float64(h)/pageSize[1]) * zoom
			longEdge := math.Max(pageSize[0], pageSize[1]) * scale
			var area contentArea
			if !r.FullPage {
				var ok bool
				area, ok = content[state.Page]
				if !ok {
					probe, err := s.rasterPage(job, doc, state.Page, 1000)
					if err != nil {
						state.Err = err
						return
					}
					area, err = measureContent(job, probe)
					if err != nil {
						state.Err = err
						return
					}
					if len(content) >= 16 {
						clear(content)
					}
					content[state.Page] = area
				}
				longEdge = float64(w) * zoom * float64(max(area.page.Dx(), area.page.Dy())) / float64(area.ink.Dx())
			}
			size := max(1, min(4000, int(longEdge)))
			key := fmt.Sprintf("%d/%d/%d/%s", docGen, state.Page, size, theme)
			var img image.Image
			for i, c := range cache {
				if c.key == key {
					img = c.img
					cache = append(append(cache[:i:i], cache[i+1:]...), c)
					break
				}
			}
			if img == nil {
				var err error
				img, err = s.rasterPage(job, doc, state.Page, size)
				if err != nil {
					state.Err = err
					return
				}
				img, err = themeRaster(job, img, theme)
				if err != nil {
					state.Err = err
					return
				}
				cache = append(cache, cached{key, img, img.Bounds().Dx() * img.Bounds().Dy() * 4})
				for {
					total := 0
					for _, c := range cache {
						total += c.bytes
					}
					if len(cache) <= 4 && total <= 64<<20 {
						break
					}
					cache = cache[1:]
				}
			}
			if job.Err() != nil {
				return
			}
			bounds := img.Bounds()
			if !r.FullPage {
				bounds = area.bounds(bounds)
			}
			state.ScrollX = max(0, min(r.ScrollX, max(0, bounds.Dx()-w)))
			state.ScrollY = max(0, min(r.ScrollY, max(0, bounds.Dy()-h)))
			viewport := image.NewRGBA(image.Rect(0, 0, w, h))
			draw.Draw(viewport, viewport.Bounds(), image.NewUniform(theme.paper()), image.Point{}, draw.Src)
			x, y := max(0, (w-bounds.Dx())/2), max(0, (h-bounds.Dy())/2)
			if !r.FullPage {
				y = 0
			}
			draw.Draw(viewport, image.Rect(x, y, x+bounds.Dx()-state.ScrollX, y+bounds.Dy()-state.ScrollY), img, bounds.Min.Add(image.Pt(state.ScrollX, state.ScrollY)), draw.Src)
			var b bytes.Buffer
			encoder := png.Encoder{CompressionLevel: png.BestSpeed}
			if err := encoder.Encode(&b, viewport); err != nil {
				state.Err = err
				return
			}
			state.Payload = base64.StdEncoding.EncodeToString(b.Bytes())
		}()
		if job.Err() != nil {
			state.Err = job.Err()
		}
		cancel()
		s.mu.Lock()
		s.active = nil
		if ctx.Err() == nil && s.pending == nil {
			s.state = state
			select {
			case s.changed <- struct{}{}:
			default:
			}
		}
		s.mu.Unlock()
	}
}

type limitedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		return 0, fmt.Errorf("Poppler output exceeds limit")
	}
	return b.Buffer.Write(p)
}

// Bound both axes before Poppler allocates a bitmap, then validate decoded PNG
// dimensions as well (custom executables must obey the same memory limits).
func (s *Service) rasterPage(ctx context.Context, doc string, page, size int) (image.Image, error) {
	p := strconv.Itoa(page)
	data, err := s.command(ctx, s.raster, "-f", p, "-l", p, "-singlefile", "-scale-to", strconv.Itoa(size), "-png", doc)
	if err != nil {
		return nil, fmt.Errorf("rasterize PDF: %w", err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if int64(cfg.Width)*int64(cfg.Height) > 16_000_000 {
		return nil, fmt.Errorf("PDF raster exceeds pixel limit")
	}
	return png.Decode(bytes.NewReader(data))
}

func (s *Service) command(ctx context.Context, exe string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, exe, args...)
	configureProcess(cmd)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	cmd.WaitDelay = time.Second
	out := &limitedBuffer{limit: 32 << 20}
	stderr := &limitedBuffer{limit: 8192}
	cmd.Stdout = out
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%w: %.500s", err, stderr.String())
	}
	return out.Bytes(), nil
}
