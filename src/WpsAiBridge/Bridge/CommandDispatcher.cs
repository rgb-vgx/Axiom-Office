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

        private static int _firstCommandLogged;

        public static object Execute(IAppHost host, string action, Dictionary<string, object> p)
        {
            // ai.ask giữ lệnh suốt các vòng gọi LLM (hàng chục giây); từng tool của nó tự qua ComGate.
            // ui.askpane chuyển sang UI thread; giữ cổng ở đây có thể deadlock với UI thread đang chờ cổng.
            bool gated = action != "ai.ask" && action != "ui.askpane";
            if (Interlocked.Exchange(ref _firstCommandLogged, 1) == 0)
            {
                try
                {
                    Logger.Info("First bridge command '" + action + "': " + ComGate.Run(delegate { return HostProbe.Describe(host); }));
                }
                catch (Exception ex)
                {
                    Logger.Error("First bridge command: document state unavailable", ex);
                }
            }
            int attempt = 0;
            while (true)
            {
                attempt++;
                try
                {
                    return gated
                        ? ComGate.Run(delegate { return ExecuteAction(host, action, p); })
                        : ExecuteAction(host, action, p);
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
            return HostProbe.IsRetryable(ex);
        }

        // Lệnh writer.* luôn nhắm vào tài liệu người dùng đang thấy: nếu ActiveDocument không có cửa sổ
        // hiển thị (tài liệu ẩn) mà có tài liệu khác đang hiển thị thì kích hoạt tài liệu đó trước.
        private static void EnsureVisibleWordDocument(IAppHost host, string action)
        {
            if (host.AppKind != "wps" || !action.StartsWith("writer.", StringComparison.Ordinal)
                || action == "writer.newDocument" || action == "writer.open" || action == "writer.closeAll")
            {
                return;
            }
            dynamic app = host.Application;
            dynamic active;
            try
            {
                active = app.ActiveDocument;
            }
            catch (COMException ex)
            {
                if (IsRetryableComError(ex))
                {
                    throw;
                }
                return;
            }
            if (IsWordDocumentVisible(active))
            {
                return;
            }
            dynamic documents = app.Documents;
            int count = Convert.ToInt32(documents.Count);
            for (int i = 1; i <= count; i++)
            {
                dynamic doc = documents[i];
                if (IsWordDocumentVisible(doc))
                {
                    Logger.Info("ActiveDocument '" + Convert.ToString(active.FullName) + "' has no visible window; activating '" +
                        Convert.ToString(doc.FullName) + "' for " + action);
                    doc.Activate();
                    return;
                }
            }
        }

        private static bool IsWordDocumentVisible(dynamic doc)
        {
            try
            {
                dynamic windows = doc.Windows;
                if (Convert.ToInt32(windows.Count) == 0)
                {
                    return false;
                }
                return Convert.ToBoolean(windows[1].Visible);
            }
            catch (COMException ex)
            {
                if (IsRetryableComError(ex))
                {
                    throw;
                }
                return true;
            }
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
            EnsureVisibleWordDocument(host, action);
            switch (action)
            {
                    case "app.info":
                        return Ok(AppInfo(host));

                    case "ai.ask":
                    {
                        string prompt = ParamString(p, "prompt", null);
                        if (string.IsNullOrEmpty(prompt))
                        {
                            throw new InvalidOperationException("'prompt' is required");
                        }
                        Ai.LlmResult agentResult = Ai.AiAgent.Run(host, prompt, delegate(string line)
                        {
                            Logger.Info("ai.ask progress: " + line);
                        });
                        var reply = new Dictionary<string, object>();
                        reply["ok"] = agentResult.Ok;
                        if (agentResult.Ok)
                        {
                            reply["reply"] = agentResult.Text;
                        }
                        else
                        {
                            reply["error"] = agentResult.Error;
                        }
                        reply["transcript"] = agentResult.Transcript;
                        reply["seconds"] = agentResult.Seconds;
                        return Ok(reply);
                    }

                    case "ui.askpane":
                    {
                        Connect connect = host as Connect;
                        if (connect == null)
                        {
                            return Err("ui.askpane is only available inside the in-process add-in");
                        }
                        bool shown = connect.TryShowTaskPane();
                        return Ok(new Dictionary<string, object> { { "taskPane", shown } });
                    }

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
                    case "writer.insertStyledText":
                        return Ok(WriterInsertStyledText(host, p));
                    case "writer.formatSelection":
                        return Ok(WriterFormatSelection(host, p));
                    case "writer.setParagraphAlignment":
                        return Ok(WriterSetParagraphAlignment(host, p));
                    case "writer.insertTable":
                        return Ok(WriterInsertTable(host, p));
                    case "writer.insertPageBreak":
                        return Ok(WriterInsertPageBreak(host));
                    case "writer.insertImage":
                        return Ok(WriterInsertImage(host, p));
                    case "writer.insertHyperlink":
                        return Ok(WriterInsertHyperlink(host, p));
                    case "writer.heading":
                        return Ok(WriterHeading(host, p));
                    case "writer.undo":
                        return Ok(WriterUndo(host, p));
                    case "writer.exportPdf":
                        return Ok(WriterExportPdf(host, p));
                    case "writer.closeAll":
                        return Ok(WriterCloseAll(host));
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
                    case "et.formatRange":
                        return Ok(EtFormatRange(host, p));
                    case "et.activateSheet":
                        return Ok(EtActivateSheet(host, p));
                    case "et.exportPdf":
                        return Ok(EtExportPdf(host, p));
                    case "et.undo":
                        return Ok(EtUndo(host, p));
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
                        return Ok(WppAddSlide(host, p));
                    case "wpp.addTextBox":
                        return Ok(WppAddTextBox(host, p));
                    case "wpp.addText":
                        return Ok(WppAddText(host, p));
                    case "wpp.addImage":
                        return Ok(WppAddImage(host, p));
                    case "wpp.addTable":
                        return Ok(WppAddTable(host, p));
                    case "wpp.setNotes":
                        return Ok(WppSetNotes(host, p));
                    case "wpp.deleteSlide":
                        return Ok(WppDeleteSlide(host, p));
                    case "wpp.exportPdf":
                        return Ok(WppExportPdf(host, p));
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

            info["state"] = Safe(() => HostProbe.Describe(host));
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
            using (new UndoRecordScope(host.Application, "AI: type text"))
            {
                app.Selection.TypeText(text);
            }
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
            using (new UndoRecordScope(host.Application, "AI: append text"))
            {
                doc.Content.InsertAfter(text);
            }
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
            dynamic result;
            using (new UndoRecordScope(host.Application, "AI: replace all"))
            {
                result = doc.Content.Find.Execute(find, false, false, false, false, false, true, 1, false, replace, 2);
            }
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

        private sealed class UndoRecordScope : IDisposable
        {
            private readonly object _undoRecord;

            public UndoRecordScope(object app, string label)
            {
                try
                {
                    dynamic dynamicApp = app;
                    object record = dynamicApp.UndoRecord;
                    if (record != null)
                    {
                        dynamic dynamicRecord = record;
                        dynamicRecord.StartCustomRecord(label);
                        _undoRecord = record;
                    }
                }
                catch
                {
                    _undoRecord = null;
                }
            }

            public void Dispose()
            {
                if (_undoRecord != null)
                {
                    try
                    {
                        dynamic dynamicRecord = _undoRecord;
                        dynamicRecord.EndCustomRecord();
                    }
                    catch
                    {
                    }
                }
            }
        }

        private static bool HasParam(Dictionary<string, object> p, string name)
        {
            object v;
            return p != null && p.TryGetValue(name, out v) && v != null;
        }

        private static int? ParseBgrColor(string hex)
        {
            if (string.IsNullOrEmpty(hex))
            {
                return null;
            }
            string text = hex.Trim().TrimStart('#');
            if (text.Length != 6)
            {
                return null;
            }
            try
            {
                int r = Convert.ToInt32(text.Substring(0, 2), 16);
                int g = Convert.ToInt32(text.Substring(2, 2), 16);
                int b = Convert.ToInt32(text.Substring(4, 2), 16);
                return r + (g << 8) + (b << 16);
            }
            catch
            {
                return null;
            }
        }

        private static int ParseAlignment(string alignment)
        {
            switch ((alignment ?? "left").ToLowerInvariant())
            {
                case "center":
                    return 1;
                case "right":
                    return 2;
                case "justify":
                    return 3;
                default:
                    return 0;
            }
        }

        private static void ApplyWriterFont(dynamic font, Dictionary<string, object> p)
        {
            if (HasParam(p, "bold"))
            {
                font.Bold = ParamBool(p, "bold", false) ? -1 : 0;
            }
            if (HasParam(p, "italic"))
            {
                font.Italic = ParamBool(p, "italic", false) ? -1 : 0;
            }
            if (HasParam(p, "underline"))
            {
                font.Underline = ParamBool(p, "underline", false) ? 1 : 0;
            }
            if (HasParam(p, "size"))
            {
                font.Size = ParamInt(p, "size", 12);
            }
            if (HasParam(p, "font"))
            {
                font.Name = ParamString(p, "font", null);
            }
            int? color = ParseBgrColor(ParamString(p, "color", null));
            if (color.HasValue)
            {
                font.Color = color.Value;
            }
        }

        private static Dictionary<string, object> WriterInsertStyledText(IAppHost host, Dictionary<string, object> p)
        {
            RequireKind(host, "wps");
            string text = ParamString(p, "text", null);
            if (string.IsNullOrEmpty(text))
            {
                return new Dictionary<string, object> { { "inserted", 0 } };
            }
            dynamic app = host.Application;
            using (new UndoRecordScope(host.Application, "AI: insert styled text"))
            {
                ApplyWriterFont(app.Selection.Font, p);
                app.Selection.TypeText(text);
            }
            return new Dictionary<string, object> { { "inserted", text.Length } };
        }

        private static Dictionary<string, object> WriterFormatSelection(IAppHost host, Dictionary<string, object> p)
        {
            RequireKind(host, "wps");
            dynamic app = host.Application;
            using (new UndoRecordScope(host.Application, "AI: format selection"))
            {
                ApplyWriterFont(app.Selection.Font, p);
                if (HasParam(p, "alignment"))
                {
                    app.Selection.ParagraphFormat.Alignment = ParseAlignment(ParamString(p, "alignment", "left"));
                }
            }
            return new Dictionary<string, object> { { "formatted", true } };
        }

        private static Dictionary<string, object> WriterSetParagraphAlignment(IAppHost host, Dictionary<string, object> p)
        {
            RequireKind(host, "wps");
            string alignment = ParamString(p, "alignment", "left");
            dynamic app = host.Application;
            using (new UndoRecordScope(host.Application, "AI: paragraph alignment"))
            {
                app.Selection.ParagraphFormat.Alignment = ParseAlignment(alignment);
            }
            return new Dictionary<string, object> { { "alignment", alignment } };
        }

        private static Dictionary<string, object> WriterInsertTable(IAppHost host, Dictionary<string, object> p)
        {
            RequireKind(host, "wps");
            int rows = ParamInt(p, "rows", 0);
            int cols = ParamInt(p, "cols", 0);
            if (rows <= 0 || cols <= 0)
            {
                throw new InvalidOperationException("'rows' and 'cols' are required");
            }
            dynamic app = host.Application;
            int filled = 0;
            using (new UndoRecordScope(host.Application, "AI: insert table"))
            {
                dynamic table = app.ActiveDocument.Tables.Add(app.Selection.Range, rows, cols);
                IList valueRows = null;
                object rawValues;
                if (p != null && p.TryGetValue("values", out rawValues))
                {
                    valueRows = rawValues as IList;
                }
                if (valueRows != null)
                {
                    for (int r = 0; r < valueRows.Count && r < rows; r++)
                    {
                        IList row = valueRows[r] as IList;
                        if (row == null)
                        {
                            continue;
                        }
                        for (int c = 0; c < row.Count && c < cols; c++)
                        {
                            table.Cell(r + 1, c + 1).Range.Text = Convert.ToString(row[c]);
                            filled++;
                        }
                    }
                }
                if (HasParam(p, "style"))
                {
                    try
                    {
                        table.set_Style(ParamString(p, "style", "Table Grid"));
                    }
                    catch
                    {
                    }
                }
            }
            return new Dictionary<string, object>
            {
                { "rows", rows },
                { "cols", cols },
                { "filled", filled }
            };
        }

        private static Dictionary<string, object> WriterInsertPageBreak(IAppHost host)
        {
            RequireKind(host, "wps");
            dynamic app = host.Application;
            using (new UndoRecordScope(host.Application, "AI: page break"))
            {
                app.Selection.InsertBreak(7);
            }
            return new Dictionary<string, object> { { "inserted", true } };
        }

        private static Dictionary<string, object> WriterInsertImage(IAppHost host, Dictionary<string, object> p)
        {
            RequireKind(host, "wps");
            string path = ParamString(p, "path", null);
            if (string.IsNullOrEmpty(path))
            {
                throw new InvalidOperationException("'path' is required");
            }
            dynamic app = host.Application;
            using (new UndoRecordScope(host.Application, "AI: insert image"))
            {
                dynamic shape = app.Selection.InlineShapes.AddPicture(path, false, true);
                if (HasParam(p, "width"))
                {
                    shape.Width = ParamInt(p, "width", 0);
                }
                if (HasParam(p, "height"))
                {
                    shape.Height = ParamInt(p, "height", 0);
                }
                return new Dictionary<string, object>
                {
                    { "width", Convert.ToDouble(shape.Width) },
                    { "height", Convert.ToDouble(shape.Height) }
                };
            }
        }

        private static Dictionary<string, object> WriterInsertHyperlink(IAppHost host, Dictionary<string, object> p)
        {
            RequireKind(host, "wps");
            string url = ParamString(p, "url", null);
            if (string.IsNullOrEmpty(url))
            {
                throw new InvalidOperationException("'url' is required");
            }
            string text = ParamString(p, "text", null);
            if (string.IsNullOrEmpty(text))
            {
                text = url;
            }
            dynamic app = host.Application;
            using (new UndoRecordScope(host.Application, "AI: insert hyperlink"))
            {
                int start = Convert.ToInt32(app.Selection.Start);
                app.Selection.TypeText(text);
                int end = Convert.ToInt32(app.Selection.Start);
                dynamic range = app.ActiveDocument.Range(start, end);
                app.ActiveDocument.Hyperlinks.Add(range, url);
            }
            return new Dictionary<string, object> { { "text", text }, { "url", url } };
        }

        private static Dictionary<string, object> WriterHeading(IAppHost host, Dictionary<string, object> p)
        {
            RequireKind(host, "wps");
            int level = Math.Max(1, Math.Min(9, ParamInt(p, "level", 1)));
            string text = ParamString(p, "text", null);
            dynamic app = host.Application;
            using (new UndoRecordScope(host.Application, "AI: heading"))
            {
                if (!string.IsNullOrEmpty(text))
                {
                    app.Selection.TypeText(text);
                }
                string styleName = "Heading " + level;
                try
                {
                    app.Selection.set_Style(styleName);
                }
                catch
                {
                    app.Selection.Style = styleName;
                }
                if (!string.IsNullOrEmpty(text) && ParamBool(p, "break", true))
                {
                    try
                    {
                        app.Selection.TypeParagraph();
                    }
                    catch
                    {
                    }
                }
            }
            return new Dictionary<string, object> { { "level", level } };
        }

        private static Dictionary<string, object> WriterUndo(IAppHost host, Dictionary<string, object> p)
        {
            RequireKind(host, "wps");
            int count = Math.Max(1, ParamInt(p, "count", 1));
            dynamic app = host.Application;
            dynamic doc = app.ActiveDocument;
            int undone = 0;
            for (int i = 0; i < count; i++)
            {
                try
                {
                    doc.Undo(1);
                    undone++;
                }
                catch (COMException ex)
                {
                    if (IsRetryableComError(ex))
                    {
                        throw;
                    }
                    Logger.Info("writer.undo stopped: " + ex.Message);
                    break;
                }
                catch (Exception ex)
                {
                    Logger.Info("writer.undo stopped: " + ex.Message);
                    break;
                }
            }
            return new Dictionary<string, object> { { "undone", undone } };
        }

        private static Dictionary<string, object> WriterExportPdf(IAppHost host, Dictionary<string, object> p)
        {
            RequireKind(host, "wps");
            string path = ParamString(p, "path", null);
            if (string.IsNullOrEmpty(path))
            {
                throw new InvalidOperationException("'path' is required");
            }
            dynamic app = host.Application;
            dynamic doc = app.ActiveDocument;
            doc.ExportAsFixedFormat(path, 17);
            return new Dictionary<string, object> { { "exported", path } };
        }

        private static Dictionary<string, object> WriterCloseAll(IAppHost host)
        {
            RequireKind(host, "wps");
            dynamic app = host.Application;
            int closed = 0;
            try
            {
                app.DisplayAlerts = 0;
            }
            catch
            {
            }
            try
            {
                dynamic documents = app.Documents;
                int count = Convert.ToInt32(documents.Count);
                for (int i = count; i >= 1; i--)
                {
                    try
                    {
                        documents[i].Close(0);
                        closed++;
                    }
                    catch
                    {
                    }
                }
            }
            finally
            {
                try
                {
                    app.DisplayAlerts = -1;
                }
                catch
                {
                }
            }
            return new Dictionary<string, object> { { "closed", closed } };
        }

        private static Dictionary<string, object> EtFormatRange(IAppHost host, Dictionary<string, object> p)
        {
            RequireKind(host, "et");
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
            RequireKind(host, "et");
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
            RequireKind(host, "et");
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
            RequireKind(host, "et");
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

        private static dynamic WppGetSlide(dynamic pres, int slideIndex)
        {
            int count = Convert.ToInt32(pres.Slides.Count);
            if (count == 0)
            {
                throw new InvalidOperationException("presentation has no slides; call wpp.addSlide first");
            }
            if (slideIndex < 1 || slideIndex > count)
            {
                slideIndex = count;
            }
            return pres.Slides[slideIndex];
        }

        private static Dictionary<string, object> WppAddSlide(IAppHost host, Dictionary<string, object> p)
        {
            RequireKind(host, "wpp");
            int layout = ParamInt(p, "layout", 12);
            dynamic app = host.Application;
            dynamic pres = app.ActivePresentation;
            int index = Convert.ToInt32(pres.Slides.Count) + 1;
            pres.Slides.Add(index, layout);
            return new Dictionary<string, object> { { "slide", index }, { "layout", layout } };
        }

        private static Dictionary<string, object> WppAddText(IAppHost host, Dictionary<string, object> p)
        {
            RequireKind(host, "wpp");
            string text = ParamString(p, "text", "");
            int left = ParamInt(p, "left", 60);
            int top = ParamInt(p, "top", 60);
            int width = ParamInt(p, "width", 540);
            int height = ParamInt(p, "height", 120);
            dynamic app = host.Application;
            dynamic pres = app.ActivePresentation;
            dynamic slide = WppGetSlide(pres, ParamInt(p, "slide", 0));
            dynamic shape = slide.Shapes.AddTextbox(1, left, top, width, height);
            dynamic textRange = shape.TextFrame.TextRange;
            textRange.Text = text;
            dynamic font = textRange.Font;
            if (HasParam(p, "fontSize"))
            {
                font.Size = ParamInt(p, "fontSize", 18);
            }
            if (HasParam(p, "bold"))
            {
                font.Bold = ParamBool(p, "bold", false) ? -1 : 0;
            }
            int? color = ParseBgrColor(ParamString(p, "color", null));
            if (color.HasValue)
            {
                font.Color = color.Value;
            }
            if (HasParam(p, "align"))
            {
                string align = ParamString(p, "align", "left").ToLowerInvariant();
                textRange.ParagraphFormat.Alignment = align == "center" ? 2 : (align == "right" ? 3 : 1);
            }
            return new Dictionary<string, object>
            {
                { "slide", Convert.ToInt32(slide.SlideIndex) },
                { "shape", Convert.ToString(shape.Name) }
            };
        }

        private static Dictionary<string, object> WppAddImage(IAppHost host, Dictionary<string, object> p)
        {
            RequireKind(host, "wpp");
            string path = ParamString(p, "path", null);
            if (string.IsNullOrEmpty(path))
            {
                throw new InvalidOperationException("'path' is required");
            }
            int left = ParamInt(p, "left", 60);
            int top = ParamInt(p, "top", 60);
            int width = ParamInt(p, "width", -1);
            int height = ParamInt(p, "height", -1);
            dynamic app = host.Application;
            dynamic pres = app.ActivePresentation;
            dynamic slide = WppGetSlide(pres, ParamInt(p, "slide", 0));
            dynamic shape = slide.Shapes.AddPicture(path, 0, -1, left, top, width, height);
            return new Dictionary<string, object>
            {
                { "slide", Convert.ToInt32(slide.SlideIndex) },
                { "shape", Convert.ToString(shape.Name) }
            };
        }

        private static Dictionary<string, object> WppAddTable(IAppHost host, Dictionary<string, object> p)
        {
            RequireKind(host, "wpp");
            int rows = ParamInt(p, "rows", 0);
            int cols = ParamInt(p, "cols", 0);
            if (rows <= 0 || cols <= 0)
            {
                throw new InvalidOperationException("'rows' and 'cols' are required");
            }
            int left = ParamInt(p, "left", 60);
            int top = ParamInt(p, "top", 120);
            int width = ParamInt(p, "width", 600);
            int height = ParamInt(p, "height", 200);
            dynamic app = host.Application;
            dynamic pres = app.ActivePresentation;
            dynamic slide = WppGetSlide(pres, ParamInt(p, "slide", 0));
            dynamic tableShape = slide.Shapes.AddTable(rows, cols, left, top, width, height);
            dynamic table = tableShape.Table;
            int filled = 0;
            IList valueRows = null;
            object rawValues;
            if (p != null && p.TryGetValue("values", out rawValues))
            {
                valueRows = rawValues as IList;
            }
            if (valueRows != null)
            {
                for (int r = 0; r < valueRows.Count && r < rows; r++)
                {
                    IList row = valueRows[r] as IList;
                    if (row == null)
                    {
                        continue;
                    }
                    for (int c = 0; c < row.Count && c < cols; c++)
                    {
                        table.Cell(r + 1, c + 1).Shape.TextFrame.TextRange.Text = Convert.ToString(row[c]);
                        filled++;
                    }
                }
            }
            return new Dictionary<string, object>
            {
                { "slide", Convert.ToInt32(slide.SlideIndex) },
                { "rows", rows },
                { "cols", cols },
                { "filled", filled }
            };
        }

        private static Dictionary<string, object> WppSetNotes(IAppHost host, Dictionary<string, object> p)
        {
            RequireKind(host, "wpp");
            string text = ParamString(p, "text", "");
            dynamic app = host.Application;
            dynamic pres = app.ActivePresentation;
            dynamic slide = WppGetSlide(pres, ParamInt(p, "slide", 0));
            dynamic notesPage = slide.NotesPage;
            dynamic target = null;
            int shapeCount = Convert.ToInt32(notesPage.Shapes.Count);
            for (int i = 1; i <= shapeCount; i++)
            {
                dynamic shape = notesPage.Shapes[i];
                try
                {
                    if (Convert.ToInt32(shape.HasTextFrame) == -1 && Convert.ToInt32(shape.PlaceholderFormat.Type) == 2)
                    {
                        target = shape;
                        break;
                    }
                }
                catch
                {
                }
            }
            if (target == null)
            {
                for (int i = 1; i <= shapeCount; i++)
                {
                    dynamic shape = notesPage.Shapes[i];
                    try
                    {
                        if (Convert.ToInt32(shape.HasTextFrame) == -1)
                        {
                            target = shape;
                            break;
                        }
                    }
                    catch
                    {
                    }
                }
            }
            if (target == null)
            {
                throw new InvalidOperationException("notes text placeholder not found");
            }
            target.TextFrame.TextRange.Text = text;
            return new Dictionary<string, object> { { "slide", Convert.ToInt32(slide.SlideIndex) }, { "notes", text.Length } };
        }

        private static Dictionary<string, object> WppDeleteSlide(IAppHost host, Dictionary<string, object> p)
        {
            RequireKind(host, "wpp");
            dynamic app = host.Application;
            dynamic pres = app.ActivePresentation;
            dynamic slide = WppGetSlide(pres, ParamInt(p, "slide", 0));
            int index = Convert.ToInt32(slide.SlideIndex);
            slide.Delete();
            return new Dictionary<string, object>
            {
                { "deleted", index },
                { "slideCount", Convert.ToInt32(pres.Slides.Count) }
            };
        }

        private static Dictionary<string, object> WppExportPdf(IAppHost host, Dictionary<string, object> p)
        {
            RequireKind(host, "wpp");
            string path = ParamString(p, "path", null);
            if (string.IsNullOrEmpty(path))
            {
                throw new InvalidOperationException("'path' is required");
            }
            dynamic app = host.Application;
            dynamic pres = app.ActivePresentation;
            try
            {
                pres.ExportAsFixedFormat(path, 2);
            }
            catch
            {
                pres.SaveAs(path, 32);
            }
            return new Dictionary<string, object> { { "exported", path } };
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
