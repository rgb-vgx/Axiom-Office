using System;
using System.Collections;
using System.Collections.Generic;
using System.Linq;
using System.Runtime.InteropServices;

namespace AxiomOffice.Bridge
{
    internal static partial class CommandDispatcher
    {
        // Excel / WPS Spreadsheets (AppKind "et").
        private static IEnumerable<CommandInfo> SpreadsheetCommands()
        {
            return new[]
            {
                Command("et.newWorkbook", "et", EtNewWorkbook, "Tạo workbook mới").ForAgent(),
                Command("et.open", "et", EtOpen, "Mở .xlsx/.xls/.csv", Req("path")),
                Command("et.listSheets", "et", EtListSheets, "Danh sách sheet + sheet đang active").ForAgent(),
                Command("et.activateSheet", "et", EtActivateSheet, "Chuyển sheet", Req("sheet")).ForAgent(),
                Command("et.readRange", "et", EtReadRange, "Đọc vùng, ví dụ `A1:C10`", Req("range"), Opt("sheet")).ForAgent(),
                Command("et.writeRange", "et", EtWriteRange, "Ghi vùng bắt đầu từ ô trên-trái `range`",
                    Req("range", "top-left cell e.g. 'A1'"), Req("values", "2D array of rows e.g. [[\"Tên\",\"Điểm\"],[\"An\",9.5]]"), Opt("sheet")).ForAgent(),
                Command("et.formatRange", "et", EtFormatRange, "Định dạng vùng (màu dạng `#RRGGBB`, `horizontal` left/center/right)",
                    Req("range"), Opt("bold"), Opt("italic"), Opt("fontSize"), Opt("fontColor"), Opt("fillColor"), Opt("numFmt"), Opt("horizontal"), Opt("wrap"), Opt("sheet")).ForAgent(),
                Command("et.undo", "et", EtUndo, "Hoàn tác", Opt("count")).ForAgent(),
                Command("et.exportPdf", "et", EtExportPdf, "Xuất PDF", Req("path")).ForAgent(),
                Command("et.save", "et", (host, p) => SaveDocument(host, "et", null), "Lưu").ForAgent(),
                Command("et.saveAs", "et", (host, p) => SaveDocument(host, "et", ParamString(p, "path", null)), "Lưu thành file mới", Req("path")).ForAgent()
            };
        }

        private static Dictionary<string, object> EtListSheets(IAppHost host, Dictionary<string, object> p)
        {
            dynamic app = host.Application;
            dynamic wb = app.ActiveWorkbook;
            int count = Convert.ToInt32(wb.Worksheets.Count);
            var sheets = new List<object>();
            for (int i = 1; i <= count; i++)
            {
                sheets.Add(Convert.ToString(wb.Worksheets[i].Name));
            }
            return new Dictionary<string, object>
            {
                { "workbook", Convert.ToString(wb.Name) },
                { "activeSheet", Convert.ToString(app.ActiveSheet.Name) },
                { "sheets", sheets }
            };
        }

        private static Dictionary<string, object> EtNewWorkbook(IAppHost host, Dictionary<string, object> p)
        {
            dynamic app = host.Application;
            dynamic wb = app.Workbooks.Add();
            return new Dictionary<string, object>
            {
                { "name", Convert.ToString(wb.Name) }
            };
        }

        private static Dictionary<string, object> EtOpen(IAppHost host, Dictionary<string, object> p)
        {
            string path = ParamString(p, "path", null);
            if (string.IsNullOrEmpty(path))
            {
                throw new InvalidOperationException("'path' is required");
            }
            dynamic app = host.Application;
            dynamic wb = app.Workbooks.Open(path);
            return new Dictionary<string, object>
            {
                { "name", Convert.ToString(wb.Name) },
                { "fullName", Convert.ToString(wb.FullName) }
            };
        }

        private static Dictionary<string, object> EtReadRange(IAppHost host, Dictionary<string, object> p)
        {
            string address = ParamString(p, "range", "A1");
            string sheetName = ParamString(p, "sheet", null);
            dynamic app = host.Application;
            dynamic wb = app.ActiveWorkbook;
            dynamic sheet = string.IsNullOrEmpty(sheetName) ? wb.ActiveSheet : wb.Worksheets[sheetName];
            dynamic range = sheet.Range[address];
            object value = range.Value2;
            return new Dictionary<string, object>
            {
                { "sheet", Convert.ToString(sheet.Name) },
                { "range", address },
                { "values", ToMatrix(value) }
            };
        }

