package xlsx

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
)

type workbook struct {
	Sheets []sheetRef `xml:"sheets>sheet"`
}

type sheetRef struct {
	Name string `xml:"name,attr"`
	RID  string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
}

type relationships struct {
	Items []relationship `xml:"Relationship"`
}

type relationship struct {
	ID     string `xml:"Id,attr"`
	Target string `xml:"Target,attr"`
}

type sharedStrings struct {
	Items []sharedItem `xml:"si"`
}

type sharedItem struct {
	Text string    `xml:"t"`
	Runs []textRun `xml:"r"`
}

type textRun struct {
	Text string `xml:"t"`
}

type cell struct {
	Ref  string `xml:"r,attr"`
	Type string `xml:"t,attr"`
	V    string `xml:"v"`
	IS   struct {
		Text string `xml:"t"`
	} `xml:"is"`
}

type row struct {
	Cells []cell `xml:"c"`
}

type Row map[int]string

func StreamRows(filePath string, cb func(rowIndex int, values Row) error) error {
	zr, err := zip.OpenReader(filePath)
	if err != nil {
		return fmt.Errorf("open xlsx %s: %w", filePath, err)
	}
	defer zr.Close()

	sheetPath, err := firstSheetPath(&zr.Reader)
	if err != nil {
		return err
	}

	shared, err := loadSharedStrings(&zr.Reader)
	if err != nil {
		return err
	}

	rc, err := openZipFile(&zr.Reader, sheetPath)
	if err != nil {
		return err
	}
	defer rc.Close()

	dec := xml.NewDecoder(rc)
	rowIndex := 0

	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("decode sheet xml: %w", err)
		}

		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "row" {
			continue
		}

		var xr row
		if err := dec.DecodeElement(&xr, &start); err != nil {
			return fmt.Errorf("decode row: %w", err)
		}

		values := make(Row, len(xr.Cells))
		for _, c := range xr.Cells {
			col := colIndex(c.Ref)
			values[col] = cellValue(c, shared)
		}

		rowIndex++
		if err := cb(rowIndex, values); err != nil {
			return err
		}
	}
}

func firstSheetPath(zr *zip.Reader) (string, error) {
	workbookBytes, err := readZipFile(zr, "xl/workbook.xml")
	if err != nil {
		return "", err
	}
	relsBytes, err := readZipFile(zr, "xl/_rels/workbook.xml.rels")
	if err != nil {
		return "", err
	}

	var wb workbook
	if err := xml.Unmarshal(workbookBytes, &wb); err != nil {
		return "", fmt.Errorf("parse workbook.xml: %w", err)
	}
	if len(wb.Sheets) == 0 {
		return "", fmt.Errorf("xlsx has no sheets")
	}

	var rels relationships
	if err := xml.Unmarshal(relsBytes, &rels); err != nil {
		return "", fmt.Errorf("parse workbook rels: %w", err)
	}

	targetByID := make(map[string]string, len(rels.Items))
	for _, rel := range rels.Items {
		targetByID[rel.ID] = rel.Target
	}

	target, ok := targetByID[wb.Sheets[0].RID]
	if !ok {
		return "", fmt.Errorf("sheet relationship %s not found", wb.Sheets[0].RID)
	}
	return path.Clean(path.Join("xl", target)), nil
}

func loadSharedStrings(zr *zip.Reader) ([]string, error) {
	if !hasZipFile(zr, "xl/sharedStrings.xml") {
		return nil, nil
	}
	data, err := readZipFile(zr, "xl/sharedStrings.xml")
	if err != nil {
		return nil, err
	}
	var sst sharedStrings
	if err := xml.Unmarshal(data, &sst); err != nil {
		return nil, fmt.Errorf("parse sharedStrings.xml: %w", err)
	}
	out := make([]string, 0, len(sst.Items))
	for _, item := range sst.Items {
		if item.Text != "" {
			out = append(out, item.Text)
			continue
		}
		var b strings.Builder
		for _, run := range item.Runs {
			b.WriteString(run.Text)
		}
		out = append(out, b.String())
	}
	return out, nil
}

func cellValue(c cell, shared []string) string {
	switch c.Type {
	case "s":
		i, err := strconv.Atoi(strings.TrimSpace(c.V))
		if err != nil || i < 0 || i >= len(shared) {
			return ""
		}
		return strings.TrimSpace(shared[i])
	case "inlineStr":
		return strings.TrimSpace(c.IS.Text)
	default:
		return strings.TrimSpace(c.V)
	}
}

func colIndex(ref string) int {
	n := 0
	for _, r := range ref {
		if r >= 'A' && r <= 'Z' {
			n = n*26 + int(r-'A'+1)
			continue
		}
		if r >= 'a' && r <= 'z' {
			n = n*26 + int(r-'a'+1)
		}
	}
	return n
}

func openZipFile(zr *zip.Reader, name string) (io.ReadCloser, error) {
	for _, f := range zr.File {
		if path.Clean(f.Name) != path.Clean(name) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", name, err)
		}
		return rc, nil
	}
	return nil, fmt.Errorf("zip entry %s not found", name)
}

func readZipFile(zr *zip.Reader, name string) ([]byte, error) {
	rc, err := openZipFile(zr, name)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	return data, nil
}

func hasZipFile(zr *zip.Reader, name string) bool {
	for _, f := range zr.File {
		if path.Clean(f.Name) == path.Clean(name) {
			return true
		}
	}
	return false
}
