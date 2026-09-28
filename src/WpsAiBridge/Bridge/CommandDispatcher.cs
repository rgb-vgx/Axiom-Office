using System;
using System.Collections;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using System.Threading;

namespace WpsAiBridge.Bridge
{
    internal static class CommandDispatcher
    {
        public static readonly string BridgeVersion =
            typeof(CommandDispatcher).Assembly.GetName().Version.ToString(3);

        private const int MaxComRetries = 10;

        public static object Execute(IAppHost host, string action, Dictionary<string, object> p)
        {
            int attempt = 0;
            while (true)
            {
                attempt++;
                try
                {
                    return ExecuteAction(host, action, p);
                }
                catch (COMException ex)
                {
                    if (IsRetryableComError(ex) && attempt <= MaxComRetries)
                    {
                        int delayMs = Math.Min(100 * attempt, 1000);
                        Logger.Info("COM call rejected [" + ex.HResult.ToString("X8") + "], retry " + attempt + "/" + MaxComRetries + " in " + delayMs + "ms");
                        Thread.Sleep(delayMs);
                        continue;
                    }
                    Logger.Error("Action failed: " + action, ex);
                    return Err(ex.GetType().Name + ": " + ex.Message);
                }
                catch (Exception ex)
                {
                    Logger.Error("Action failed: " + action, ex);
                    return Err(ex.GetType().Name + ": " + ex.Message);
                }
            }
        }

        private static bool IsRetryableComError(COMException ex)
        {
            int hr = ex.HResult;
            return hr == unchecked((int)0x80010001)
                || hr == unchecked((int)0x8001010A)
                || hr == unchecked((int)0x80010002)
                || hr == unchecked((int)0x800AC472);
        }

        public static object Health(IAppHost host, int port)
        {
            return Ok(new Dictionary<string, object>
            {
                { "app", host.AppKind },
                { "pid", System.Diagnostics.Process.GetCurrentProcess().Id },
                { "port", port },
                { "version", BridgeVersion },
                { "log", Logger.LogFilePath }
            });
        }

        public static object ConfigInfo()
        {
            return Ok(new Dictionary<string, object>
            {
                { "provider", Config.LlmProvider },
                { "endpoint", Config.LlmEndpoint },
                { "model", Config.LlmModel },
                { "apiKey", MaskKey(Config.LlmApiKey) }
            });
        }

        private static string MaskKey(string key)
        {
            if (string.IsNullOrEmpty(key))
            {
                return "";
            }
            if (key.Length <= 8)
            {
                return "********";
            }
            return key.Substring(0, 4) + "..." + key.Substring(key.Length - 4);
        }

        private static object ExecuteAction(IAppHost host, string action, Dictionary<string, object> p)
        {
            switch (action)
            {
                    case "app.info":
                        return Ok(AppInfo(host));

                    case "writer.getText":
                        return Ok(WriterGetText(host, p));
                    case "writer.newDocument":
                        return Ok(WriterNewDocument(host));
                    case "writer.open":
                        return Ok(WriterOpen(host, p));
                    case "writer.selection":
                        return Ok(WriterSelection(host));
                    case "writer.typeText":
                        return Ok(WriterTypeText(host, p));
                    case "writer.appendText":
                        return Ok(WriterAppendText(host, p));
                    case "writer.replaceAll":
                        return Ok(WriterReplaceAll(host, p));
                    case "writer.save":
                        return Ok(SaveDocument(host, "wps", null));
                    case "writer.saveAs":
                        return Ok(SaveDocument(host, "wps", ParamString(p, "path", null)));

                    case "et.listSheets":
                        return Ok(EtListSheets(host));
                    case "et.newWorkbook":
                        return Ok(EtNewWorkbook(host));
                    case "et.open":
                        return Ok(EtOpen(host, p));
                    case "et.readRange":
                        return Ok(EtReadRange(host, p));
                    case "et.writeRange":
                        return Ok(EtWriteRange(host, p));
                    case "et.save":
                        return Ok(SaveDocument(host, "et", null));
                    case "et.saveAs":
                        return Ok(SaveDocument(host, "et", ParamString(p, "path", null)));

                    case "wpp.listSlides":
                        return Ok(WppListSlides(host));
                    case "wpp.newPresentation":
                        return Ok(WppNewPresentation(host));
                    case "wpp.open":
                        return Ok(WppOpen(host, p));
                    case "wpp.addSlide":
                        return Ok(WppAddSlide(host));
                    case "wpp.addTextBox":
                        return Ok(WppAddTextBox(host, p));
                    case "wpp.save":
                        return Ok(SaveDocument(host, "wpp", null));
                    case "wpp.saveAs":
                        return Ok(SaveDocument(host, "wpp", ParamString(p, "path", null)));

                    default:
                        return Err("unknown action: " + action);
            }
        }