        private static Dictionary<string, object> EtWriteRange(IAppHost host, Dictionary<string, object> p)
        {
            string address = ParamString(p, "range", null);
            if (string.IsNullOrEmpty(address))
            {
                throw new ArgumentException("'range' is required: the top-left cell to write at, e.g. 'A1'");
            }
            string sheetName = ParamString(p, "sheet", null);
            List<IList> rowList = ParamMatrix(p, "values", true);

            int rowCount = rowList.Count;
            int colCount = Math.Max(1, rowList.Max(row => row.Count));
            object[,] matrix = new object[rowCount, colCount];
            for (int r = 0; r < rowCount; r++)
            {
                IList row = rowList[r];
                for (int c = 0; c < colCount; c++)
                {
                    matrix[r, c] = c < row.Count ? row[c] : null;
                }
            }

            dynamic app = host.Application;
            dynamic wb = app.ActiveWorkbook;
            dynamic sheet = string.IsNullOrEmpty(sheetName) ? wb.ActiveSheet : wb.Worksheets[sheetName];
            dynamic target = sheet.Range[address];
            try
            {
                target = target.Resize[rowCount, colCount];
            }
            catch
            {
            }
            target.Value2 = matrix;

            return new Dictionary<string, object>
            {
                { "written", rowCount * colCount },
                { "sheet", Convert.ToString(sheet.Name) }
            };
        }

        private static Dictionary<string, object> EtFormatRange(IAppHost host, Dictionary<string, object> p)
        {
            string address = ParamString(p, "range", null);
            if (string.IsNullOrEmpty(address))
            {
                throw new InvalidOperationException("'range' is required");
            }
            string sheetName = ParamString(p, "sheet", null);
            dynamic app = host.Application;
            dynamic wb = app.ActiveWorkbook;
            dynamic sheet = string.IsNullOrEmpty(sheetName) ? wb.ActiveSheet : wb.Worksheets[sheetName];
            dynamic range = sheet.Range[address];
            dynamic font = range.Font;
            if (HasParam(p, "bold"))
            {
                font.Bold = ParamBool(p, "bold", false);
            }
            if (HasParam(p, "italic"))
            {
                font.Italic = ParamBool(p, "italic", false);
            }
            if (HasParam(p, "fontSize"))
            {
                font.Size = ParamInt(p, "fontSize", 11);
            }
            int? fontColor = ParseBgrColor(ParamString(p, "fontColor", null));
            if (fontColor.HasValue)
            {
                font.Color = fontColor.Value;
            }
            int? fillColor = ParseBgrColor(ParamString(p, "fillColor", null));
            if (fillColor.HasValue)
            {
                range.Interior.Color = fillColor.Value;
            }
            if (HasParam(p, "numFmt"))
            {
                range.NumberFormat = ParamString(p, "numFmt", "General");
            }
            if (HasParam(p, "horizontal"))
            {
                string horizontal = ParamString(p, "horizontal", "left").ToLowerInvariant();
                range.HorizontalAlignment = horizontal == "center" ? -4108 : (horizontal == "right" ? -4152 : -4131);
            }
            if (HasParam(p, "wrap"))
            {
                range.WrapText = ParamBool(p, "wrap", false);
            }
            return new Dictionary<string, object>
            {
                { "sheet", Convert.ToString(sheet.Name) },
                { "range", address }
            };
        }

        private static Dictionary<string, object> EtActivateSheet(IAppHost host, Dictionary<string, object> p)
        {
            string sheetName = ParamString(p, "sheet", null);
            if (string.IsNullOrEmpty(sheetName))
            {
                throw new InvalidOperationException("'sheet' is required");
            }
            dynamic app = host.Application;
            dynamic wb = app.ActiveWorkbook;
            wb.Worksheets[sheetName].Activate();
            return new Dictionary<string, object> { { "active", sheetName } };
        }

        private static Dictionary<string, object> EtExportPdf(IAppHost host, Dictionary<string, object> p)
        {
            string path = ParamString(p, "path", null);
            if (string.IsNullOrEmpty(path))
            {
                throw new InvalidOperationException("'path' is required");
            }
            dynamic app = host.Application;
            dynamic wb = app.ActiveWorkbook;
            wb.ExportAsFixedFormat(0, path);
            return new Dictionary<string, object> { { "exported", path } };
        }

        private static Dictionary<string, object> EtUndo(IAppHost host, Dictionary<string, object> p)
        {
            int count = Math.Max(1, ParamInt(p, "count", 1));
            dynamic app = host.Application;
            int undone = 0;
            for (int i = 0; i < count; i++)
            {
                try
                {
                    app.Undo();
                    undone++;
                }
                catch (COMException ex)
                {
                    if (IsRetryableComError(ex))
                    {
                        throw;
                    }
                    Logger.Info("et.undo stopped: " + ex.Message);
                    break;
                }
                catch (Exception ex)
                {
                    Logger.Info("et.undo stopped: " + ex.Message);
                    break;
                }
            }
            return new Dictionary<string, object> { { "undone", undone } };
        }
    }
}
