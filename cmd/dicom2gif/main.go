package main

import (
	"fmt"
	"image"
	"image/color/palette"
	"image/gif"
	"log"
	"os"
	"strconv"

	"golang.org/x/image/draw"

	"github.com/gradienthealth/dicom"
	"github.com/gradienthealth/dicom/dicomtag"

	"github.com/andydotxyz/dicomgraphics"
)

func findImage(data *dicom.DataSet) ([]dicom.NativeFrame, int16, int16) {
	var frames []dicom.NativeFrame
	level := int16(40)
	width := int16(380)
	for _, elem := range data.Elements {
		if elem.Tag == dicomtag.PixelData {
			slices := elem.Value[0].(dicom.PixelDataInfo).Frames

			if len(slices) == 0 {
				panic("No images found")
			}

			for _, slice := range slices {
				frames = append(frames, slice.NativeData)
			}
		} else if elem.Tag == dicomtag.WindowCenter {
			l, _ := strconv.Atoi(fmt.Sprintf("%v", elem.Value[0]))
			level = int16(l)
		} else if elem.Tag == dicomtag.WindowWidth {
			l, _ := strconv.Atoi(fmt.Sprintf("%v", elem.Value[0]))
			width = int16(l)
		}
	}

	return frames, level, width
}

func main() {
	if len(os.Args) != 2 {
		log.Println("Must pass a parameter - the file to convert")
		return
	}

	path := os.Args[1]
	// TODO support a directory list as well
	parse, err := dicom.NewParserFromFile(path, nil)
	if err != nil {
		log.Println("Error loading " + path)
		return
	}

	data, err := parse.Parse(dicom.ParseOptions{DropPixelData: false})
	if err != nil {
		log.Println("Error parsing " + path)
		return
	}

	frames, level, width := findImage(data)
	if frames == nil {
		log.Println("No images found")
		return
	}

	gifPath := path[:len(path)-3] + "gif"
	f, err := os.Create(gifPath)
	if err != nil {
		panic(err)
	}

	var images []*image.Paletted
	var delays []int
	for _, frame := range frames {
		src := dicomgraphics.NewDICOMImage(&frame, level, width)
		img := image.NewPaletted(src.Bounds(), palette.WebSafe)
		draw.Copy(img, image.ZP, src, src.Bounds(), draw.Src, nil)

		images = append(images, img)
		delays = append(delays, 0)
	}
	err = gif.EncodeAll(f, &gif.GIF{
		Image: images,
		Delay: delays,
	})
	if err != nil {
		panic(err)
	}
	err = f.Close()
	if err != nil {
		panic(err)
	}

	fmt.Println("Written", len(images), "frames to", gifPath, "at", level, "width", width)
}
