package web

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/seanmcmahon101/Digital-Project-Management/internal/store"
)

const (
	maxXLSXPartSize = 64 << 20
	maxTransferRows = 10_000
	maxTransferCols = 64
)

var projectColumns = []string{
	"code", "name", "stage", "status", "sponsor", "lead", "department",
	"problem_statement", "goal", "current_state", "business_case", "scope_in",
	"scope_out", "start_date", "target_end", "go_live", "closed_at", "closure_summary",
}

func projectsTable(rows []store.ProjectTransferRow) [][]string {
	table := [][]string{append([]string(nil), projectColumns...)}
	for _, r := range rows {
		table = append(table, []string{r.Code, r.Name, r.Stage, r.Status, r.Sponsor, r.Lead,
			r.Department, r.ProblemStatement, r.Goal, r.CurrentState, r.BusinessCase,
			r.ScopeIn, r.ScopeOut, r.StartDate, r.TargetEnd, r.GoLive, r.ClosedAt, r.ClosureSummary})
	}
	return table
}

func writeProjectsCSV(w io.Writer, rows []store.ProjectTransferRow) error {
	// UTF-8 BOM lets desktop Excel recognise Unicode without an import wizard.
	if _, err := io.WriteString(w, "\ufeff"); err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	table := projectsTable(rows)
	for row := range table {
		for col := range table[row] {
			table[row][col] = safeCSVCell(table[row][col])
		}
	}
	err := cw.WriteAll(table)
	return err
}

// safeCSVCell prevents spreadsheet applications from interpreting exported
// user text as a formula. Quoting alone does not disable formula evaluation.
func safeCSVCell(value string) string {
	if value == "" {
		return value
	}
	switch value[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + value
	default:
		return value
	}
}

func readProjectsCSV(r io.Reader) ([]store.ProjectTransferRow, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.ReuseRecord = false
	table, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read CSV: %w", err)
	}
	if len(table) > 0 && len(table[0]) > 0 {
		table[0][0] = strings.TrimPrefix(table[0][0], "\ufeff")
	}
	return projectRowsFromTable(table)
}

func projectRowsFromTable(table [][]string) ([]store.ProjectTransferRow, error) {
	if len(table) == 0 {
		return nil, fmt.Errorf("the spreadsheet is empty")
	}
	if len(table)-1 > maxTransferRows {
		return nil, fmt.Errorf("the spreadsheet exceeds the %d-row import limit", maxTransferRows)
	}
	if len(table[0]) > maxTransferCols {
		return nil, fmt.Errorf("the spreadsheet contains too many columns")
	}
	indexes := map[string]int{}
	for i, h := range table[0] {
		h = strings.ToLower(strings.TrimSpace(h))
		indexes[h] = i
	}
	for _, required := range []string{"name"} {
		if _, ok := indexes[required]; !ok {
			return nil, fmt.Errorf("missing required column %q; start from an exported file", required)
		}
	}
	get := func(row []string, name string) string {
		i, ok := indexes[name]
		if !ok || i >= len(row) {
			return ""
		}
		return row[i]
	}
	var out []store.ProjectTransferRow
	for _, row := range table[1:] {
		allBlank := true
		for _, cell := range row {
			if strings.TrimSpace(cell) != "" {
				allBlank = false
				break
			}
		}
		if allBlank {
			continue
		}
		out = append(out, store.ProjectTransferRow{
			Code: get(row, "code"), Name: get(row, "name"), Stage: get(row, "stage"),
			Status: get(row, "status"), Sponsor: get(row, "sponsor"), Lead: get(row, "lead"),
			Department: get(row, "department"), ProblemStatement: get(row, "problem_statement"),
			Goal: get(row, "goal"), CurrentState: get(row, "current_state"),
			BusinessCase: get(row, "business_case"), ScopeIn: get(row, "scope_in"),
			ScopeOut: get(row, "scope_out"), StartDate: get(row, "start_date"),
			TargetEnd: get(row, "target_end"), GoLive: get(row, "go_live"), ClosedAt: get(row, "closed_at"),
			ClosureSummary: get(row, "closure_summary"),
		})
	}
	return out, nil
}

func writeProjectsXLSX(w io.Writer, rows []store.ProjectTransferRow) error {
	zw := zip.NewWriter(w)
	files := map[string]string{
		"[Content_Types].xml":        `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/></Types>`,
		"_rels/.rels":                `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`,
		"xl/workbook.xml":            `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Projects" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`,
		"xl/styles.xml":              `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><fonts count="2"><font><sz val="11"/><name val="Calibri"/></font><font><b/><sz val="11"/><color rgb="FFFFFFFF"/><name val="Calibri"/></font></fonts><fills count="3"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill><fill><patternFill patternType="solid"><fgColor rgb="FF5C1E30"/><bgColor indexed="64"/></patternFill></fill></fills><borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders><cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs><cellXfs count="2"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/><xf numFmtId="0" fontId="1" fillId="2" borderId="0" xfId="0" applyFont="1" applyFill="1"/></cellXfs></styleSheet>`,
		"xl/worksheets/sheet1.xml":   worksheetXML(projectsTable(rows)),
	}
	order := []string{"[Content_Types].xml", "_rels/.rels", "xl/workbook.xml", "xl/_rels/workbook.xml.rels", "xl/styles.xml", "xl/worksheets/sheet1.xml"}
	for _, name := range order {
		entry, err := zw.Create(name)
		if err != nil {
			_ = zw.Close()
			return err
		}
		if _, err := io.WriteString(entry, files[name]); err != nil {
			_ = zw.Close()
			return err
		}
	}
	return zw.Close()
}

