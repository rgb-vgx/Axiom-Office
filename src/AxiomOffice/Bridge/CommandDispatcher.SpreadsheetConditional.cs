using System;
using System.Collections;
using System.Collections.Generic;
using System.Linq;

namespace AxiomOffice.Bridge
{
    // Định dạng có điều kiện dạng trực quan (docs/plans/21-v0 P3): thang màu, thanh dữ liệu, bộ biểu tượng, top/bottom N,
    // trên/dưới trung bình, trùng lặp. Trước đây chỉ có rule so sánh giá trị / công thức nên "tô màu theo mức" hay "đánh
    // dấu 3 giá trị cao nhất" model phải tự viết công thức RANK - model yếu hay sai.
    internal static partial class CommandDispatcher
    {
        // XlFormatConditionType của các kiểu trực quan -> tên trả về ở et.listConditionalFormats.
        private static readonly Dictionary<int, string> VisualTypeNames = new Dictionary<int, string>
        {
            { 3, "colorScale" }, { 4, "dataBar" }, { 5, "top10" }, { 6, "iconSet" }, { 8, "uniqueValues" }, { 12, "aboveAverage" }
        };

        // XlIconSet.
        private static readonly Dictionary<string, int> IconSets = new Dictionary<string, int>(StringComparer.OrdinalIgnoreCase)
        {
            { "3arrows", 1 }, { "3flags", 3 }, { "3traffic", 4 }, { "3signs", 6 }, { "3symbols", 7 }, { "4arrows", 9 },
            { "4traffic", 12 }, { "5arrows", 13 }, { "3stars", 18 }
        };

        private static readonly string[] VisualRuleTypes =
            { "colorscale", "databar", "iconset", "top", "bottom", "aboveaverage", "belowaverage", "duplicate", "unique" };

        // Mặc định thang màu: thấp = đỏ, giữa = vàng, cao = xanh (cùng bảng màu của Excel).
        private static readonly string[] DefaultScale = { "#F8696B", "#FFEB84", "#63BE7B" };

        private static void CheckVisualRule(Dictionary<string, object> rule, string type, int index)
        {
            if (!VisualRuleTypes.Contains(type))
            {
                throw new InvalidOperationException(string.Format("'rules[{0}].type' must be one of colorScale/dataBar/iconSet/top/bottom/"
                    + "aboveAverage/belowAverage/duplicate/unique, got '{1}'", index, ParamString(rule, "type", type)));
            }
            if (type == "colorscale")
            {
                List<string> colors = RuleColors(rule);
                if (colors.Count != 0 && colors.Count != 2 && colors.Count != 3)
                {
                    throw new InvalidOperationException(string.Format("'rules[{0}].colors' needs 2 or 3 colours (low → high), got {1}", index, colors.Count));
                }
                foreach (string color in colors)
                {
                    if (!ParseBgrColor(color).HasValue)
                    {
                        throw new InvalidOperationException(string.Format("'rules[{0}].colors': '{1}' is not a #RRGGBB colour", index, color));
                    }
                }
            }
            if (type == "iconset" && !IconSets.ContainsKey(ParamString(rule, "icons", "3arrows")))
            {
                throw new InvalidOperationException(string.Format("'rules[{0}].icons' must be one of {1}", index, string.Join("/", IconSets.Keys)));
            }
            if ((type == "top" || type == "bottom") && ParamInt(rule, "rank", 10) < 1)
            {
                throw new InvalidOperationException(string.Format("'rules[{0}].rank' must be 1 or more", index));
            }
            if (VisualRuleTakesStyle(type) && !HasParam(rule, "fillColor") && !HasParam(rule, "fontColor") && !HasParam(rule, "bold"))
            {
                throw new InvalidOperationException(string.Format("'rules[{0}]' ({1}) needs a style: fillColor, fontColor and/or bold", index, type));
            }
        }

        private static bool VisualRuleTakesStyle(string type)
        {
            return type != "colorscale" && type != "databar" && type != "iconset";
        }

