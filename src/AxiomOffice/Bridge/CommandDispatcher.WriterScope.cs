using System;
using System.Collections.Generic;

namespace AxiomOffice.Bridge
{
    // Khoá vùng bôi đen (người dùng 16/10/2026: "khi bôi đen thì AI chỉ tác động trong vùng bôi đen đó thôi"). Khung chat
    // khoá khi gửi yêu cầu lúc đang bôi đen, mở khi lượt xong; muốn AI sửa cả tài liệu thì bỏ bôi đen trước khi hỏi.
    //
    // Vùng khoá được nhớ bằng phần chữ TRƯỚC và SAU nó chứ không bằng một Range: AI viết lại cả đoạn đã chọn (typeText
    // thay vùng chọn) thì Range cũ có thể co về một điểm, còn "giữa phần trước và phần sau" luôn đúng dù chữ bên trong
    // dài ra hay ngắn đi. Mọi lệnh writer.* ghi đều chạy trong một bước hoàn tác; phần ngoài vùng đổi thì hoàn tác
    // ngay bước đó và báo lỗi cho model.
    internal static partial class CommandDispatcher
    {
        private sealed class SelectionLockState
        {
            public string Document;
            public int PrefixEnd;
            public int SuffixLength;
            public string Prefix;
            public string Suffix;
            // Số bảng NẰM TRỌN trong vùng khoá: bảng trong vùng là bất khả xâm phạm (xem LockTableRefused).
            public int TableCount;
        }

        private static readonly object LockSync = new object();
        private static SelectionLockState _selectionLock;

        // Lệnh đọc (không đổi tài liệu) chạy như thường khi đang khoá.
        private static readonly HashSet<string> LockReadOnly = new HashSet<string>(StringComparer.Ordinal)
        {
            "writer.getText", "writer.selection", "writer.checkTables", "writer.outline", "writer.checkLayout", "writer.undo",
            "writer.save", "writer.saveAs", "writer.exportPdf"
        };

        // Lệnh luôn chạm ra ngoài vùng chọn: từ chối, nói model dùng lệnh nào thay.
        private static readonly Dictionary<string, string> LockRefused = new Dictionary<string, string>(StringComparer.Ordinal)
        {
            { "writer.appendText", "appends at the end of the document" },
            { "writer.restyle", "restyles the whole document" },
            { "writer.insertBlocks", "inserts blocks across the document" },
            { "writer.newDocument", "creates another document" },
            { "writer.open", "opens another document" },
            { "writer.closeAll", "closes the documents" },
            // InsertBreak của Word THAY cả vùng chọn bằng dấu ngắt: khoá vùng luôn có vùng chọn nên lệnh này
            // xoá cả đoạn đã bôi đen. Muốn ngắt trang thì bỏ bôi đen rồi yêu cầu.
            { "writer.insertPageBreak", "replaces the whole selected passage with a page break" },
            { "writer.insertToc", "inserts a table of contents outside the selection" },
            { "writer.setHeaderFooter", "changes the headers and footers of the whole document" },
            { "writer.insertCaption", "inserts a new paragraph" },
            { "writer.writeContent", "appends at the end of the document" },
            { "writer.deleteParagraphs", "deletes whole paragraphs by number" }
        };

        // Lệnh THAY cả vùng chọn bằng chữ mới: vô hại với vùng chữ, nhưng nếu vùng chứa TRỌN một bảng thì bảng bị
        // xoá, thay bằng chữ thường (lượt thật 16/10/2026: typeText 3530 ký tự đè lên bảng trong giáo án). Từ chối
        // và chỉ đường sửa chữ trong ô; lưới an toàn sau khi chạy (TableCount) bắt nốt các lệnh còn lại.
        private static readonly Dictionary<string, string> LockTableRefused = new Dictionary<string, string>(StringComparer.Ordinal)
        {
            { "writer.typeText", "it would replace the table with plain text" },
            { "writer.insertStyledText", "it would replace the table with plain text" },
            { "writer.heading", "it would replace or restyle the table as a heading" },
            { "writer.insertHyperlink", "it would replace the table with the link text" }
        };

        // writer.lockSelection / writer.unlockSelection: khung chat gọi thẳng LockSelection; hai lệnh này cho client
        // khác (khung chat WPS, script thử) - không mở cho agent.
        private static IEnumerable<CommandInfo> WriterScopeCommands()
        {
            return new[]
            {
                Command("writer.lockSelection", "wps", (host, p) => new Dictionary<string, object> { { "chars", LockSelection(host) } },
                    "Khoá vùng đang bôi đen: tới khi mở khoá, lệnh writer.* chỉ được đổi chữ trong vùng đó (chars = 0: không bôi đen, không khoá)"),
                Command("writer.unlockSelection", "wps", (host, p) =>
                {
                    UnlockSelection();
                    return new Dictionary<string, object> { { "locked", false } };
                }, "Mở khoá vùng bôi đen")
            };
        }

