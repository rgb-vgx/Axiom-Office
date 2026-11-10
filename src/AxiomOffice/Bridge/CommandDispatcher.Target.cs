using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;

namespace AxiomOffice.Bridge
{
    // Lệnh nhắm tài liệu tường minh (docs/plans/19-h0 H1). Trước 25/10/2026 phần lớn lệnh writer.*/et.* và một số
    // wpp.* làm trên tài liệu ĐANG ACTIVE: benchmark slide có lượt bài mới không thành active nên slide bị ghi nối
    // vào bài của lượt trước. Giờ mọi lệnh của ba loại app nhận tham số chọn tài liệu (Core tự gắn tài liệu của
    // lượt); lệnh vốn dùng ActiveDocument/Selection thì bridge kích hoạt đúng tài liệu trước khi chạy.
    internal static partial class CommandDispatcher
    {
        // Lớp lồng: static của nó khởi tạo ở lần dùng đầu tiên, nên dùng được ngay trong bộ khởi tạo của `Commands`
        // (thứ tự khởi tạo static giữa các file partial không được bảo đảm).
        private static class Target
        {
            public static readonly Dictionary<string, string> ParamByKind = new Dictionary<string, string>
            {
                { "wpp", "presentation" },
                { "et", "workbook" },
                { "wps", "document" }
            };

            // Lệnh được thêm tham số ở đây (chưa tự xử lý tham số đó): bridge kích hoạt tài liệu đích trước khi chạy.
            public static readonly HashSet<string> Activates = new HashSet<string>(StringComparer.Ordinal);
        }

        // Lệnh tạo / mở / đóng tài liệu: không có "tài liệu đích".
        private static bool CreatesOrCloses(string name)
        {
            return name.EndsWith(".open", StringComparison.Ordinal)
                || name.EndsWith(".newPresentation", StringComparison.Ordinal)
                || name.EndsWith(".newDocument", StringComparison.Ordinal)
                || name.EndsWith(".newWorkbook", StringComparison.Ordinal)
                || name.EndsWith(".closeAll", StringComparison.Ordinal)
                || name == "wpp.newFromTemplate";
        }

        private static IEnumerable<CommandInfo> WithTargetParams(IEnumerable<CommandInfo> commands)
        {
            foreach (CommandInfo command in commands)
            {
                string name;
                if (command.Kind == null || !Target.ParamByKind.TryGetValue(command.Kind, out name)
                    || CreatesOrCloses(command.Name) || command.Params.Any(p => p.Name == name))
                {
                    yield return command;
                    continue;
                }
                Target.Activates.Add(command.Name);
                yield return command.WithParam(new CommandParam(name, false,
                    "full path of the " + name + " to work on; default: the active one"));
            }
        }

        // Kích hoạt tài liệu đích (nếu lệnh dùng tài liệu active và có truyền tham số). Tài liệu đã đóng thì báo lỗi rõ,
        // KHÔNG lặng lẽ làm trên tài liệu active.
        private static void ActivateTarget(IAppHost host, CommandInfo command, Dictionary<string, object> p)
        {
            if (command == null || !Target.Activates.Contains(command.Name))
            {
                return;
            }
            string param = Target.ParamByKind[command.Kind];
            string wanted = ParamString(p, param, null);
            if (string.IsNullOrWhiteSpace(wanted))
            {
                return;
            }
            wanted = wanted.Trim();
            dynamic app = host.Application;
            switch (command.Kind)
            {
                case "wpp":
                    dynamic pres = FindPresentation(app, wanted);
                    if (pres == null)
                    {
                        throw new ArgumentException(Closed(param, wanted));
                    }
                    if (!SameDocument(app.ActivePresentation, pres) && Convert.ToInt32(pres.Windows.Count) > 0)
                    {
                        try { pres.Windows[1].Activate(); } catch (Exception) { }
                    }
                    // ActivePresentation có thể không đổi (PowerPoint 16); ghi nhớ bài đích để lệnh dùng.
                    TrackPresentation(pres);
                    break;
                case "et":
                    dynamic book = FindIn(app.Workbooks, wanted);
                    if (book == null)
                    {
                        throw new ArgumentException(Closed(param, wanted));
                    }
                    if (!SameDocument(app.ActiveWorkbook, book))
                    {
                        book.Activate();
                    }
                    break;
                case "wps":
                    dynamic doc = FindIn(app.Documents, wanted);
                    if (doc == null)
                    {
                        throw new ArgumentException(Closed(param, wanted));
                    }
                    if (!SameDocument(app.ActiveDocument, doc))
                    {
                        doc.Activate();
                    }
                    break;
            }
        }

        private static string Closed(string param, string wanted)
        {
            return "DOCUMENT_CLOSED: the " + param + " '" + wanted + "' is not open (it was closed or renamed); open it again or ask the user";
        }

        // Tìm theo đường dẫn đầy đủ, rồi theo tên (tài liệu chưa lưu chỉ có tên: "Document1", "Book1").
        private static dynamic FindIn(dynamic collection, string wanted)
        {
            string full = wanted;
            try
            {
                full = Path.GetFullPath(wanted);
            }
            catch (Exception)
            {
            }
            dynamic byName = null;
            int count = Convert.ToInt32(collection.Count);
            for (int i = 1; i <= count; i++)
            {
                dynamic item = collection[i];
                string fullName = Convert.ToString(item.FullName);
                if (string.Equals(fullName, wanted, StringComparison.OrdinalIgnoreCase)
                    || string.Equals(fullName, full, StringComparison.OrdinalIgnoreCase))
                {
                    return item;
                }
                if (byName == null && string.Equals(Convert.ToString(item.Name), Path.GetFileName(wanted), StringComparison.OrdinalIgnoreCase))
                {
                    byName = item;
                }
            }
            return byName;
        }

        private static bool SameDocument(dynamic active, dynamic target)
        {
            try
            {
                return string.Equals(Convert.ToString(active.FullName), Convert.ToString(target.FullName), StringComparison.OrdinalIgnoreCase);
            }
            catch (Exception)
            {
                return false; // không có tài liệu active
            }
        }
    }
}
