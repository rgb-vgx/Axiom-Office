using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Web.Script.Serialization;

namespace AxiomOffice.Bridge
{
    internal delegate object CommandHandler(IAppHost host, Dictionary<string, object> p);

    // Một tham số của lệnh bridge. Hint: gợi ý ngắn (tiếng Anh) kèm tên tham số trong mô tả tool cho model.
    internal sealed class CommandParam
    {
        public readonly string Name;
        public readonly bool Required;
        public readonly string Hint;

        public CommandParam(string name, bool required, string hint)
        {
            Name = name;
            Required = required;
            Hint = hint;
        }
    }

    // Khai báo một lệnh POST /cmd. Danh sách lệnh (CommandDispatcher.Commands, khai báo cạnh handler
    // trong từng file CommandDispatcher.*.cs) là nguồn duy nhất cho: dispatcher, tool office_action của
    // Ask AI pane, mô tả tool MCP *_command và bảng lệnh trong README.
    internal sealed class CommandInfo
    {
        public readonly string Name;
        // null = mọi app; "wps" = Word/Writer, "et" = Excel/Spreadsheets, "wpp" = PowerPoint/Presentation.
        public readonly string Kind;
        public readonly CommandHandler Handler;
        // Mô tả tiếng Việt cho bảng lệnh trong README.
        public readonly string Summary;
        public readonly CommandParam[] Params;

        public CommandInfo(string name, string kind, CommandHandler handler, string summary, CommandParam[] parameters)
        {
            Name = name;
            Kind = kind;
            Handler = handler;
            Summary = summary;
            Params = parameters ?? new CommandParam[0];
            Gated = true;
        }

        // Model trong Ask AI pane thấy lệnh này trong tool office_action.
        public bool Agent { get; private set; }

        // Chạy trong ComGate. Lệnh tự lo việc tuần tự hoá COM (ai.ask, ui.askpane) thì tắt.
        public bool Gated { get; private set; }

        public CommandInfo ForAgent()
        {
            Agent = true;
            return this;
        }

        public CommandInfo Ungated()
        {
            Gated = false;
            return this;
        }

        // "et.readRange {range,sheet}"; tham số có hint: "values (2D array of rows)".
        public string Signature
        {
            get
            {
                return Name + " {" + string.Join(",", Params.Select(p => p.Hint == null ? p.Name : p.Name + " (" + p.Hint + ")").ToArray()) + "}";
            }
        }
    }

    internal static class CommandCatalog
    {
        public static IList<CommandInfo> All
        {
            get { return CommandDispatcher.Commands; }
        }

        // Lệnh của một loại app (kind = null: lệnh chung mọi app).
        public static IEnumerable<CommandInfo> ForKind(string kind)
        {
            return All.Where(c => c.Kind == kind);
        }

        // "a {x}; b {}" - danh sách lệnh kèm tham số cho mô tả tool mà model đọc.
        public static string Signatures(IEnumerable<CommandInfo> commands)
        {
            return string.Join("; ", commands.Select(c => c.Signature).ToArray());
        }

        // Lệnh cho tool office_action của Ask AI pane theo loại app đang mở.
        public static string AgentActions(string kind)
        {
            return Signatures(ForKind(kind).Where(c => c.Agent));
        }

        // Bảng lệnh cho README (AxiomOffice.Host.exe commands --markdown).
        public static string ToMarkdown()
        {
            var text = new StringBuilder();
            text.Append("| Action | Params | Mô tả |\n");
            text.Append("|---|---|---|\n");
            foreach (CommandInfo command in All)
            {
                string parameters = command.Params.Length == 0
                    ? "—"
                    : string.Join(", ", command.Params.Select(p => "`" + p.Name + (p.Required ? "" : "?") + "`").ToArray());
                text.Append("| `").Append(command.Name).Append("` | ").Append(parameters).Append(" | ").Append(command.Summary).Append(" |\n");
            }
            return text.ToString();
        }

        public static string ToJson()
        {
            var list = All.Select(c => new Dictionary<string, object>
            {
                { "name", c.Name },
                { "kind", c.Kind },
                { "agent", c.Agent },
                { "summary", c.Summary },
                { "params", c.Params.Select(p => new Dictionary<string, object>
                    {
                        { "name", p.Name },
                        { "required", p.Required },
                        { "hint", p.Hint }
                    }).ToList() }
            }).ToList();
            return new JavaScriptSerializer().Serialize(list);
        }
    }
}
