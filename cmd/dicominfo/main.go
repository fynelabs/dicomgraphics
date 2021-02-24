package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/suyashkumar/dicom"
	"github.com/suyashkumar/dicom/pkg/tag"
)

type info struct {
	studyDate, name, id string
}

func (i *info) loadInfo(data dicom.Dataset) {
	for _, elem := range data.Elements {
		if elem.Tag == tag.PatientName {
			i.name = fmt.Sprintf("%v", elem.Value)
		} else if elem.Tag == tag.PatientID {
			i.id = fmt.Sprintf("%v", elem.Value)
		} else if elem.Tag == tag.StudyDate {
			i.studyDate = fmt.Sprintf("%v", elem.Value)
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
	data, err := dicom.ParseFile(path, nil)
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
