/*
 *  FillPDF - Fill PDF forms
 *  Copyright 2022 Karel Bilek
 *  Copyright DesertBit
 *  Author: Roland Singer
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package fillpdf

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
)

type Config struct {
	Java  string
	PDFTk string
	McPDF string
}

type Executor struct {
	java, pdftk, mcpdf string
}

func NewExecutor(config Config) (*Executor, error) {
	pdftk, java, mcpdf := config.PDFTk, config.Java, config.McPDF
	if _, err := exec.LookPath(pdftk); err != nil {
		return nil, fmt.Errorf("pdftk utility is not installed at %q: %w", pdftk, err)
	}

	if _, err := exec.LookPath(java); err != nil {
		return nil, fmt.Errorf("pdftk utility is not installed at %q: %w", java, err)
	}

	// sniff the start of mcpdf to tell if it's the correct one...
	f, err := os.Open(mcpdf)
	if err != nil {
		return nil, fmt.Errorf("mcpdf file not found at %s: %w", mcpdf, err)
	}
	defer f.Close()

	buffer := make([]byte, 512)
	if _, err := f.Read(buffer); err != nil {
		return nil, fmt.Errorf("mcpdf file cannot be read from %s: %w", mcpdf, err)
	}

	contentType := http.DetectContentType(buffer)
	if contentType != "application/zip" {
		return nil, fmt.Errorf("mcpdf file does not seem to be %q, is %q", "application/zip", contentType)
	}

	return &Executor{
		java:  java,
		pdftk: pdftk,
		mcpdf: mcpdf,
	}, nil
}

type FillPDF struct {
	dir         string
	fieldsNames []string
	fields      map[string]FormField
	e           Executor
}

type FormField struct {
	Name         string
	Type         string
	CurrentValue string
}

func parseFormField(output []byte) ([]FormField, error) {
	parts := bytes.Split(output, []byte("\n---\n"))
	if len(parts) > 0 {
		parts[0] = bytes.TrimPrefix(parts[0], []byte("---\n"))
	}

	fields := make([]FormField, 0, len(parts))

	for _, p := range parts {
		field := FormField{}
		p = bytes.TrimRight(p, "\n")
		lines := bytes.Split(p, []byte{'\n'})
		for _, l := range lines {
			spl := bytes.SplitN(l, []byte(": "), 2)
			if len(spl) != 2 {
				return nil, fmt.Errorf("cannot parse form field: %q", l)
			}

			typ, str := string(spl[0]), string(spl[1])

			switch typ {
			case "FieldType":
				field.Type = html.UnescapeString(str)
			case "FieldName":
				field.Name = html.UnescapeString(str)
			case "FieldValue":
				field.CurrentValue = html.UnescapeString(str)
			}
		}
		fields = append(fields, field)
	}

	return fields, nil
}

func (e *Executor) CreateFromFile(path string) (res *FillPDF, retCleanup func(), retErr error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot open file %q: %w", path, err)
	}

	return e.Create(file)
}

func (e *Executor) CreateFromBytes(bs []byte) (res *FillPDF, retCleanup func(), retErr error) {
	return e.Create(bytes.NewReader(bs))
}

func (e *Executor) Create(input io.Reader) (res *FillPDF, retCleanup func(), retErr error) {
	cleanup := func() {}

	newDir, err := ioutil.TempDir("", "fillpdf-create")
	if err != nil {
		return nil, nil, fmt.Errorf("cannot create temporary directory for fillpdf: %w", err)
	}

	cleanup = func() {
		os.RemoveAll(newDir)
	}

	defer func() {
		if retErr != nil {
			cleanup()
		}
	}()

	newFile := path.Join(newDir, "input.pdf")
	destFile, err := os.Create(newFile)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot create file %s: %w", newFile, err)
	}

	defer destFile.Close()

	_, err = io.Copy(destFile, input)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot copy to file %s: %w", newFile, err)
	}

	fieldsBytes, err := runCommandInPath(newDir, e.pdftk, "input.pdf", "dump_data_fields")
	if err != nil {
		return nil, nil, fmt.Errorf("pdftk error when trying to dump fields %s: %w", newFile, err)
	}

	fields, err := parseFormField(fieldsBytes)

	names := make([]string, 0, len(fields))
	m := make(map[string]FormField, len(fields))
	for _, f := range fields {
		names = append(names, f.Name)
		m[f.Name] = f
	}

	return &FillPDF{
		dir:         newDir,
		fieldsNames: names,
		fields:      m,
	}, cleanup, nil
}

func (f *FillPDF) Fields() []FormField {
	r := make([]FormField, 0, len(f.fieldsNames))
	for _, fi := range f.fieldsNames {
		r = append(r, f.fields[fi])
	}
	return r
}

func (f *FillPDF) DefaultTextValues() map[string]string {
	r := make(map[string]string, len(f.fieldsNames))

	for k, v := range f.fields {
		if v.Type == "Text" {
			r[k] = k
		}
	}
	return r
}

func (f *FillPDF) AllButtonsTrue() map[string]bool {
	r := make(map[string]bool, len(f.fieldsNames))

	for k, v := range f.fields {
		if v.Type == "Button" {
			r[k] = true
		}
	}
	return r
}

func (f *FillPDF) FillToFile(out string, textValues map[string]string, buttonValues map[string]bool) error {
	destFile, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("cannot create file %s: %w", out, err)
	}
	defer destFile.Close()

	return f.Fill(destFile, textValues, buttonValues)
}

func (f *FillPDF) FillToBytes(textValues map[string]string, buttonValues map[string]bool) ([]byte, error) {
	rs := &bytes.Buffer{}

	if err := f.Fill(rs, textValues, buttonValues); err != nil {
		return nil, err
	}

	return rs.Bytes(), nil
}

func (f *FillPDF) Fill(out io.Writer, textValues map[string]string, buttonValues map[string]bool) error {
	for k := range textValues {
		fi, ok := f.fields[k]
		if !ok {
			return fmt.Errorf("field %q is not in the form", k)
		}
		if fi.Type != "Text" {
			return fmt.Errorf("field %q is not Text, is %q", k, fi.Type)
		}
	}

	inbs, err := createXfdfFile(textValues, buttonValues)
	if err != nil {
		return fmt.Errorf("cannot create FDF file: %w", err)
	}

	bs, err := runCommandInPathWithStdin(inbs, f.dir, f.e.java, "-jar", f.e.mcpdf, "input.pdf", "fill_form", "-", "output", "-")
	if err != nil {
		return fmt.Errorf("mcpdf error when trying to fill form: %w", err)
	}

	if _, err := io.Copy(out, bytes.NewReader(bs)); err != nil {
		return fmt.Errorf("cannot copy file to result: %w", err)
	}

	return nil
}

func createXfdfFile(textValues map[string]string, buttonValues map[string]bool) ([]byte, error) {
	const xfdfHeader = `<?xml version="1.0" encoding="UTF-8" standalone="no"?><xfdf><fields>`

	const xfdfFooter = `</fields></xfdf>`

	bsb := &bytes.Buffer{}

	if _, err := fmt.Fprintln(bsb, xfdfHeader); err != nil {
		return nil, fmt.Errorf("cannot print header: %w", err)
	}

	for key, value := range textValues {
		valueStr := html.EscapeString(value)
		if _, err := fmt.Fprintf(bsb, "<field name=\"%s\"><value>%s</value></field>", key, valueStr); err != nil {
			return nil, fmt.Errorf("cannot print field: %w", err)
		}
	}

	for key, value := range buttonValues {
		fill := "Off"
		if value {
			fill = "Yes"
		}
		if _, err := fmt.Fprintf(bsb, "<field name=\"%s\"><value>%s</value></field>", key, fill); err != nil {
			return nil, fmt.Errorf("cannot print field: %w", err)
		}
	}

	if _, err := fmt.Fprintln(bsb, xfdfFooter); err != nil {
		return nil, fmt.Errorf("cannot print footer: %w", err)
	}

	return bsb.Bytes(), nil
}
