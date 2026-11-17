package wildberries

import (
	"bytes"
	"encoding/base64"
	"hash/fnv"
	"html"
	"image"
	"image/color"
	"image/png"
	"strconv"
)

// labelPNG draws a real PNG picture of the given size in pixels (580 x 400 is a 58 x 40 mm label): white, with black bars
// that depend on the text, so that two labels differ. It is a picture to be stored and printed, not a code to be scanned.
func labelPNG(w, h int, text string) []byte {
	img := image.NewGray(image.Rect(0, 0, w, h))

	for i := range img.Pix {
		img.Pix[i] = 0xFF
	}

	hash := fnv.New64a()
	_, _ = hash.Write([]byte(text))
	bits := hash.Sum64()

	barWidth := w / 40
	for bar := range 32 {
		if bits>>uint(bar%64)&1 == 0 {
			continue
		}

		for x := (bar + 4) * barWidth; x < (bar+5)*barWidth && x < w; x++ {
			for y := h / 5; y < h*4/5; y++ {
				img.SetGray(x, y, color.Gray{})
			}
		}
	}

	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, img); err != nil {
		return nil
	}

	return buf.Bytes()
}

func labelSVG(w, h int, text string) []byte {
	return []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="` + strconv.Itoa(w) + `" height="` + strconv.Itoa(h) + `"><rect width="100%" height="100%" fill="white"/><text x="20" y="40">` +
		html.EscapeString(text) + `</text></svg>`)
}

func labelZPL(text string) []byte {
	return []byte("^XA^FO20,20^BQN,2,6^FDQA," + text + "^FS^XZ")
}

// pictureBytes makes the picture of a sticker in one of the four formats the files name (svg, zplv, zplh, png).
func pictureBytes(kind string, w, h int, text string) []byte {
	switch kind {
	case "png":
		return labelPNG(w, h, text)
	case "svg":
		return labelSVG(w, h, text)
	default:
		return labelZPL(text)
	}
}

// pictureFile is a sticker's "file" as a string: PNG and SVG written in base64, ZPL as it is.
func pictureFile(kind string, w, h int, text string) string {
	data := pictureBytes(kind, w, h, text)
	if kind == "zplv" || kind == "zplh" {
		return string(data)
	}

	return base64.StdEncoding.EncodeToString(data)
}
