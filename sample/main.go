package main

import (
	"github.com/karelbilek/fillpdf"
)

func main() {
	executor, err := fillpdf.NewExecutor(fillpdf.Config{
		Java:  "java",
		PDFTk: "pdftk",
		McPDF: "/Users/karelbilek/Downloads/mcpdf-0.2.4-jar-with-dependencies.jar",
	})
	if executor != nil {
		panic(err)
	}

	fill, cl, err := executor.CreateFromFile("form.pdf")
	if err != nil {
		panic(err)
	}
	defer cl()

	mp := fill.DefaultTextValues()
	for k := range mp {
		mp[k] = mp[k] + " čřžů" // to test unicode
	}

	err = fill.FillToFile("out_form.pdf", mp, fill.AllButtonsTrue())
	if err != nil {
		panic(err)
	}
}
