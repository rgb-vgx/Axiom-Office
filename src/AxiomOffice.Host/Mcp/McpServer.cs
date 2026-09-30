using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Globalization;
using System.IO;
using System.Linq;
using System.Text;
#if PORTABLE
using AxiomOffice.Host.Mcp.Portable;   // ban .NET 10 (axiom-office-mcp): JavaScriptSerializer tren System.Text.Json
#else
using System.Web.Script.Serialization;
#endif
using AxiomOffice.Bridge;

namespace AxiomOffice.Host.Mcp
{
    // MCP server qua stdio: JSON-RPC 2.0, mỗi message một dòng. stdout chỉ dành cho giao thức;
    // log ghi vào bridge.log, mọi Console.Write lạc chỗ bị chuyển sang stderr.
    internal sealed class McpServer
    {
        private static readonly string[] SupportedVersions = { "2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05" };

        private readonly string _name;
        private readonly List<McpTool> _tools;
        private readonly Dictionary<string, McpTool> _byName;
        private readonly JavaScriptSerializer _json = Json.Create();

        public McpServer(string name, IEnumerable<McpTool> tools)
        {
            _name = name;
            _tools = new List<McpTool>();
            _byName = new Dictionary<string, McpTool>(StringComparer.Ordinal);
            foreach (McpTool tool in tools)
            {
                if (!_byName.ContainsKey(tool.Name))
                {
                    _tools.Add(tool);
                    _byName[tool.Name] = tool;
                }
            }
        }

        public int ToolCount
        {
            get { return _tools.Count; }
        }

        public IEnumerable<McpTool> Tools
        {
            get { return _tools; }
        }

        public int Run(TextReader input, TextWriter output)
        {
            Logger.Info("MCP server '" + _name + "' started with " + _tools.Count + " tools");
            string line;
            while ((line = input.ReadLine()) != null)
            {
                line = line.Trim();
                if (line.Length == 0)
                {
                    continue;
                }
                string reply = HandleLine(line);
                if (reply != null)
                {
                    output.Write(reply);
                    output.Write('\n');
                    output.Flush();
                }
            }
            Logger.Info("MCP server '" + _name + "' stdin closed, exiting");
            return 0;
        }

        // Xử lý một dòng JSON-RPC; trả null khi không cần trả lời (notification).
        public string HandleLine(string line)
        {
            object parsed;
            try
            {
                parsed = _json.DeserializeObject(line);
            }
            catch (Exception ex)
            {
                return _json.Serialize(ErrorResponse(null, -32700, "Parse error: " + ex.Message));
            }

            var batch = parsed as object[];
            if (batch != null)
            {
                var replies = new List<object>();
                foreach (object item in batch)
                {
                    object reply = Handle(item as Dictionary<string, object>);
                    if (reply != null)
                    {
                        replies.Add(reply);
                    }
                }
                return replies.Count == 0 ? null : _json.Serialize(replies);
            }
            object single = Handle(parsed as Dictionary<string, object>);
            return single == null ? null : _json.Serialize(single);
        }

        private object Handle(Dictionary<string, object> message)
        {
            if (message == null)
            {
                return ErrorResponse(null, -32600, "Invalid Request");
            }
            object id;
            bool isRequest = message.TryGetValue("id", out id) && id != null;
            string method = message.ContainsKey("method") ? Convert.ToString(message["method"]) : null;
            if (method == null)
            {
                // Response từ client (ta không gửi request nào) hoặc message lỗi: bỏ qua.
                return null;
            }
            var parameters = message.ContainsKey("params") ? message["params"] as Dictionary<string, object> : null;
            try
            {
                switch (method)
                {
                    case "initialize":
                        return Result(id, Initialize(parameters));
                    case "ping":
                        return isRequest ? Result(id, new Dictionary<string, object>()) : null;
                    case "tools/list":
                        return Result(id, ListTools());
                    case "tools/call":
                        return Result(id, CallTool(parameters));
                    default:
                        if (method.StartsWith("notifications/", StringComparison.Ordinal) || !isRequest)
                        {
                            return null;
                        }
                        return ErrorResponse(id, -32601, "Method not found: " + method);
                }
            }
            catch (McpProtocolException ex)
            {
                return ErrorResponse(id, ex.Code, ex.Message);
            }
            catch (Exception ex)
            {
                Logger.Error("MCP " + method + " failed", ex);
                return ErrorResponse(id, -32603, ex.Message);
            }
        }

