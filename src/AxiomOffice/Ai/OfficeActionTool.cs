using System;
using System.Collections.Generic;
using System.Web.Script.Serialization;
using AxiomOffice.Bridge;

namespace AxiomOffice.Ai
{
    internal sealed class LlmToolDef
    {
        public string Name;
        public string Description;
        public string ParametersJson;
    }

    internal static class OfficeActionTool
    {
        public const string ToolName = "office_action";

        public static LlmToolDef Definition(string kind)
        {
            // Lệnh đánh dấu ForAgent() trong CommandDispatcher.*.cs của loại app đang mở (mặc định Writer).
            string actions = CommandCatalog.AgentActions(kind == "et" || kind == "wpp" ? kind : "wps");
            return new LlmToolDef
            {
                Name = ToolName,
                Description =
                    "Read and modify the LIVE document that is currently open in the office application. " +
                    "Call this for every document change the user asks for so it happens immediately on screen. " +
                    "Array params such as values must be real JSON arrays of rows, not objects. " +
                    "Change existing content in place (e.g. writer.formatTable to restyle a table); " +
                    "never use undo to start over - only undo when the user asks. " +
                    "Available actions (with params): " + actions,
                ParametersJson =
                    "{\"type\":\"object\",\"properties\":{" +
                    "\"action\":{\"type\":\"string\",\"description\":\"Action name, e.g. writer.insertStyledText or wpp.addSlide\"}," +
                    "\"params\":{\"type\":\"object\",\"description\":\"Action parameters as an object (may be omitted)\"}}," +
                    "\"required\":[\"action\"]}"
            };
        }

        // Tên lệnh viết sai nhẹ ("et_writeRange", khác hoa/thường) thì quy về tên đúng của lệnh agent được dùng,
        // giống OfficeActionTool.CanonicalAction của Core; không khớp lệnh agent nào thì giữ nguyên để từ chối.
        internal static string CanonicalAction(string action)
        {
            if (string.IsNullOrEmpty(action))
            {
                return action;
            }
            string trimmed = action.Trim();
            int underscore = trimmed.IndexOf('_');
            string dotted = trimmed.IndexOf('.') < 0 && underscore > 0
                ? trimmed.Substring(0, underscore) + "." + trimmed.Substring(underscore + 1)
                : trimmed;
            foreach (CommandInfo command in CommandDispatcher.Commands)
            {
                if (command.Agent && (string.Equals(command.Name, trimmed, StringComparison.OrdinalIgnoreCase)
                    || string.Equals(command.Name, dotted, StringComparison.OrdinalIgnoreCase)))
                {
                    return command.Name;
                }
            }
            return action;
        }

        public static string Execute(IAppHost host, string name, string argumentsJson)
        {
            var serializer = new JavaScriptSerializer();
            if (!string.Equals(name, ToolName, StringComparison.OrdinalIgnoreCase))
            {
                return serializer.Serialize(new Dictionary<string, object>
                {
                    { "ok", false },
                    { "error", "unknown tool: " + name }
                });
            }
            try
            {
                var arguments = serializer.Deserialize<Dictionary<string, object>>(argumentsJson ?? "{}");
                object actionValue;
                if (arguments == null || !arguments.TryGetValue("action", out actionValue) || actionValue == null)
                {
                    return serializer.Serialize(new Dictionary<string, object>
                    {
                        { "ok", false },
                        { "error", "missing 'action' argument" }
                    });
                }
                string action = CanonicalAction(Convert.ToString(actionValue));
                // Model chỉ được gọi lệnh có trong mô tả tool (ForAgent): chặn lệnh ngoài danh sách như
                // writer.closeAll (đóng mọi tài liệu, không lưu) hay ai.ask lồng nhau. HTTP/MCP vẫn gọi được mọi lệnh.
                CommandInfo command = CommandDispatcher.FindCommand(action);
                if (command == null || !command.Agent)
                {
                    Logger.Info("office_action refused: " + action);
                    return serializer.Serialize(new Dictionary<string, object>
                    {
                        { "ok", false },
                        { "error", "'" + action + "' is not an available action; use one of the actions listed in the office_action tool description" }
                    });
                }
                Dictionary<string, object> parameters = null;
                object paramsValue;
                if (arguments.TryGetValue("params", out paramsValue))
                {
                    parameters = paramsValue as Dictionary<string, object>;
                }
                object result = CommandDispatcher.Execute(host, action, parameters);
                return serializer.Serialize(result);
            }
            catch (Exception ex)
            {
                return serializer.Serialize(new Dictionary<string, object>
                {
                    { "ok", false },
                    { "error", ex.Message }
                });
            }
        }
    }
}