        public static bool SelectionLocked
        {
            get { return _selectionLock != null; }
        }

        // Khoá vùng đang bôi đen của tài liệu Word đang mở. Trả số ký tự của vùng (0 = không bôi đen, không khoá).
        public static int LockSelection(IAppHost host)
        {
            UnlockSelection();
            if (host == null || host.AppKind != "wps")
            {
                return 0;
            }
            return ComGate.Run(delegate
            {
                dynamic app = host.Application;
                dynamic selection = app.Selection;
                int start = Convert.ToInt32(selection.Start);
                int end = Convert.ToInt32(selection.End);
                string text = Convert.ToString(selection.Text) ?? "";
                // Con trỏ đứng yên (Word trả 1 ký tự) hoặc chỉ bôi khoảng trắng: không phải vùng chọn.
                if (end - start < 2 || text.Trim().Length < 2)
                {
                    return 0;
                }
                dynamic doc = app.ActiveDocument;
                int docEnd = Convert.ToInt32(doc.Content.End);
                var state = new SelectionLockState
                {
                    Document = Convert.ToString(doc.FullName),
                    PrefixEnd = start,
                    SuffixLength = docEnd - end,
                    Prefix = RangeText(doc, 0, start),
                    Suffix = RangeText(doc, end, docEnd),
                    TableCount = CountWholeTables((object)doc, start, end)
                };
                lock (LockSync)
                {
                    _selectionLock = state;
                }
                Logger.Info("Selection lock: " + state.Document + " [" + start + ", " + end + ") of " + docEnd
                    + (state.TableCount > 0 ? " (" + state.TableCount + " whole table(s))" : ""));
                return end - start;
            });
        }

        public static void UnlockSelection()
        {
            lock (LockSync)
            {
                if (_selectionLock != null)
                {
                    Logger.Info("Selection lock released");
                }
                _selectionLock = null;
            }
        }

        // Khoá đang áp cho tài liệu này: trả vị trí đầu/cuối hiện tại của vùng khoá.
        private static bool LockBounds(object document, out int start, out int end)
        {
            dynamic doc = document;
            SelectionLockState state = _selectionLock;
            start = 0;
            end = 0;
            if (state == null || !string.Equals(Convert.ToString(doc.FullName), state.Document, StringComparison.OrdinalIgnoreCase))
            {
                return false;
            }
            start = state.PrefixEnd;
            end = Math.Max(start, Convert.ToInt32(doc.Content.End) - state.SuffixLength);
            return true;
        }

        // Chạy một lệnh writer.* trong vùng khoá (ExecuteAction gọi cho mọi lệnh).
        private static object RunInSelectionLock(IAppHost host, string action, CommandInfo command, Dictionary<string, object> p)
        {
            SelectionLockState state = _selectionLock;
            if (state == null || host.AppKind != "wps" || !action.StartsWith("writer.", StringComparison.Ordinal)
                || LockReadOnly.Contains(action))
            {
                return command.Handler(host, p);
            }
            dynamic app = host.Application;
            dynamic doc = app.ActiveDocument;
            int start, end;
            if (!LockBounds((object)doc, out start, out end))
            {
                return command.Handler(host, p);
            }
            string reason;
            if (LockRefused.TryGetValue(action, out reason))
            {
                throw new CommandRejectedException(SelectionLockMessage(action + " " + reason));
            }
            if (state.TableCount > 0 && LockTableRefused.TryGetValue(action, out reason))
            {
                Logger.Info("Selection lock: refused " + action + " (whole table in selection)");
                throw new CommandRejectedException(TableLockMessage(reason));
            }
            if (action == "writer.formatTable")
            {
                RequireTableInLock((object)doc, p, start, end);
            }
            // Lệnh theo con trỏ/vùng chọn (typeText, formatSelection, heading...): con trỏ ra ngoài vùng khoá thì chọn
            // lại cả vùng khoá.
            dynamic selection = app.Selection;
            if (Convert.ToInt32(selection.Start) < start || Convert.ToInt32(selection.End) > end)
            {
                selection.SetRange(start, end);
            }
            object result;
            using (new UndoRecordScope(app, "AI: " + action))
            {
                result = command.Handler(host, p);
            }
            if (!OutsideUnchanged((object)doc, state))
            {
                try
                {
                    doc.Undo(1);
                }
                catch (Exception ex)
                {
                    Logger.Error("Selection lock: undo failed after " + action, ex);
                }
                Logger.Info("Selection lock: " + action + " changed text outside the selection - undone");
                throw new CommandRejectedException(SelectionLockMessage(action + " changed text outside the selection, so it was undone"));
            }
            // Lưới an toàn: lệnh nào làm MẤT một bảng nằm trọn trong vùng khoá thì hoàn tác ngay (bắt cả lệnh
            // không nằm trong danh sách chặn trước, vd insertImage nếu Word thay vùng chọn bằng ảnh).
            if (state.TableCount > 0 && LockBounds((object)doc, out start, out end)
                && CountWholeTables((object)doc, start, end) < state.TableCount)
            {
                try
                {
                    doc.Undo(1);
                }
                catch (Exception ex)
                {
                    Logger.Error("Selection lock: undo failed after " + action, ex);
                }
                Logger.Info("Selection lock: " + action + " destroyed a table in the selection - undone");
                throw new CommandRejectedException(TableLockMessage(action + " changed the table structure, so it was undone"));
            }
            return result;
        }

