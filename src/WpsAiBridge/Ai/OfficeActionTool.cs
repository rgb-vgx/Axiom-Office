using System;
using System.Collections.Generic;
using System.Web.Script.Serialization;
using WpsAiBridge.Bridge;

namespace WpsAiBridge.Ai
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
            string actions;
            switch (kind)
            {
                case "et":
                    actions = "et.listSheets {}; et.readRange {range,sheet}; et.writeRange {range,values,sheet}; et.newWorkbook {}; et.formatRange {range,bold,italic,fontSize,fontColor,fillColor,numFmt,horizontal,wrap,sheet}; et.activateSheet {sheet}; et.undo {count}; et.save {}; et.saveAs {path}; et.exportPdf {path}";
                    break;
                case "wpp":
                    actions = "wpp.listSlides {}; wpp.addSlide {layout}; wpp.addText {text,slide,left,top,width,height,fontSize,bold,color,align}; wpp.addImage {path,slide,left,top,width,height}; wpp.addTable {rows,cols,values,slide,left,top,width,height}; wpp.setNotes {text,slide}; wpp.deleteSlide {slide}; wpp.save {}; wpp.saveAs {path}; wpp.exportPdf {path}";
                    break;
                default:
                    actions = "writer.getText {maxChars}; writer.selection {}; writer.insertStyledText {text,bold,italic,underline,size,color,font}; writer.formatSelection {bold,italic,underline,size,color,font,alignment}; writer.setParagraphAlignment {alignment}; writer.heading {text,level}; writer.insertTable {rows,cols,values,style}; writer.insertPageBreak {}; writer.insertImage {path,width,height}; writer.insertHyperlink {url,text}; writer.replaceAll {find,replace}; writer.undo {count}; writer.save {}; writer.saveAs {path}; writer.exportPdf {path}";
                    break;
            }
            return new LlmToolDef
            {
                Name = ToolName,
                Description =
                    "Read and modify the LIVE document that is currently open in the office application. " +
                    "Call this for every document change the user asks for so it happens immediately on screen. " +
                    "Available actions (with params): " + actions,
                ParametersJson =
                    "{\"type\":\"object\",\"properties\":{" +
                    "\"action\":{\"type\":\"string\",\"description\":\"Action name, e.g. writer.insertStyledText or wpp.addSlide\"}," +
                    "\"params\":{\"type\":\"object\",\"description\":\"Action parameters as an object (may be omitted)\"}}," +
                    "\"required\":[\"action\"]}"
            };
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
                string action = Convert.ToString(actionValue);
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
