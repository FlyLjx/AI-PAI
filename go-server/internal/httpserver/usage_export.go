package httpserver

import (
	"archive/zip"
	"bufio"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"aipi-go/internal/apiaccess"
)

const (
	usageWorkbookContentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
</Types>`
	usageWorkbookRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`
	usageWorkbookXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<sheets><sheet name="用量记录" sheetId="1" r:id="rId1"/></sheets>
</workbook>`
	usageWorkbookSheetRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
</Relationships>`
)

func (r *Router) exportUserUsageXLSX(w http.ResponseWriter, ctx context.Context, input apiaccess.ListLogsInput, startDate string, endDate string) {
	tempFile, err := os.CreateTemp("", "aipi-usage-*.xlsx")
	if err != nil {
		writeError(w, err)
		return
	}
	tempPath := tempFile.Name()
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		writeError(w, err)
		return
	}
	defer os.Remove(tempPath)

	rowCount, err := writeUsageWorkbook(ctx, tempPath, apiaccess.NewRepository(r.db), input)
	if err != nil {
		writeError(w, err)
		return
	}
	file, err := os.Open(tempPath)
	if err != nil {
		writeError(w, err)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="usage-%s-%s.xlsx"`, startDate, endDate))
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Export-Row-Count", strconv.Itoa(rowCount))
	_, _ = io.Copy(w, file)
}

func writeUsageWorkbook(ctx context.Context, path string, repo *apiaccess.Repository, input apiaccess.ListLogsInput) (int, error) {
	file, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	removeOnError := true
	defer func() {
		_ = file.Close()
		if removeOnError {
			_ = os.Remove(path)
		}
	}()

	archive := zip.NewWriter(file)
	writeEntry := func(name, value string) error {
		entry, err := archive.Create(name)
		if err != nil {
			return err
		}
		_, err = io.WriteString(entry, value)
		return err
	}
	if err := writeEntry("[Content_Types].xml", usageWorkbookContentTypes); err != nil {
		_ = archive.Close()
		return 0, err
	}
	if err := writeEntry("_rels/.rels", usageWorkbookRels); err != nil {
		_ = archive.Close()
		return 0, err
	}
	if err := writeEntry("xl/workbook.xml", usageWorkbookXML); err != nil {
		_ = archive.Close()
		return 0, err
	}
	if err := writeEntry("xl/_rels/workbook.xml.rels", usageWorkbookSheetRels); err != nil {
		_ = archive.Close()
		return 0, err
	}

	sheet, err := archive.Create("xl/worksheets/sheet1.xml")
	if err != nil {
		_ = archive.Close()
		return 0, err
	}
	bufferedSheet := bufio.NewWriter(sheet)
	if _, err := io.WriteString(bufferedSheet, `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews>
<cols>
<col min="1" max="1" width="20" customWidth="1"/><col min="2" max="2" width="25" customWidth="1"/>
<col min="3" max="3" width="24" customWidth="1"/><col min="4" max="4" width="16" customWidth="1"/>
<col min="5" max="5" width="22" customWidth="1"/><col min="6" max="6" width="12" customWidth="1"/>
<col min="7" max="7" width="12" customWidth="1"/><col min="8" max="8" width="14" customWidth="1"/>
<col min="9" max="9" width="12" customWidth="1"/><col min="10" max="10" width="40" customWidth="1"/>
</cols><sheetData>`); err != nil {
		_ = archive.Close()
		return 0, err
	}

	headers := []string{"请求时间", "接口地址", "任务 ID", "请求类型", "渠道 / 模型", "分辨率", "数量", "扣费金额", "状态", "错误信息"}
	if err := writeUsageXLSXRow(bufferedSheet, 1, headers, nil); err != nil {
		_ = archive.Close()
		return 0, err
	}
	rowCount := 0
	err = repo.StreamLogExportRows(ctx, input, func(item apiaccess.UsageExportRow) error {
		rowCount++
		values := []string{
			item.CreatedAt.Format("2006-01-02 15:04:05"),
			item.Endpoint,
			item.TaskID,
			exportRequestType(item.Endpoint),
			item.Model,
			item.Size,
			strconv.Itoa(item.Quantity),
			fmt.Sprintf("%.4f", item.ChargedCredits),
			exportStatusLabel(item.Status),
			item.ErrorMessage,
		}
		return writeUsageXLSXRow(bufferedSheet, rowCount+1, values, map[int]bool{6: true, 7: true})
	})
	if err != nil {
		_ = archive.Close()
		return 0, err
	}
	if _, err := io.WriteString(bufferedSheet, fmt.Sprintf(`</sheetData><autoFilter ref="A1:J%d"/></worksheet>`, rowCount+1)); err != nil {
		_ = archive.Close()
		return 0, err
	}
	if err := bufferedSheet.Flush(); err != nil {
		_ = archive.Close()
		return 0, err
	}
	if err := archive.Close(); err != nil {
		return 0, err
	}
	if err := file.Close(); err != nil {
		return 0, err
	}
	removeOnError = false
	return rowCount, nil
}

func writeUsageXLSXRow(w io.Writer, rowNumber int, values []string, numeric map[int]bool) error {
	if _, err := fmt.Fprintf(w, `<row r="%d">`, rowNumber); err != nil {
		return err
	}
	for index, value := range values {
		cellRef := excelColumnName(index+1) + strconv.Itoa(rowNumber)
		if numeric != nil && numeric[index] {
			if _, err := fmt.Fprintf(w, `<c r="%s"><v>%s</v></c>`, cellRef, xmlSafeText(value)); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintf(w, `<c r="%s" t="inlineStr"><is><t xml:space="preserve">`, cellRef); err != nil {
			return err
		}
		if err := xml.EscapeText(w, []byte(xmlSafeText(value))); err != nil {
			return err
		}
		if _, err := io.WriteString(w, `</t></is></c>`); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, `</row>`)
	return err
}

func excelColumnName(column int) string {
	name := ""
	for column > 0 {
		column--
		name = string(rune('A'+column%26)) + name
		column /= 26
	}
	return name
}

func xmlSafeText(value string) string {
	return strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' || r >= 0x20 {
			return r
		}
		return -1
	}, value)
}

func exportRequestType(endpoint string) string {
	if strings.Contains(strings.ToLower(endpoint), "/edits") {
		return "图生图"
	}
	return "文生图"
}

func exportStatusLabel(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "success", "succeeded":
		return "成功"
	case "failed":
		return "失败"
	case "processing":
		return "处理中"
	case "canceled", "cancelled":
		return "已取消"
	default:
		return "排队中"
	}
}
