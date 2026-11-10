using System;
using System.Collections.Generic;
using System.Linq;

namespace AxiomOffice.Bridge
{
    // Hình học của đường nối (Shape.Connector) cho wpp.checkLayout và wpp.addConnector. Trước 25/10/2026 checkLayout
    // bỏ qua mọi đường nối (khung bao của đường gấp khúc trùm lên các hộp nó nối, không phải chồng) - nên đường nối
    // cắt ngang qua hộp khác không ai thấy (lượt thật: sơ đồ kiến trúc 10 slide, issueCount 0 mà dây xuyên qua hai hộp).
    internal static partial class CommandDispatcher
    {
        // Dây đi sát mép hộp (chạm viền) không tính là cắt qua.
        private const double ConnectorInset = 2.0;
        // Hai đoạn song song cách nhau dưới mức này và cùng phủ một quãng dài hơn MinOverlapRun là chồng nhau.
        private const double ConnectorParallel = 2.0;
        private const double MinOverlapRun = 8.0;

        // Đường đi thật của đường nối (toạ độ slide). PowerPoint lưu đường nối như hình chữ nhật Left/Top/Width/Height
        // (khung TRƯỚC khi xoay) + lật + xoay; đường thẳng đi từ góc (0,0) tới (w,h) của khung, đường gấp khúc
        // (bentConnector3) đi (0,0) -> (a*w,0) -> (a*w,h) -> (w,h) với a = Adjustments(1) - a ngoài [0,1] là đường vòng
        // chữ U ra ngoài khung. Lật trong khung trước, rồi xoay quanh tâm khung.
        internal static List<double[]> ConnectorPath(double left, double top, double width, double height, bool flipH, bool flipV,
            double rotation, bool elbow, double adjust)
        {
            var local = new List<double[]> { new[] { 0.0, 0.0 } };
            if (elbow)
            {
                local.Add(new[] { adjust * width, 0.0 });
                local.Add(new[] { adjust * width, height });
            }
            local.Add(new[] { width, height });
            double centerX = width / 2, centerY = height / 2;
            double radians = rotation * Math.PI / 180;
            double cos = Math.Cos(radians), sin = Math.Sin(radians);
            var path = new List<double[]>();
            foreach (double[] point in local)
            {
                double x = flipH ? width - point[0] : point[0];
                double y = flipV ? height - point[1] : point[1];
                double dx = x - centerX, dy = y - centerY;
                path.Add(new[] { left + centerX + dx * cos - dy * sin, top + centerY + dx * sin + dy * cos });
            }
            return path;
        }

        // Dây cắt qua hộp không phải đầu/cuối của nó, và hai dây chồng lên nhau trên một quãng.
        internal static void ConnectorIssues(IList<ShapeBox> boxes, double slideWidth, double slideHeight, List<Dictionary<string, object>> issues)
        {
            List<ShapeBox> connectors = boxes.Where(b => b.Connector && b.Path != null && b.Path.Count >= 2).ToList();
            if (connectors.Count == 0)
            {
                return;
            }
            double slideArea = slideWidth * slideHeight;
            List<ShapeBox> solids = boxes.Where(b => !b.Connector && b.Width * b.Height < slideArea * 0.9).ToList();
            foreach (ShapeBox connector in connectors)
            {
                ShapeBox begin = solids.FirstOrDefault(b => b.Name == connector.BeginShape);
                ShapeBox end = solids.FirstOrDefault(b => b.Name == connector.EndShape);
                foreach (ShapeBox box in solids)
                {
                    if (box.Name == connector.BeginShape || box.Name == connector.EndShape)
                    {
                        continue;
                    }
                    // Khung bao quanh một đầu (ranh giới tin cậy, nhóm) - dây đi trong khung là đúng ý.
                    if ((begin != null && Contains(box, begin)) || (end != null && Contains(box, end)))
                    {
                        continue;
                    }
                    if (PathHitsBox(connector.Path, box))
                    {
                        issues.Add(Issue("connector-crossing",
                            "connector '" + connector.Name + "' runs across '" + box.Name + "', which is not one of its ends: route it "
                            + "around (other sides, an elbow that goes outside the row), move the shapes, or drop the arrow",
                            new Dictionary<string, object> { { "shapes", new[] { connector.Name, box.Name } } }));
                    }
                }
            }
            for (int a = 0; a < connectors.Count; a++)
            {
                for (int c = a + 1; c < connectors.Count; c++)
                {
                    ShapeBox first = connectors[a], second = connectors[c];
                    // Cùng xuất phát (toả nhánh) hoặc cùng đích (gom nhánh) dùng chung một đoạn thân là kiểu sơ đồ tổ chức.
                    bool sameSource = first.BeginShape != null && first.BeginShape == second.BeginShape;
                    bool sameTarget = first.EndShape != null && first.EndShape == second.EndShape;
                    if (sameSource || sameTarget)
                    {
                        continue;
                    }
                    if (PathsOverlap(first.Path, second.Path))
                    {
                        issues.Add(Issue("connector-overlap",
                            "connectors '" + first.Name + "' and '" + second.Name + "' run on top of each other: the arrows cannot be "
                            + "told apart - route one on another side or move the shapes",
                            new Dictionary<string, object> { { "shapes", new[] { first.Name, second.Name } } }));
                    }
                }
            }
        }

        private static bool PathHitsBox(List<double[]> path, ShapeBox box)
        {
            double left = box.Left + ConnectorInset, top = box.Top + ConnectorInset;
            double right = box.Left + box.Width - ConnectorInset, bottom = box.Top + box.Height - ConnectorInset;
            if (right <= left || bottom <= top)
            {
                return false;
            }
            for (int i = 1; i < path.Count; i++)
            {
                if (SegmentHitsRect(path[i - 1][0], path[i - 1][1], path[i][0], path[i][1], left, top, right, bottom))
                {
                    return true;
                }
            }
            return false;
        }