        // `colors` (thấp → cao) hoặc minColor / midColor / maxColor (bộ đo 27/10/2026 ca x12: model yếu gửi kiểu này).
        private static List<string> RuleColors(Dictionary<string, object> rule)
        {
            var colors = new List<string>();
            object raw;
            if (rule.TryGetValue("colors", out raw) && raw is IEnumerable && !(raw is string))
            {
                foreach (object item in (IEnumerable)raw)
                {
                    colors.Add(Convert.ToString(UnwrapScalar(item)));
                }
            }
            if (colors.Count == 0 && HasParam(rule, "minColor") && HasParam(rule, "maxColor"))
            {
                colors.Add(ParamString(rule, "minColor", null));
                if (HasParam(rule, "midColor"))
                {
                    colors.Add(ParamString(rule, "midColor", null));
                }
                colors.Add(ParamString(rule, "maxColor", null));
            }
            return colors;
        }

        // Xoá rule cũ CÓ VÙNG ÁP DỤNG TRÙNG KHÍT `range` (gọi lại trên cùng vùng = thay). Range.FormatConditions.Delete()
        // xoá cả rule của vùng khác chỉ chồng một phần (bộ đo 27/10/2026 ca x12: tô hàng top 3 trên A2:C13 xoá luôn thang
        // màu của B2:B13 và thanh dữ liệu của C2:C13).
        private static void DeleteSameRangeConditions(dynamic range)
        {
            string target = Convert.ToString(range.Address(false, false));
            dynamic conditions = range.FormatConditions;
            for (int i = Convert.ToInt32(conditions.Count); i >= 1; i--)
            {
                dynamic condition = conditions[i];
                string applies;
                try
                {
                    applies = Convert.ToString(condition.AppliesTo.Address(false, false));
                }
                catch (Exception)
                {
                    continue;
                }
                if (string.Equals(applies, target, StringComparison.OrdinalIgnoreCase))
                {
                    condition.Delete();
                }
            }
        }

        private static void ApplyConditionStyle(dynamic condition, Dictionary<string, object> rule)
        {
            if (ParamBool(rule, "bold", false))
            {
                condition.Font.Bold = true;
            }
            if (ParamBool(rule, "italic", false))
            {
                condition.Font.Italic = true;
            }
            int? fontColor = ParseBgrColor(ParamString(rule, "fontColor", null));
            if (fontColor.HasValue)
            {
                condition.Font.Color = fontColor.Value;
            }
            int? fillColor = ParseBgrColor(ParamString(rule, "fillColor", null));
            if (fillColor.HasValue)
            {
                condition.Interior.Color = fillColor.Value;
            }
        }

        private static dynamic AddVisualCondition(dynamic range, dynamic wb, string type, Dictionary<string, object> rule)
        {
            dynamic conditions = range.FormatConditions;
            switch (type)
            {
                case "colorscale":
                {
                    List<string> colors = RuleColors(rule);
                    if (colors.Count == 0)
                    {
                        colors = DefaultScale.ToList();
                    }
                    dynamic scale = conditions.AddColorScale(colors.Count);
                    for (int i = 0; i < colors.Count; i++)
                    {
                        scale.ColorScaleCriteria[i + 1].FormatColor.Color = ParseBgrColor(colors[i]).Value;
                    }
                    return scale;
                }
                case "databar":
                {
                    dynamic bar = conditions.AddDatabar();
                    int? color = ParseBgrColor(ParamString(rule, "color", "#5B9BD5"));
                    if (color.HasValue)
                    {
                        bar.BarColor.Color = color.Value;
                    }
                    return bar;
                }
                case "iconset":
                {
                    dynamic icons = conditions.AddIconSetCondition();
                    icons.IconSet = wb.IconSets[IconSets[ParamString(rule, "icons", "3arrows")]];
                    return icons;
                }
                case "top":
                case "bottom":
                {
                    dynamic top = conditions.AddTop10();
                    // XlTopBottom: xlTop10Top = 1, xlTop10Bottom = 0.
                    top.TopBottom = type == "top" ? 1 : 0;
                    top.Rank = ParamInt(rule, "rank", 10);
                    top.Percent = ParamBool(rule, "percent", false);
                    return top;
                }
                case "aboveaverage":
                case "belowaverage":
                {
                    dynamic average = conditions.AddAboveAverage();
                    // XlAboveBelow: xlAboveAverage = 0, xlBelowAverage = 1.
                    average.AboveBelow = type == "aboveaverage" ? 0 : 1;
                    return average;
                }
                default:
                {
                    dynamic unique = conditions.AddUniqueValues();
                    // XlDupeUnique: xlUnique = 0, xlDuplicate = 1.
                    unique.DupeUnique = type == "duplicate" ? 1 : 0;
                    return unique;
                }
            }
        }
    }
}
