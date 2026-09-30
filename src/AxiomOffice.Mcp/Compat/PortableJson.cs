using System;
using System.Collections.Generic;
using System.Text.Encodings.Web;
using System.Text.Json;

namespace AxiomOffice.Host.Mcp.Portable
{
    // Ban .NET 10 cua System.Web.Script.Serialization.JavaScriptSerializer (chi co tren .NET Framework):
    // dung dung API ma McpServer.cs/CommandDispatcher dung, nhung chay tren System.Text.Json.
    // DeserializeObject tra ve Dictionary<string, object> / object[] / string / long / double / bool / null -
    // dung tap kieu ma JavaScriptSerializer tra (ToolArgs dua vao do).
    internal sealed class JavaScriptSerializer
    {
        private readonly JsonSerializerOptions _options;

        public JavaScriptSerializer()
        {
            RecursionLimit = 100;
            MaxJsonLength = int.MaxValue;
            _options = new JsonSerializerOptions
            {
                Encoder = JavaScriptEncoder.UnsafeRelaxedJsonEscaping, // giu tieng Viet doc duoc trong log/JSON-RPC
                WriteIndented = false,
            };
        }

        public JavaScriptSerializer(object _) : this()
        {
        }

        // Chi de tuong thich: System.Text.Json khong gioi han do sau theo kieu nay.
        public int MaxJsonLength { get; set; }

        public int RecursionLimit { get; set; }

        public string Serialize(object value)
        {
            return JsonSerializer.Serialize(value, _options);
        }

        public object DeserializeObject(string input)
        {
            using (JsonDocument document = JsonDocument.Parse(input))
            {
                return ConvertElement(document.RootElement);
            }
        }

        private static object ConvertElement(JsonElement element)
        {
            switch (element.ValueKind)
            {
                case JsonValueKind.Object:
                    var map = new Dictionary<string, object>(StringComparer.Ordinal);
                    foreach (JsonProperty property in element.EnumerateObject())
                    {
                        map[property.Name] = ConvertElement(property.Value);
                    }
                    return map;
                case JsonValueKind.Array:
                    var items = new List<object>();
                    foreach (JsonElement item in element.EnumerateArray())
                    {
                        items.Add(ConvertElement(item));
                    }
                    return items.ToArray();
                case JsonValueKind.String:
                    return element.GetString();
                case JsonValueKind.Number:
                    long integer;
                    if (element.TryGetInt64(out integer))
                    {
                        return integer;
                    }
                    double real;
                    return element.TryGetDouble(out real) ? (object)real : element.GetRawText();
                case JsonValueKind.True:
                    return true;
                case JsonValueKind.False:
                    return false;
                default:
                    return null;
            }
        }
    }
}