        private static Dictionary<string, object> Ok(object result)
        {
            return new Dictionary<string, object> { { "ok", true }, { "result", result } };
        }

        private static Dictionary<string, object> Err(string message)
        {
            return new Dictionary<string, object> { { "ok", false }, { "error", message } };
        }

        private static string ParamString(Dictionary<string, object> p, string name, string fallback)
        {
            object v;
            if (p != null && p.TryGetValue(name, out v) && v != null)
            {
                return Convert.ToString(v);
            }
            return fallback;
        }

        private static int ParamInt(Dictionary<string, object> p, string name, int fallback)
        {
            object v;
            if (p != null && p.TryGetValue(name, out v) && v != null)
            {
                return Convert.ToInt32(v);
            }
            return fallback;
        }

        private static bool ParamBool(Dictionary<string, object> p, string name, bool fallback)
        {
            object v;
            if (p != null && p.TryGetValue(name, out v) && v != null)
            {
                if (v is bool)
                {
                    return (bool)v;
                }
                return Convert.ToBoolean(v);
            }
            return fallback;
        }

        private static void RequireKind(IAppHost host, string kind)
        {
            if (host.AppKind != kind)
            {
                throw new InvalidOperationException(
                    "command requires '" + kind + "' but the connected application is '" + host.AppKind + "'");
            }
        }

        private static string ExcelErrorName(int code)
        {
            switch (code)
            {
                case -2146826288:
                    return "#NULL!";
                case -2146826281:
                    return "#DIV/0!";
                case -2146826273:
                    return "#VALUE!";
                case -2146826265:
                    return "#REF!";
                case -2146826259:
                    return "#NAME?";
                case -2146826252:
                    return "#NUM!";
                default:
                    return null;
            }
        }

        private static object Safe(Func<object> getter)
        {
            try
            {
                return getter();
            }
            catch
            {
                return null;
            }
        }

        private static object ToPlain(object value)
        {
            if (value == null)
            {
                return null;
            }
            if (value is int)
            {
                int intValue = (int)value;
                if (intValue == -2146826246 || intValue == -2147352572)
                {
                    return null;
                }
                string excelError = ExcelErrorName(intValue);
                if (excelError != null)
                {
                    return excelError;
                }
                return intValue;
            }
            if (value is string || value is bool || value is long || value is double || value is decimal)
            {
                return value;
            }
            if (value is float)
            {
                return (double)(float)value;
            }
            if (value is short)
            {
                return (int)(short)value;
            }
            if (value is byte)
            {
                return (int)(byte)value;
            }
            if (value is DateTime)
            {
                return ((DateTime)value).ToString("o");
            }
            if (value is Array)
            {
                return ToMatrix(value);
            }
            return Convert.ToString(value);
        }

        private static List<List<object>> ToMatrix(object value)
        {
            var rows = new List<List<object>>();
            Array array = value as Array;
            if (array == null)
            {
                rows.Add(new List<object> { ToPlain(value) });
                return rows;
            }

            int r0 = array.GetLowerBound(0);
            int r1 = array.GetUpperBound(0);
            int c0 = array.GetLowerBound(1);
            int c1 = array.GetUpperBound(1);
            for (int r = r0; r <= r1; r++)
            {
                var row = new List<object>();
                for (int c = c0; c <= c1; c++)
                {
                    row.Add(ToPlain(array.GetValue(r, c)));
                }
                rows.Add(row);
            }
            return rows;
        }

