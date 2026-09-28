using System;
using System.Collections.Generic;
using System.Threading;
using WpsAiBridge.Bridge;

namespace WpsAiBridge.Ai
{
    internal static class AiAgent
    {
        public static LlmResult Run(IAppHost host, string prompt, Action<string> progress)
        {
            return Run(host, prompt, progress, CancellationToken.None);
        }

        public static LlmResult Run(IAppHost host, string prompt, Action<string> progress, CancellationToken cancel)
        {
            string kind = host.AppKind;
            string appName;
            if (kind == "et")
            {
                appName = "Microsoft Excel / WPS Spreadsheets";
            }
            else if (kind == "wpp")
            {
                appName = "Microsoft PowerPoint / WPS Presentation";
            }
            else
            {
                appName = "Microsoft Word / WPS Writer";
            }

            string system =
                "You are an AI assistant embedded inside " + appName + ", working on the document that is currently open. " +
                "Use the office_action tool for EVERY document change so the user sees it happen live on screen, and also for reading the document when needed (for example read the open file before answering questions about it). " +
                "Prefer a few well-chosen actions over many tiny ones. Write all generated content (letters, reports, slide contents, tables) in the user's language. " +
                "After finishing, reply with a very short summary (1-2 sentences). Never invent tool results.";

            var tools = new List<LlmToolDef> { OfficeActionTool.Definition(kind) };
            Func<string, string, string> executor = delegate(string name, string args)
            {
                return OfficeActionTool.Execute(host, name, args);
            };
            return LlmClient.RunAgent(system, prompt, tools, executor, 8, progress, cancel);
        }
    }
}
