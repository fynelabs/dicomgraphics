package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/gradienthealth/dicom"
	"github.com/gradienthealth/dicom/dicomtag"
)

type info struct {
	studyDate, name, id string
}

func (i *info) loadInfo(data *dicom.DataSet) {
	for _, elem := range data.Elements {
		if elem.Tag == dicomtag.PatientName {
			i.name = fmt.Sprintf("%v", elem.Value[0])
		} else if elem.Tag == dicomtag.PatientID {
			i.id = fmt.Sprintf("%v", elem.Value[0])
		} else if elem.Tag == dicomtag.StudyDate {
			i.studyDate = fmt.Sprintf("%v", elem.Value[0])
		}
	}

}

func main() {
	showHeader := true
	flag.BoolVar(&showHeader, "header", false, "Show header information")
	flag.Parse()

	if len(flag.Args()) != 1 {
		log.Println("Must pass a parameter - the file to extract information")
		return
	}

	path := flag.Arg(0)
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

	info := &info{}
	info.loadInfo(data)

	if showHeader {
		fmt.Printf("PatientId,PatientName,StudyDate\n")
	}
	fmt.Printf("%s,%s,%s\n", info.id, info.name, info.studyDate)
}