        private static Dictionary<string, object> AppInfo(IAppHost host)
        {
            dynamic app = host.Application;
            var info = new Dictionary<string, object>
            {
                { "kind", host.AppKind },
                { "name", Safe(() => Convert.ToString(app.Name)) },
                { "version", Safe(() => Convert.ToString(app.Version)) }
            };

            if (host.AppKind == "wps")
            {
                info["documents"] = Safe(() => Convert.ToInt32(app.Documents.Count));
                info["activeDocument"] = Safe(() => new Dictionary<string, object>
                {
                    { "name", Convert.ToString(app.ActiveDocument.Name) },
                    { "fullName", Convert.ToString(app.ActiveDocument.FullName) },
                    { "saved", Convert.ToBoolean(app.ActiveDocument.Saved) }
                });
            }
            else if (host.AppKind == "et")
            {
                info["workbooks"] = Safe(() => Convert.ToInt32(app.Workbooks.Count));
                info["activeWorkbook"] = Safe(() => new Dictionary<string, object>
                {
                    { "name", Convert.ToString(app.ActiveWorkbook.Name) },
                    { "fullName", Convert.ToString(app.ActiveWorkbook.FullName) },
                    { "saved", Convert.ToBoolean(app.ActiveWorkbook.Saved) },
                    { "sheet", Convert.ToString(app.ActiveSheet.Name) }
                });
            }
            else if (host.AppKind == "wpp")
            {
                info["presentations"] = Safe(() => Convert.ToInt32(app.Presentations.Count));
                info["activePresentation"] = Safe(() => new Dictionary<string, object>
                {
                    { "name", Convert.ToString(app.ActivePresentation.Name) },
                    { "fullName", Convert.ToString(app.ActivePresentation.FullName) },
                    { "saved", Convert.ToBoolean(app.ActivePresentation.Saved) },
                    { "slides", Convert.ToInt32(app.ActivePresentation.Slides.Count) }
                });
            }
            return info;
        }

        private static Dictionary<string, object> WriterGetText(IAppHost host, Dictionary<string, object> p)
        {
            RequireKind(host, "wps");
            dynamic app = host.Application;
            dynamic doc = app.ActiveDocument;
            string text = Convert.ToString(doc.Content.Text);
            int totalChars = text.Length;
            int maxChars = ParamInt(p, "maxChars", 0);
            bool truncated = false;
            if (maxChars > 0 && text.Length > maxChars)
            {
                text = text.Substring(0, maxChars);
                truncated = true;
            }
            return new Dictionary<string, object>
            {
                { "name", Convert.ToString(doc.Name) },
                { "fullName", Convert.ToString(doc.FullName) },
                { "totalChars", totalChars },
                { "truncated", truncated },
                { "text", text }
            };
        }

        private static Dictionary<string, object> WriterNewDocument(IAppHost host)
        {
            RequireKind(host, "wps");
            dynamic app = host.Application;
            dynamic doc = app.Documents.Add();
            return new Dictionary<string, object>
            {
                { "name", Convert.ToString(doc.Name) }
            };
        }

        private static Dictionary<string, object> WriterOpen(IAppHost host, Dictionary<string, object> p)
        {
            RequireKind(host, "wps");
            string path = ParamString(p, "path", null);
            if (string.IsNullOrEmpty(path))
            {
                throw new InvalidOperationException("'path' is required");
            }
            dynamic app = host.Application;
            dynamic doc = app.Documents.Open(path);
            return new Dictionary<string, object>
            {
                { "name", Convert.ToString(doc.Name) },
                { "fullName", Convert.ToString(doc.FullName) }
            };
        }

        private static Dictionary<string, object> WriterSelection(IAppHost host)
        {
            RequireKind(host, "wps");
            dynamic app = host.Application;
            dynamic selection = app.Selection;
            return new Dictionary<string, object>
            {
                { "text", Convert.ToString(selection.Text) },
                { "start", Convert.ToInt32(selection.Start) },
                { "end", Convert.ToInt32(selection.End) }
            };
        }