        // Liang-Barsky: đoạn thẳng có đi qua phần trong của hình chữ nhật không.
        internal static bool SegmentHitsRect(double x1, double y1, double x2, double y2, double left, double top, double right, double bottom)
        {
            double dx = x2 - x1, dy = y2 - y1;
            double enter = 0, leave = 1;
            double[] p = { -dx, dx, -dy, dy };
            double[] q = { x1 - left, right - x1, y1 - top, bottom - y1 };
            for (int i = 0; i < 4; i++)
            {
                if (Math.Abs(p[i]) < 1e-9)
                {
                    if (q[i] <= 0)
                    {
                        return false;
                    }
                    continue;
                }
                double t = q[i] / p[i];
                if (p[i] < 0)
                {
                    enter = Math.Max(enter, t);
                }
                else
                {
                    leave = Math.Min(leave, t);
                }
                if (enter >= leave)
                {
                    return false;
                }
            }
            return leave - enter > 1e-6;
        }

        // Hai đường có đoạn ngang (hoặc dọc) trùng nhau trên một quãng dài.
        internal static bool PathsOverlap(List<double[]> first, List<double[]> second)
        {
            for (int i = 1; i < first.Count; i++)
            {
                for (int j = 1; j < second.Count; j++)
                {
                    if (SegmentsOverlap(first[i - 1], first[i], second[j - 1], second[j]))
                    {
                        return true;
                    }
                }
            }
            return false;
        }

        private static bool SegmentsOverlap(double[] a1, double[] a2, double[] b1, double[] b2)
        {
            bool aHorizontal = Math.Abs(a1[1] - a2[1]) < 0.5, bHorizontal = Math.Abs(b1[1] - b2[1]) < 0.5;
            bool aVertical = Math.Abs(a1[0] - a2[0]) < 0.5, bVertical = Math.Abs(b1[0] - b2[0]) < 0.5;
            if (aHorizontal && bHorizontal && Math.Abs(a1[1] - b1[1]) < ConnectorParallel)
            {
                return Run(a1[0], a2[0], b1[0], b2[0]) > MinOverlapRun;
            }
            if (aVertical && bVertical && Math.Abs(a1[0] - b1[0]) < ConnectorParallel)
            {
                return Run(a1[1], a2[1], b1[1], b2[1]) > MinOverlapRun;
            }
            return false;
        }

        private static double Run(double a1, double a2, double b1, double b2)
        {
            return Math.Min(Math.Max(a1, a2), Math.Max(b1, b2)) - Math.Max(Math.Min(a1, a2), Math.Min(b1, b2));
        }

        // Đọc đường đi và hai đầu của một đường nối vào ShapeBox (gọi TRƯỚC RotateBox: cần khung chưa xoay).
        private static void ReadConnector(dynamic shape, ShapeBox box)
        {
            bool elbow = false;
            try
            {
                elbow = Convert.ToInt32(shape.ConnectorFormat.Type) != 1; // msoConnectorStraight = 1
            }
            catch (Exception)
            {
            }
            double adjust = 0.5;
            try
            {
                if (Convert.ToInt32(shape.Adjustments.Count) > 0)
                {
                    adjust = Convert.ToDouble(shape.Adjustments[1]);
                }
            }
            catch (Exception)
            {
            }
            box.Path = ConnectorPath(box.Left, box.Top, box.Width, box.Height, ComFlag(() => shape.HorizontalFlip),
                ComFlag(() => shape.VerticalFlip), ComDouble(() => shape.Rotation), elbow, adjust);
            try
            {
                if (ComFlag(() => shape.ConnectorFormat.BeginConnected))
                {
                    box.BeginShape = Convert.ToString(shape.ConnectorFormat.BeginConnectedShape.Name);
                }
                if (ComFlag(() => shape.ConnectorFormat.EndConnected))
                {
                    box.EndShape = Convert.ToString(shape.ConnectorFormat.EndConnectedShape.Name);
                }
            }
            catch (Exception)
            {
            }
        }

        // Đường vòng chữ U (cùng một cạnh ở hai đầu): đặt đoạn giữa cách đầu xa hơn `offset` pt ra phía cạnh đó - PowerPoint
        // tự chọn độ sâu, hai đường vòng dưới cùng một hàng sẽ trùng nhau. Đoạn giữa là điểm (a*w, 0) của khung, tuyến tính
        // theo a: đo hai giá trị a rồi giải.
        private static void SetLoopOffset(dynamic connector, string side, double offset)
        {
            double left = Convert.ToDouble(connector.Left), top = Convert.ToDouble(connector.Top);
            double width = Convert.ToDouble(connector.Width), height = Convert.ToDouble(connector.Height);
            bool flipH = ComFlag(() => connector.HorizontalFlip), flipV = ComFlag(() => connector.VerticalFlip);
            double rotation = ComDouble(() => connector.Rotation);
            List<double[]> at0 = ConnectorPath(left, top, width, height, flipH, flipV, rotation, true, 0);
            List<double[]> at1 = ConnectorPath(left, top, width, height, flipH, flipV, rotation, true, 1);
            bool vertical = side == "top" || side == "bottom";
            int axis = vertical ? 1 : 0;
            double slope = at1[1][axis] - at0[1][axis];
            if (Math.Abs(slope) < 1e-6)
            {
                return;
            }
            double start = at0[0][axis], finish = at0[3][axis];
            double target = side == "bottom" || side == "right" ? Math.Max(start, finish) + offset : Math.Min(start, finish) - offset;
            connector.Adjustments[1] = (float)((target - at0[1][axis]) / slope);
        }
    }
}
