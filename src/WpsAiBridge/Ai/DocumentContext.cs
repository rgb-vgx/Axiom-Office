using System;
using System.Text;
using WpsAiBridge.Bridge;

namespace WpsAiBridge.Ai
{
    internal static class DocumentContext
    {
        public const int MaxChars = 50000;

        public static string GetSelection(Connect connect)
        {
            try
            {
                dynamic app = connect.Application;
                string text = Convert.ToString(app.Selection.Text);
                return text ?? "";
            }
            catch
            {
                return "";
            }
        }

        public static string GetWholeDocument(Connect connect, string kind)
        {
            try
            {
                switch (kind)
                {
                    case "et":
                        return Truncate(GetSheetText(connect));
                    case "wpp":
                        return Truncate(GetSlidesText(connect));
                    default:
                        return Truncate(GetWriterText(connect));
                }
            }
            catch (Exception ex)
            {
                Logger.Error("DocumentContext failed", ex);
                return "";
            }
        }

        private static string GetWriterText(Connect connect)
        {
            dynamic app = connect.Application;
            dynamic doc = app.ActiveDocument;
            return Convert.ToString(doc.Content.Text) ?? "";
        }

        private static string GetSheetText(Connect connect)
        {
            dynamic app = connect.Application;
            dynamic sheet = app.ActiveSheet;
            dynamic used = sheet.UsedRange;
            object raw = used.Value2;
            object[,] matrix = raw as object[,];
            if (matrix == null)
            {
                string scalar = Convert.ToString(raw);
                return scalar ?? "";
            }
            int r0 = matrix.GetLowerBound(0);
            int r1 = matrix.GetUpperBound(0);
            int c0 = matrix.GetLowerBound(1);
            int c1 = matrix.GetUpperBound(1);
            int rows = r1 - r0 + 1;
            int cols = c1 - c0 + 1;
            int maxRows = Math.Min(rows, 1000);
            int maxCols = Math.Min(cols, 80);
            var sb = new StringBuilder();
            for (int r = r0; r < r0 + maxRows; r++)
            {
                for (int c = c0; c < c0 + maxCols; c++)
                {
                    if (c > c0)
                    {
                        sb.Append('\t');
                    }
                    object v = matrix[r, c];
                    sb.Append(v == null ? "" : Convert.ToString(v));
                }
                sb.Append('\n');
                if (sb.Length > MaxChars)
                {
                    break;
                }
            }
            if (rows > maxRows)
            {
                sb.Append("[... ").Append(rows - maxRows).Append(" more rows omitted]\n");
            }
            return sb.ToString();
        }

        private static string GetSlidesText(Connect connect)
        {
            dynamic app = connect.Application;
            dynamic pres = app.ActivePresentation;
            int count = Convert.ToInt32(pres.Slides.Count);
            var sb = new StringBuilder();
            for (int i = 1; i <= count; i++)
            {
                sb.Append("[Slide ").Append(i).Append("]\n");
                dynamic slide = pres.Slides[i];
                int shapeCount = Convert.ToInt32(slide.Shapes.Count);
                for (int s = 1; s <= shapeCount; s++)
                {
                    try
                    {
                        dynamic shape = slide.Shapes[s];
                        if (Convert.ToInt32(shape.HasTextFrame) == -1 && Convert.ToInt32(shape.TextFrame.HasText) == -1)
                        {
                            sb.Append(Convert.ToString(shape.TextFrame.TextRange.Text)).Append('\n');
                        }
                    }
                    catch
                    {
                    }
                }
                if (sb.Length > MaxChars)
                {
                    break;
                }
            }
            return sb.ToString();
        }

        private static string Truncate(string text)
        {
            if (string.IsNullOrEmpty(text))
            {
                return "";
            }
            if (text.Length <= MaxChars)
            {
                return text;
            }
            return text.Substring(0, MaxChars) + "\n[... truncated at " + MaxChars + " chars]";
        }
    }
}
