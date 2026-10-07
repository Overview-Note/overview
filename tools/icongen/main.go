package main

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"flag"
	"fmt"
	"hash/crc32"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"

	xdraw "golang.org/x/image/draw"
)

var pngSignature = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

const radiusFraction = 44.0 / 192.0

var iconSizes = []int{16, 24, 32, 48, 64, 128, 192, 256}

type frame struct {
	size int
	data []byte
}

type outputs struct {
	src      string
	ico      string
	png512   string
	png192   string
	appicon  string
	maskable string
}

func main() {
	root := findRepoRoot()
	src := flag.String("src", "web/public/icon-512.png", "source PNG (repo-root relative)")
	ico := flag.String("ico", "cmd/overview-desktop/icon.ico", "output ICO path (repo-root relative)")
	png512 := flag.String("png", "cmd/overview-desktop/icon.png", "output 512 RGBA PNG path (repo-root relative)")
	png192 := flag.String("png192", "web/public/icon-192.png", "output 192 RGBA PNG path (repo-root relative)")
	appicon := flag.String("appicon", "build/appicon.png", "output 512 RGBA PNG for the Wails packager (repo-root relative)")
	maskable := flag.String("maskable", "web/public/icon-512-maskable.png", "output 512 full-bleed opaque PNG (repo-root relative)")
	flag.Parse()

	out := outputs{
		src:      *src,
		ico:      *ico,
		png512:   *png512,
		png192:   *png192,
		appicon:  *appicon,
		maskable: *maskable,
	}

	if err := run(root, out); err != nil {
		fmt.Fprintln(os.Stderr, "icongen:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s, %s, %s, %s and %s (%d ICO frames)\n",
		resolve(root, out.png512), resolve(root, out.png192), resolve(root, out.appicon), resolve(root, out.maskable), resolve(root, out.ico), len(iconSizes))
}

func run(root string, out outputs) error {
	src := resolve(root, out.src)
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	img, err := png.Decode(f)
	_ = f.Close()
	if err != nil {
		return fmt.Errorf("decode %s: %w", src, err)
	}

	master := toNRGBA(img)
	full := master
	if isOpaque(img) {
		master = roundCorners(master, radiusFraction)
	} else {
		full = forceOpaque(master)
	}

	if err := writePNG(resolve(root, out.png512), master); err != nil {
		return err
	}
	if err := writePNG(resolve(root, out.appicon), master); err != nil {
		return err
	}
	if err := writePNG(resolve(root, out.png192), scale(master, 192)); err != nil {
		return err
	}
	if err := writeRGBA(resolve(root, out.maskable), full); err != nil {
		return err
	}
	ico, err := buildICO(master)
	if err != nil {
		return err
	}
	icoPath := resolve(root, out.ico)
	if err := os.MkdirAll(filepath.Dir(icoPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(icoPath, ico, 0o644)
}

// findRepoRoot locates the module root so output paths never depend on the
// process working directory. It prefers the source file's location (stable for
// `go run` and local builds) and falls back to walking up from the CWD.
func findRepoRoot() string {
	if _, file, _, ok := runtime.Caller(0); ok {
		if root, found := repoRootFrom(filepath.Dir(file)); found {
			return root
		}
	}
	if wd, err := os.Getwd(); err == nil {
		if root, found := repoRootFrom(wd); found {
			return root
		}
	}
	return "."
}

func repoRootFrom(dir string) (string, bool) {
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func resolve(root, p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(root, p)
}

func isOpaque(img image.Image) bool {
	if o, ok := img.(interface{ Opaque() bool }); ok {
		return o.Opaque()
	}
	return false
}

func toNRGBA(img image.Image) *image.NRGBA {
	b := img.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	xdraw.Draw(dst, dst.Bounds(), img, b.Min, xdraw.Src)
	return dst
}

func roundCorners(src *image.NRGBA, fraction float64) *image.NRGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	radius := fraction * float64(w)
	dst := image.NewNRGBA(b)
	const samples = 4
	const total = samples * samples
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			covered := 0
			for sy := 0; sy < samples; sy++ {
				for sx := 0; sx < samples; sx++ {
					px := float64(x) + (float64(sx)+0.5)/samples
					py := float64(y) + (float64(sy)+0.5)/samples
					if insideRoundedRect(px, py, float64(w), float64(h), radius) {
						covered++
					}
				}
			}
			c := src.NRGBAAt(x, y)
			if covered != total {
				c.A = uint8(int(c.A) * covered / total)
			}
			dst.SetNRGBA(x, y, c)
		}
	}
	return dst
}

func forceOpaque(src *image.NRGBA) *image.NRGBA {
	b := src.Bounds()
	dst := image.NewNRGBA(b)
	copy(dst.Pix, src.Pix)
	for i := 3; i < len(dst.Pix); i += 4 {
		dst.Pix[i] = 0xff
	}
	return dst
}

func insideRoundedRect(px, py, w, h, radius float64) bool {
	if px < 0 || py < 0 || px > w || py > h {
		return false
	}
	cx := min(max(px, radius), w-radius)
	cy := min(max(py, radius), h-radius)
	dx, dy := px-cx, py-cy
	return dx*dx+dy*dy <= radius*radius
}

func scale(src *image.NRGBA, size int) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)
	return dst
}