func worksheetXML(table [][]string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews><cols>`)
	for i := range projectColumns {
		width := 18
		if i == 1 || (i >= 7 && i <= 12) || i == 17 {
			width = 32
		}
		fmt.Fprintf(&b, `<col min="%d" max="%d" width="%d" customWidth="1"/>`, i+1, i+1, width)
	}
	b.WriteString(`</cols><sheetData>`)
	for ri, row := range table {
		fmt.Fprintf(&b, `<row r="%d">`, ri+1)
		for ci, value := range row {
			style := ""
			if ri == 0 {
				style = ` s="1"`
			}
			fmt.Fprintf(&b, `<c r="%s%d" t="inlineStr"%s><is><t xml:space="preserve">`, columnName(ci+1), ri+1, style)
			_ = xml.EscapeText(&b, []byte(value))
			b.WriteString(`</t></is></c>`)
		}
		b.WriteString(`</row>`)
	}
	b.WriteString(`</sheetData><autoFilter ref="A1:R1"/></worksheet>`)
	return b.String()
}

func columnName(n int) string {
	var out string
	for n > 0 {
		n--
		out = string(rune('A'+n%26)) + out
		n /= 26
	}
	return out
}

func readProjectsXLSX(data []byte) ([]store.ProjectTransferRow, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open XLSX: %w", err)
	}
	var shared []string
	for _, f := range zr.File {
		if f.Name == "xl/sharedStrings.xml" {
			if f.UncompressedSize64 > maxXLSXPartSize {
				return nil, fmt.Errorf("XLSX shared strings are too large")
			}
			shared, err = readSharedStrings(f)
			if err != nil {
				return nil, fmt.Errorf("read XLSX shared strings: %w", err)
			}
		}
	}
	var sheets []*zip.File
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "xl/worksheets/sheet") && strings.HasSuffix(f.Name, ".xml") {
			sheets = append(sheets, f)
		}
	}
	if len(sheets) == 0 {
		return nil, fmt.Errorf("XLSX has no worksheets")
	}
	sort.Slice(sheets, func(i, j int) bool { return sheets[i].Name < sheets[j].Name })
	if sheets[0].UncompressedSize64 > maxXLSXPartSize {
		return nil, fmt.Errorf("XLSX worksheet is too large")
	}
	table, err := readWorksheet(sheets[0], shared)
	if err != nil {
		return nil, err
	}
	return projectRowsFromTable(table)
}

func readSharedStrings(f *zip.File) ([]string, error) {
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	dec := xml.NewDecoder(r)
	var out []string
	var current strings.Builder
	insideSI := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "si" {
				insideSI = true
				current.Reset()
			} else if insideSI && t.Name.Local == "t" {
				var value string
				if err := dec.DecodeElement(&value, &t); err != nil {
					return nil, err
				}
				current.WriteString(value)
			}
		case xml.EndElement:
			if t.Name.Local == "si" {
				out = append(out, current.String())
				insideSI = false
			}
		}
	}
}

func readWorksheet(f *zip.File, shared []string) ([][]string, error) {
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	dec := xml.NewDecoder(r)
	var table [][]string
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return table, nil
		}
		if err != nil {
			return nil, fmt.Errorf("read worksheet: %w", err)
		}
		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "c" {
			continue
		}
		ref, typ := "", ""
		for _, a := range start.Attr {
			if a.Name.Local == "r" {
				ref = a.Value
			}
			if a.Name.Local == "t" {
				typ = a.Value
			}
		}
		value, err := readCell(dec, start)
		if err != nil {
			return nil, err
		}
		if typ == "s" {
			i, _ := strconv.Atoi(value)
			if i >= 0 && i < len(shared) {
				value = shared[i]
			}
		}
		row, col := cellPosition(ref)
		if row < 1 || col < 1 {
			continue
		}
		if row > maxTransferRows+1 {
			return nil, fmt.Errorf("worksheet exceeds the %d-row import limit", maxTransferRows)
		}
		if col > maxTransferCols {
			return nil, fmt.Errorf("worksheet contains an unsupported column")
		}
		for len(table) < row {
			table = append(table, nil)
		}
		for len(table[row-1]) < col {
			table[row-1] = append(table[row-1], "")
		}
		table[row-1][col-1] = value
	}
}

func readCell(dec *xml.Decoder, start xml.StartElement) (string, error) {
	var value strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			return "", err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "v" || t.Name.Local == "t" {
				var part string
				if err := dec.DecodeElement(&part, &t); err != nil {
					return "", err
				}
				value.WriteString(part)
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				return value.String(), nil
			}
		}
	}
}

func cellPosition(ref string) (row, col int) {
	for _, r := range ref {
		if r >= 'A' && r <= 'Z' {
			col = col*26 + int(r-'A'+1)
		} else if r >= 'a' && r <= 'z' {
			col = col*26 + int(r-'a'+1)
		} else if r >= '0' && r <= '9' {
			row = row*10 + int(r-'0')
		}
	}
	return row, col
}