        private static Dictionary<string, object> WriterTypeText(IAppHost host, Dictionary<string, object> p)
        {
            RequireKind(host, "wps");
            string text = ParamString(p, "text", null);
            if (text == null)
            {
                return new Dictionary<string, object> { { "typed", 0 } };
            }
            dynamic app = host.Application;
            app.Selection.TypeText(text);
            return new Dictionary<string, object> { { "typed", text.Length } };
        }

        private static Dictionary<string, object> WriterAppendText(IAppHost host, Dictionary<string, object> p)
        {
            RequireKind(host, "wps");
            string text = ParamString(p, "text", null);
            if (text == null)
            {
                return new Dictionary<string, object> { { "appended", 0 } };
            }
            dynamic app = host.Application;
            dynamic doc = app.ActiveDocument;
            doc.Content.InsertAfter(text);
            return new Dictionary<string, object> { { "appended", text.Length } };
        }

        private static Dictionary<string, object> WriterReplaceAll(IAppHost host, Dictionary<string, object> p)
        {
            RequireKind(host, "wps");
            string find = ParamString(p, "find", null);
            string replace = ParamString(p, "replace", "");
            if (string.IsNullOrEmpty(find))
            {
                return new Dictionary<string, object> { { "replaced", false } };
            }
            dynamic app = host.Application;
            dynamic doc = app.ActiveDocument;
            dynamic result = doc.Content.Find.Execute(find, false, false, false, false, false, true, 1, false, replace, 2);
            return new Dictionary<string, object> { { "replaced", Convert.ToBoolean(result) } };
        }