        private Dictionary<string, object> Initialize(Dictionary<string, object> parameters)
        {
            string requested = parameters != null && parameters.ContainsKey("protocolVersion")
                ? Convert.ToString(parameters["protocolVersion"])
                : null;
            string version = Array.IndexOf(SupportedVersions, requested) >= 0 ? requested : SupportedVersions[0];
            string client = "";
            if (parameters != null && parameters.ContainsKey("clientInfo"))
            {
                var info = parameters["clientInfo"] as Dictionary<string, object>;
                if (info != null && info.ContainsKey("name"))
                {
                    client = Convert.ToString(info["name"]) + " " + (info.ContainsKey("version") ? Convert.ToString(info["version"]) : "");
                }
            }
            Logger.Info("MCP initialize: client=" + client.Trim() + " protocol=" + requested + " -> " + version);
            return new Dictionary<string, object>
            {
                { "protocolVersion", version },
                { "capabilities", new Dictionary<string, object> { { "tools", new Dictionary<string, object> { { "listChanged", false } } } } },
                { "serverInfo", new Dictionary<string, object> { { "name", _name }, { "version", CommandDispatcher.BridgeVersion } } },
                { "instructions", "Office/WPS tools. File tools (doc_*, ppt_* file, excel_*) work on files on disk without the app. " +
                    "Live tools talk to the Axiom Office add-in inside a running Word/Excel/PowerPoint or WPS; call office_sessions() to find live bridges and their ports." }
            };
        }

        private Dictionary<string, object> ListTools()
        {
            var list = new List<object>();
            foreach (McpTool tool in _tools)
            {
                list.Add(new Dictionary<string, object>
                {
                    { "name", tool.Name },
                    { "description", tool.Description },
                    { "inputSchema", tool.InputSchema }
                });
            }
            return new Dictionary<string, object> { { "tools", list } };
        }

        private Dictionary<string, object> CallTool(Dictionary<string, object> parameters)
        {
            string name = parameters != null && parameters.ContainsKey("name") ? Convert.ToString(parameters["name"]) : null;
            McpTool tool;
            if (name == null || !_byName.TryGetValue(name, out tool))
            {
                throw new McpProtocolException(-32602, "Unknown tool: " + name);
            }
            var arguments = parameters.ContainsKey("arguments") ? parameters["arguments"] as Dictionary<string, object> : null;
            var watch = Stopwatch.StartNew();
            string text;
            bool isError = false;
            try
            {
                object result = tool.Handler(new ToolArgs(arguments));
                text = result as string ?? _json.Serialize(result);
            }
            catch (Exception ex)
            {
                Exception inner = ex is System.Reflection.TargetInvocationException && ex.InnerException != null ? ex.InnerException : ex;
                isError = true;
                text = _json.Serialize(new Dictionary<string, object> { { "ok", false }, { "error", inner.Message } });
                Logger.Info("MCP tool " + name + " error: " + inner.GetType().Name + ": " + inner.Message);
            }
            Logger.Info("MCP tool " + name + " " + (isError ? "failed" : "ok") + " in " + watch.ElapsedMilliseconds + "ms");
            return new Dictionary<string, object>
            {
                { "content", new object[] { new Dictionary<string, object> { { "type", "text" }, { "text", text } } } },
                { "isError", isError }
            };
        }

        private static Dictionary<string, object> Result(object id, object result)
        {
            return new Dictionary<string, object> { { "jsonrpc", "2.0" }, { "id", id }, { "result", result } };
        }

        private static Dictionary<string, object> ErrorResponse(object id, int code, string message)
        {
            return new Dictionary<string, object>
            {
                { "jsonrpc", "2.0" },
                { "id", id },
                { "error", new Dictionary<string, object> { { "code", code }, { "message", message } } }
            };
        }
    }

    internal sealed class McpProtocolException : Exception
    {
        public readonly int Code;

        public McpProtocolException(int code, string message) : base(message)
        {
            Code = code;
        }
    }

    internal sealed class McpTool
    {
        public string Name;
        public string Description;
        public Dictionary<string, object> InputSchema;
        public Func<ToolArgs, object> Handler;

        public McpTool(string name, string description, Param[] parameters, Func<ToolArgs, object> handler)
        {
            Name = name;
            Description = description;
            InputSchema = Param.Schema(parameters);
            Handler = handler;
        }
    }

    // Mô tả tham số tool -> JSON Schema cho tools/list.
    internal sealed class Param
    {
        public string Name;
        public string Type;
        public string Description;
        public bool Required;
        public object Default;
        public object Items;

        public static Param Str(string name, string description = null, bool required = false, string def = null)
        {
            return new Param { Name = name, Type = "string", Description = description, Required = required, Default = def };
        }

        public static Param Int(string name, string description = null, bool required = false, object def = null)
        {
            return new Param { Name = name, Type = "integer", Description = description, Required = required, Default = def };
        }

        public static Param Bool(string name, string description = null, bool required = false, object def = null)
        {
            return new Param { Name = name, Type = "boolean", Description = description, Required = required, Default = def };
        }