func buildICO(master *image.NRGBA) ([]byte, error) {
	frames := make([]frame, 0, len(iconSizes))
	for _, size := range iconSizes {
		var buf bytes.Buffer
		enc := png.Encoder{CompressionLevel: png.BestCompression}
		if err := enc.Encode(&buf, scale(master, size)); err != nil {
			return nil, err
		}
		frames = append(frames, frame{size: size, data: buf.Bytes()})
	}

	out := make([]byte, 6+16*len(frames))
	binary.LittleEndian.PutUint16(out[0:], 0)
	binary.LittleEndian.PutUint16(out[2:], 1)
	binary.LittleEndian.PutUint16(out[4:], uint16(len(frames)))

	offset := len(out)
	for i, fr := range frames {
		entry := out[6+16*i : 6+16*i+16]
		dim := byte(0)
		if fr.size < 256 {
			dim = byte(fr.size)
		}
		entry[0] = dim
		entry[1] = dim
		entry[2] = 0
		entry[3] = 0
		binary.LittleEndian.PutUint16(entry[4:], 1)
		binary.LittleEndian.PutUint16(entry[6:], 32)
		binary.LittleEndian.PutUint32(entry[8:], uint32(len(fr.data)))
		binary.LittleEndian.PutUint32(entry[12:], uint32(offset))
		offset += len(fr.data)
	}
	for _, fr := range frames {
		out = append(out, fr.data...)
	}
	return out, nil
}

func writePNG(path string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, img); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// writeRGBA writes an NRGBA image as a forced colorType=6 (RGBA) PNG. The Go
// png.Encoder picks colorType=2 for fully opaque images, so this hand-rolled
// writer keeps the maskable icon RGBA to match the other brand assets.
func writeRGBA(path string, img *image.NRGBA) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := encodeRGBA(img)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func encodeRGBA(img *image.NRGBA) ([]byte, error) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()

	var raw bytes.Buffer
	row := make([]byte, 1+w*4)
	for y := 0; y < h; y++ {
		row[0] = 0 // filter type: None
		copy(row[1:], img.Pix[img.PixOffset(b.Min.X, b.Min.Y+y):][:w*4])
		raw.Write(row)
	}

	var idat bytes.Buffer
	zw, err := zlib.NewWriterLevel(&idat, zlib.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := zw.Write(raw.Bytes()); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}

	var out bytes.Buffer
	out.Write(pngSignature)
	writeChunk(&out, "IHDR", ihdrData(w, h))
	writeChunk(&out, "IDAT", idat.Bytes())
	writeChunk(&out, "IEND", nil)
	return out.Bytes(), nil
}

func ihdrData(w, h int) []byte {
	d := make([]byte, 13)
	binary.BigEndian.PutUint32(d[0:], uint32(w))
	binary.BigEndian.PutUint32(d[4:], uint32(h))
	d[8] = 8 // bit depth
	d[9] = 6 // color type: RGBA
	d[10] = 0
	d[11] = 0
	d[12] = 0
	return d
}

func writeChunk(w *bytes.Buffer, typ string, data []byte) {
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(data)))
	w.Write(hdr[:])
	w.WriteString(typ)
	w.Write(data)
	crc := crc32.NewIEEE()
	crc.Write([]byte(typ))
	crc.Write(data)
	var sum [4]byte
	binary.BigEndian.PutUint32(sum[:], crc.Sum32())
	w.Write(sum[:])
}
