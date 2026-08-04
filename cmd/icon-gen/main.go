// Command icon-gen creates Neuropipe's source PNG and multi-resolution Windows icon.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

const sourceSize = 1024

func main() {
	icon := render(sourceSize)
	if err := writePNG(filepath.Join("build", "appicon.png"), icon); err != nil {
		fail(err)
	}
	if err := writePNG(filepath.Join("frontend", "src", "assets", "appicon.png"), icon); err != nil {
		fail(err)
	}
	if err := writeICO(filepath.Join("build", "windows", "icon.ico"), icon, []int{16, 20, 24, 32, 40, 48, 64, 128, 256}); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func render(size int) *image.RGBA {
	image := image.NewRGBA(image.Rect(0, 0, size, size))
	canvas := float64(size)
	background := color.RGBA{R: 16, G: 16, B: 18, A: 255}
	foreground := color.RGBA{R: 250, G: 250, B: 250, A: 255}

	fillRoundedRectangle(image, 72*canvas/1024, 72*canvas/1024, 952*canvas/1024, 952*canvas/1024, 232*canvas/1024, background)
	stroke := 118 * canvas / 1024
	drawRoundLine(image, point{X: 294 * canvas / 1024, Y: 730 * canvas / 1024}, point{X: 294 * canvas / 1024, Y: 294 * canvas / 1024}, stroke, foreground)
	drawRoundLine(image, point{X: 294 * canvas / 1024, Y: 294 * canvas / 1024}, point{X: 730 * canvas / 1024, Y: 730 * canvas / 1024}, stroke, foreground)
	drawRoundLine(image, point{X: 730 * canvas / 1024, Y: 730 * canvas / 1024}, point{X: 730 * canvas / 1024, Y: 294 * canvas / 1024}, stroke, foreground)
	return image
}

type point struct{ X, Y float64 }

func fillRoundedRectangle(destination *image.RGBA, left, top, right, bottom, radius float64, fill color.RGBA) {
	for y := 0; y < destination.Bounds().Dy(); y++ {
		for x := 0; x < destination.Bounds().Dx(); x++ {
			px, py := float64(x)+0.5, float64(y)+0.5
			closestX := math.Max(left+radius, math.Min(px, right-radius))
			closestY := math.Max(top+radius, math.Min(py, bottom-radius))
			if math.Hypot(px-closestX, py-closestY) <= radius {
				destination.SetRGBA(x, y, fill)
			}
		}
	}
}

func drawRoundLine(destination *image.RGBA, start, end point, width float64, fill color.RGBA) {
	minX := int(math.Max(0, math.Floor(math.Min(start.X, end.X)-width/2)))
	maxX := int(math.Min(float64(destination.Bounds().Dx()-1), math.Ceil(math.Max(start.X, end.X)+width/2)))
	minY := int(math.Max(0, math.Floor(math.Min(start.Y, end.Y)-width/2)))
	maxY := int(math.Min(float64(destination.Bounds().Dy()-1), math.Ceil(math.Max(start.Y, end.Y)+width/2)))
	lineX, lineY := end.X-start.X, end.Y-start.Y
	lengthSquared := lineX*lineX + lineY*lineY
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			px, py := float64(x)+0.5, float64(y)+0.5
			projection := ((px-start.X)*lineX + (py-start.Y)*lineY) / lengthSquared
			projection = math.Max(0, math.Min(1, projection))
			closestX, closestY := start.X+projection*lineX, start.Y+projection*lineY
			if math.Hypot(px-closestX, py-closestY) <= width/2 {
				destination.SetRGBA(x, y, fill)
			}
		}
	}
}

func writePNG(path string, source image.Image) (err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create icon directory: %w", err)
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create PNG: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close PNG: %w", closeErr)
		}
	}()
	if err := png.Encode(file, source); err != nil {
		return fmt.Errorf("encode PNG: %w", err)
	}
	return nil
}

func writeICO(path string, source image.Image, sizes []int) (err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create icon directory: %w", err)
	}
	images := make([][]byte, 0, len(sizes))
	for _, size := range sizes {
		var imageData bytes.Buffer
		if err := png.Encode(&imageData, scale(source, size)); err != nil {
			return fmt.Errorf("encode %dpx icon: %w", size, err)
		}
		images = append(images, imageData.Bytes())
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create ICO: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close ICO: %w", closeErr)
		}
	}()
	if err := binary.Write(file, binary.LittleEndian, uint16(0)); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, uint16(1)); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, uint16(len(images))); err != nil {
		return err
	}
	offset := uint32(6 + 16*len(images))
	for index, imageData := range images {
		size := sizes[index]
		width, height := uint8(size), uint8(size)
		if size == 256 {
			width, height = 0, 0
		}
		entry := struct {
			Width, Height, Colors, Reserved uint8
			Planes, BitCount                uint16
			Bytes, Offset                   uint32
		}{Width: width, Height: height, Planes: 1, BitCount: 32, Bytes: uint32(len(imageData)), Offset: offset}
		if err := binary.Write(file, binary.LittleEndian, entry); err != nil {
			return fmt.Errorf("write ICO directory: %w", err)
		}
		offset += uint32(len(imageData))
	}
	for _, imageData := range images {
		if _, err := file.Write(imageData); err != nil {
			return fmt.Errorf("write ICO image: %w", err)
		}
	}
	return nil
}

func scale(source image.Image, size int) *image.RGBA {
	target := image.NewRGBA(image.Rect(0, 0, size, size))
	bounds := source.Bounds()
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			sourceX := bounds.Min.X + x*bounds.Dx()/size
			sourceY := bounds.Min.Y + y*bounds.Dy()/size
			target.Set(x, y, source.At(sourceX, sourceY))
		}
	}
	return target
}