        public static Param Obj(string name, string description = null, bool required = false)
        {
            return new Param { Name = name, Type = "object", Description = description, Required = required };
        }

        public static Param Arr(string name, string description = null, bool required = false, object items = null)
        {
            return new Param { Name = name, Type = "array", Description = description, Required = required, Items = items };
        }

        public static Param Matrix(string name, string description = null, bool required = false)
        {
            return Arr(name, description, required, new Dictionary<string, object> { { "type", "array" } });
        }

        public static Param Any(string name, string description = null, bool required = false, params string[] types)
        {
            return new Param { Name = name, Type = types.Length == 0 ? null : string.Join("|", types), Description = description, Required = required };
        }

        public static Dictionary<string, object> Schema(Param[] parameters)
        {
            var properties = new Dictionary<string, object>();
            var required = new List<object>();
            foreach (Param p in parameters ?? new Param[0])
            {
                var property = new Dictionary<string, object>();
                if (p.Type != null)
                {
                    string[] types = p.Type.Split('|');
                    property["type"] = types.Length == 1 ? (object)types[0] : types;
                }
                if (p.Items != null)
                {
                    property["items"] = p.Items;
                }
                if (p.Description != null)
                {
                    property["description"] = p.Description;
                }
                if (p.Default != null)
                {
                    property["default"] = p.Default;
                }
                properties[p.Name] = property;
                if (p.Required)
                {
                    required.Add(p.Name);
                }
            }
            var schema = new Dictionary<string, object> { { "type", "object" }, { "properties", properties } };
            if (required.Count > 0)
            {
                schema["required"] = required;
            }
            return schema;
        }
    }

    // Đọc arguments của tools/call. JSON null coi như không truyền.
    internal sealed class ToolArgs
    {
        private readonly Dictionary<string, object> _values;

        public ToolArgs(Dictionary<string, object> values)
        {
            _values = values ?? new Dictionary<string, object>();
        }

        public bool Has(string name)
        {
            object value;
            return _values.TryGetValue(name, out value) && value != null;
        }

        public object Raw(string name)
        {
            object value;
            return _values.TryGetValue(name, out value) ? value : null;
        }

        public string Req(string name)
        {
            string value = Str(name);
            if (string.IsNullOrEmpty(value))
            {
                throw new ArgumentException("missing required argument '" + name + "'");
            }
            return value;
        }

        public string Str(string name, string def = null)
        {
            object value = Raw(name);
            if (value == null)
            {
                return def;
            }
            return Convert.ToString(value, CultureInfo.InvariantCulture);
        }

        public int Int(string name, int def)
        {
            int? value = IntOpt(name);
            return value.HasValue ? value.Value : def;
        }

        public int? IntOpt(string name)
        {
            object value = Raw(name);
            if (value == null)
            {
                return null;
            }
            try
            {
                return Convert.ToInt32(value, CultureInfo.InvariantCulture);
            }
            catch (Exception)
            {
                throw new ArgumentException("argument '" + name + "' must be an integer");
            }
        }

        public bool Bool(string name, bool def)
        {
            bool? value = BoolOpt(name);
            return value.HasValue ? value.Value : def;
        }

        public bool? BoolOpt(string name)
        {
            object value = Raw(name);
            if (value == null)
            {
                return null;
            }
            if (value is bool)
            {
                return (bool)value;
            }
            string text = Convert.ToString(value, CultureInfo.InvariantCulture).Trim().ToLowerInvariant();
            if (text == "true" || text == "1")
            {
                return true;
            }
            if (text == "false" || text == "0")
            {
                return false;
            }
            throw new ArgumentException("argument '" + name + "' must be a boolean");
        }

        public Dictionary<string, object> Obj(string name)
        {
            return Raw(name) as Dictionary<string, object>;
        }

        public object[] Arr(string name)
        {
            object value = Raw(name);
            if (value == null)
            {
                return null;
            }
            var array = value as object[];
            if (array != null)
            {
                return array;
            }
            var list = value as System.Collections.ArrayList;
            if (list != null)
            {
                return list.ToArray();
            }
            throw new ArgumentException("argument '" + name + "' must be an array");
        }

        public object[][] Matrix(string name)
        {
            object[] rows = Arr(name);
            if (rows == null)
            {
                return null;
            }
            return rows.Select(row => row as object[] ?? new object[] { row }).ToArray();
        }
    }

    internal static class Json
    {
        public static JavaScriptSerializer Create()
        {
            return new JavaScriptSerializer { MaxJsonLength = int.MaxValue, RecursionLimit = 256 };
        }

        public static string Serialize(object value)
        {
            return Create().Serialize(value);
        }
    }
}