        private static Dictionary<string, object> EtListSheets(IAppHost host)
        {
            RequireKind(host, "et");
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

        private static Dictionary<string, object> EtNewWorkbook(IAppHost host)
        {
            RequireKind(host, "et");
            dynamic app = host.Application;
            dynamic wb = app.Workbooks.Add();
            return new Dictionary<string, object>
            {
                { "name", Convert.ToString(wb.Name) }
            };
        }

        private static Dictionary<string, object> EtOpen(IAppHost host, Dictionary<string, object> p)
        {
            RequireKind(host, "et");
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
            RequireKind(host, "et");
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
            RequireKind(host, "et");
            string address = ParamString(p, "range", null);
            if (string.IsNullOrEmpty(address))
            {
                return new Dictionary<string, object> { { "written", 0 } };
            }
            string sheetName = ParamString(p, "sheet", null);
            object rawValues;
            if (p == null || !p.TryGetValue("values", out rawValues) || rawValues == null)
            {
                return new Dictionary<string, object> { { "written", 0 } };
            }

            IList rowList = rawValues as IList;
            if (rowList == null || rowList.Count == 0)
            {
                return new Dictionary<string, object> { { "written", 0 } };
            }

            int rowCount = rowList.Count;
            int colCount = 1;
            for (int i = 0; i < rowCount; i++)
            {
                IList probeRow = rowList[i] as IList;
                if (probeRow != null && probeRow.Count > colCount)
                {
                    colCount = probeRow.Count;
                }
            }

            object[,] matrix = new object[rowCount, colCount];
            for (int r = 0; r < rowCount; r++)
            {
                IList row = rowList[r] as IList;
                if (row == null)
                {
                    IList single = new object[] { rowList[r] };
                    row = single;
                }
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

        private static Dictionary<string, object> WppListSlides(IAppHost host)
        {
            RequireKind(host, "wpp");
            dynamic app = host.Application;
            dynamic pres = app.ActivePresentation;
            int count = Convert.ToInt32(pres.Slides.Count);
            var slides = new List<object>();
            for (int i = 1; i <= count; i++)
            {
                dynamic slide = pres.Slides[i];
                var texts = new List<string>();
                int shapeCount = Convert.ToInt32(slide.Shapes.Count);
                for (int s = 1; s <= shapeCount; s++)
                {
                    try
                    {
                        dynamic shape = slide.Shapes[s];
                        if (Convert.ToInt32(shape.HasTextFrame) == -1 && Convert.ToInt32(shape.TextFrame.HasText) == -1)
                        {
                            texts.Add(Convert.ToString(shape.TextFrame.TextRange.Text));
                        }
                    }
                    catch
                    {
                    }
                }
                slides.Add(new Dictionary<string, object>
                {
                    { "index", i },
                    { "shapeTexts", texts }
                });
            }
            return new Dictionary<string, object>
            {
                { "presentation", Convert.ToString(pres.Name) },
                { "slideCount", count },
                { "slides", slides }
            };
        }

        private static Dictionary<string, object> WppNewPresentation(IAppHost host)
        {
            RequireKind(host, "wpp");
            dynamic app = host.Application;
            dynamic pres = app.Presentations.Add();
            return new Dictionary<string, object>
            {
                { "name", Convert.ToString(pres.Name) }
            };
        }

        private static Dictionary<string, object> WppOpen(IAppHost host, Dictionary<string, object> p)
        {
            RequireKind(host, "wpp");
            string path = ParamString(p, "path", null);
            if (string.IsNullOrEmpty(path))
            {
                throw new InvalidOperationException("'path' is required");
            }
            dynamic app = host.Application;
            dynamic pres = app.Presentations.Open(path);
            return new Dictionary<string, object>
            {
                { "name", Convert.ToString(pres.Name) },
                { "fullName", Convert.ToString(pres.FullName) }
            };
        }

        private static Dictionary<string, object> WppAddSlide(IAppHost host)
        {
            RequireKind(host, "wpp");
            dynamic app = host.Application;
            dynamic pres = app.ActivePresentation;
            int index = Convert.ToInt32(pres.Slides.Count) + 1;
            pres.Slides.Add(index, 12);
            return new Dictionary<string, object> { { "slide", index } };
        }

        private static Dictionary<string, object> WppAddTextBox(IAppHost host, Dictionary<string, object> p)
        {
            RequireKind(host, "wpp");
            string text = ParamString(p, "text", null);
            int slideIndex = ParamInt(p, "slide", 0);
            int left = ParamInt(p, "left", 60);
            int top = ParamInt(p, "top", 60);
            int width = ParamInt(p, "width", 540);
            int height = ParamInt(p, "height", 120);
            dynamic app = host.Application;
            dynamic pres = app.ActivePresentation;
            int count = Convert.ToInt32(pres.Slides.Count);
            if (count == 0)
            {
                throw new InvalidOperationException("presentation has no slides; call wpp.addSlide first");
            }
            if (slideIndex < 1 || slideIndex > count)
            {
                slideIndex = count;
            }
            dynamic slide = pres.Slides[slideIndex];
            dynamic shape = slide.Shapes.AddTextbox(1, left, top, width, height);
            if (text != null)
            {
                shape.TextFrame.TextRange.Text = text;
            }
            return new Dictionary<string, object>
            {
                { "slide", slideIndex },
                { "shape", Convert.ToString(shape.Name) }
            };
        }

        private static Dictionary<string, object> SaveDocument(IAppHost host, string kind, string path)
        {
            RequireKind(host, kind);
            dynamic app = host.Application;
            dynamic target;
            if (kind == "wps")
            {
                target = app.ActiveDocument;
            }
            else if (kind == "et")
            {
                target = app.ActiveWorkbook;
            }
            else
            {
                target = app.ActivePresentation;
            }

            if (!string.IsNullOrEmpty(path))
            {
                try
                {
                    target.SaveAs2(path);
                }
                catch
                {
                    target.SaveAs(path);
                }
                return new Dictionary<string, object>
                {
                    { "saved", true },
                    { "fullName", Convert.ToString(target.FullName) }
                };
            }

            target.Save();
            return new Dictionary<string, object>
            {
                { "saved", true },
                { "fullName", Convert.ToString(target.FullName) }
            };
        }
    }
}