        private static bool OutsideUnchanged(object document, SelectionLockState state)
        {
            dynamic doc = document;
            int docEnd = Convert.ToInt32(doc.Content.End);
            int suffixStart = docEnd - state.SuffixLength;
            if (suffixStart < state.PrefixEnd)
            {
                return false;
            }
            return string.Equals(RangeText(doc, 0, state.PrefixEnd), state.Prefix, StringComparison.Ordinal)
                && string.Equals(RangeText(doc, suffixStart, docEnd), state.Suffix, StringComparison.Ordinal);
        }

        // Chữ của [start, end). Khoảng rỗng thì "" mà không gọi Range: Word ném "Value out of range" cho Range(n, n) tại
        // cuối tài liệu (n = Content.End) - bôi đen cả tài liệu (Ctrl+A) rồi khoá là dính (đo 25/10/2026, Word x64).
        private static string RangeText(object document, int start, int end)
        {
            if (end <= start)
            {
                return "";
            }
            dynamic doc = document;
            return Convert.ToString(doc.Range(start, end).Text) ?? "";
        }

        // Đếm số bảng NẰM TRỌN trong [start, end): bảng nằm trọn mới có thể bị xoá cả khi thay vùng chọn;
        // bảng nằm một phần (con trỏ trong ô) vẫn sửa chữ bình thường được.
        private static int CountWholeTables(object document, int start, int end)
        {
            dynamic doc = document;
            dynamic content = doc.Range(start, end);
            int whole = 0;
            int count = Convert.ToInt32(content.Tables.Count);
            for (int index = 1; index <= count; index++)
            {
                dynamic range = content.Tables[index].Range;
                if (Convert.ToInt32(range.Start) >= start && Convert.ToInt32(range.End) <= end)
                {
                    whole++;
                }
            }
            return whole;
        }

        private static void RequireTableInLock(object document, Dictionary<string, object> p, int start, int end)
        {
            dynamic doc = document;
            int index = ParamInt(p, "table", 0);
            if (index <= 0)
            {
                // Mặc định là bảng tại con trỏ - con trỏ đã nằm trong vùng khoá.
                return;
            }
            if (index > Convert.ToInt32(doc.Tables.Count))
            {
                return;
            }
            dynamic range = doc.Tables[index].Range;
            if (Convert.ToInt32(range.Start) < start || Convert.ToInt32(range.End) > end)
            {
                throw new CommandRejectedException(SelectionLockMessage("table " + index + " is outside the selection"));
            }
        }

        private static string SelectionLockMessage(string problem)
        {
            return "Refused: the user selected part of the document, so only the selected text may change (" + problem +
                "). Edit inside the selection: writer.replaceAll/replaceMany search only the selection, writer.typeText " +
                "replaces it, writer.formatSelection formats it. If the request really needs other parts, stop and tell " +
                "the user to clear the selection and ask again.";
        }

        private static string TableLockMessage(string problem)
        {
            return "Refused: the selection contains a whole table; " + problem + ". The table is protected: edit text " +
                "in its cells with writer.replaceAll/replaceMany (they search only inside the selection) and restyle " +
                "with writer.formatTable. If the user really wants the table removed or its rows changed, stop and " +
                "tell them to clear the selection and ask again.";
        }
    }
}
