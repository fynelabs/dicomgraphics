package main

import (
	"fmt"
	"image/jpeg"
	"log"
	"os"
	"strconv"

	"github.com/fynelabs/dicomgraphics"
	"github.com/suyashkumar/dicom"
	"github.com/suyashkumar/dicom/pkg/frame"
	"github.com/suyashkumar/dicom/pkg/tag"
)

func findImage(data dicom.Dataset) (*frame.NativeFrame, int16, int16) {
	var frame *frame.NativeFrame
	level := int16(40)
	width := int16(380)
	for _, elem := range data.Elements {
		if elem.Tag == tag.PixelData {
			frames := elem.Value.GetValue().(dicom.PixelDataInfo).Frames

			if len(frames) == 0 {
				panic("No images found")
			} else if len(frames) > 1 {
				log.Println("Many images found, displaying only first element")
			}

			frame = &frames[0].NativeData
		} else if elem.Tag == tag.WindowCenter {
			l, _ := strconv.Atoi(fmt.Sprintf("%v", elem.Value))
			level = int16(l)
		} else if elem.Tag == tag.WindowWidth {
			l, _ := strconv.Atoi(fmt.Sprintf("%v", elem.Value))
			width = int16(l)
		}
	}

	return frame, level, width
}

func main() {
	if len(os.Args) != 2 {
		log.Println("Must pass a parameter - the file to convert")
		return
	}

	path := os.Args[1]
	data, err := dicom.ParseFile(path, nil)
	if err != nil {
		log.Println("Error parsing " + path)
		return
	}

	frame, level, width := findImage(data)
	if frame == nil {
		log.Println("No image found")
		return
	}

	jpegPath := path[:len(path)-3] + "jpg"
	f, err := os.Create(jpegPath)
	if err != nil {
		panic(err)
	}
	err = jpeg.Encode(f, dicomgraphics.NewDICOMImage(frame, level, width), nil)
	if err != nil {
		panic(err)
	}
	err = f.Close()
	if err != nil {
		panic(err)
	}

	fmt.Println("Written", jpegPath, "at", level, "width", width)
}
