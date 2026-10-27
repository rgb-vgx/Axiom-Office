using System;
using System.Collections.Generic;
using System.Linq;

namespace AxiomOffice.Bridge
{
    // Comment và revision của Word (docs/research/tinh-nang-con-thieu-so-voi-claude-m365.md W2, W3): đọc luồng comment,
    // trả lời / đánh dấu xong / thêm comment, sửa đúng vùng được neo; đọc redline (cả header/footer) và chấp nhận / từ chối.
    // Vị trí quy về số đoạn của writer.outline (doc.Range(0, start).Paragraphs.Count - không duyệt cả tài liệu).
    internal static partial class CommandDispatcher
    {
        private const string AiPrefix = "AI: ";

        private static IEnumerable<CommandInfo> WriterReviewCommands()
        {
            return new[]
            {
                Command("writer.listComments", "wps", WriterListComments,
                    "Đọc comment của tài liệu: id, tác giả, nội dung, chữ được neo (anchor), số đoạn theo writer.outline, đã xong chưa, các trả lời",
                    Opt("includeDone", "true = include resolved comments (default false)"), Opt("max", "default 100")).ForAgent(),
                Command("writer.addComment", "wps", WriterAddComment,
                    "Thêm comment vào một đoạn (`paragraph` theo writer.outline) hoặc vào chỗ có chữ `find` (lần xuất hiện đầu)",
                    Req("text"), Opt("paragraph"), Opt("find", "exact text to anchor the comment to")).ForAgent(),
                Command("writer.replyComment", "wps", WriterReplyComment,
                    "Trả lời trong luồng của một comment (id theo writer.listComments); nội dung tự thêm tiền tố \"AI: \"",
                    Req("comment", "comment id from writer.listComments"), Req("text")).ForAgent(),
                Command("writer.resolveComment", "wps", WriterResolveComment,
                    "Đánh dấu một comment là đã xong (`done: false` để mở lại)",
                    Req("comment", "comment id from writer.listComments"), Opt("done", "default true")).ForAgent(),
                Command("writer.editCommentScope", "wps", WriterEditCommentScope,
                    "Thay chữ ĐƯỢC NEO bởi một comment (sửa đúng chỗ người góp ý chỉ ra), giữ định dạng của ký tự đầu",
                    Req("comment", "comment id from writer.listComments"), Req("text", "new text for the anchored passage")).ForAgent(),
                Command("writer.listRevisions", "wps", WriterListRevisions,
                    "Đọc các thay đổi được theo dõi (redline) - cả header/footer/chú thích: id, loại, tác giả, thời điểm, chữ, số đoạn",
                    Opt("author", "only revisions by this author"), Opt("max", "default 200")).ForAgent(),
                Command("writer.reviewRevisions", "wps", WriterReviewRevisions,
                    "Chấp nhận hoặc từ chối thay đổi được theo dõi: theo `ids` (từ writer.listRevisions), theo `author`, hoặc `all: true`",
                    Req("action", "accept | reject"), Opt("ids", "array of revision ids"), Opt("author"), Opt("all", "true = every revision"),
                    Opt("since", "only revisions made at or after this time (ISO 8601), with author")).ForAgent(),
                // Chế độ "AI sửa bằng Track Changes" (W4): Core bật ở đầu lượt, trả lại ở cuối lượt - không cho agent.
                Command("writer.setTracking", "wps", WriterSetTracking,
                    "Bật/tắt theo dõi thay đổi; `author` = tên tác giả của các thay đổi trong lúc bật, `restoreAuthor` trả lại tên cũ",
                    Req("on", "true/false: Document.TrackRevisions"), Opt("author"), Opt("restoreAuthor", "true = put back the user's name")),
            };
        }

        // Tên người dùng Office trước khi Core đổi sang tác giả "Axiom AI" (một process Word, một lượt chạy tại một thời điểm).
        private static string _savedUserName;
        private static string _savedUserInitials;
        private static bool? _savedUseLocalUserInfo;

        private static Dictionary<string, object> WriterSetTracking(IAppHost host, Dictionary<string, object> p)
        {
            bool on = ParamBool(p, "on", true);
            string author = (ParamString(p, "author", null) ?? "").Trim();
            dynamic app = host.Application;
            dynamic doc = app.ActiveDocument;
            bool previous = Convert.ToBoolean(doc.TrackRevisions);
            if (author.Length > 0)
            {
                if (_savedUserName == null)
                {
                    _savedUserName = Convert.ToString(app.UserName) ?? "";
                    _savedUserInitials = Convert.ToString(app.UserInitials) ?? "";
                    try
                    {
                        // Microsoft 365 ghi tác giả theo tài khoản đăng nhập trừ khi bật "dùng tên này bất kể đăng nhập".
                        _savedUseLocalUserInfo = Convert.ToBoolean(app.Options.UseLocalUserInfo);
                        app.Options.UseLocalUserInfo = true;
                    }
                    catch
                    {
                        _savedUseLocalUserInfo = null;
                    }
                }
                app.UserName = author;
                try
                {
                    app.UserInitials = "AI";
                }
                catch
                {
                }
            }
            if (ParamBool(p, "restoreAuthor", false) && _savedUserName != null)
            {
                app.UserName = _savedUserName;
                try
                {
                    app.UserInitials = _savedUserInitials;
                    if (_savedUseLocalUserInfo.HasValue)
                    {
                        app.Options.UseLocalUserInfo = _savedUseLocalUserInfo.Value;
                    }
                }
                catch
                {
                }
                _savedUserName = null;
                _savedUserInitials = null;
                _savedUseLocalUserInfo = null;
            }
            doc.TrackRevisions = on;
            return new Dictionary<string, object>
            {
                { "tracking", Convert.ToBoolean(doc.TrackRevisions) }, { "previous", previous }, { "author", Convert.ToString(app.UserName) ?? "" }
            };
        }

        private static int ParagraphNumberAt(dynamic doc, int position)
        {
            try
            {
                return Math.Max(1, Convert.ToInt32(doc.Range(0, Math.Max(0, position)).Paragraphs.Count));
            }
            catch
            {
                return 0;
            }
        }

        private static string RangeText(dynamic range, int max)
        {
            string text = "";
            try
            {
                text = (Convert.ToString(range.Text) ?? "").Replace("\r", " ").Replace("\a", " ").Trim();
            }
            catch
            {
            }
            return Shorten(text, max);
        }

        private static dynamic CommentById(dynamic doc, Dictionary<string, object> p)
        {
            int id = ParamInt(p, "comment", 0);
            int count = Convert.ToInt32(doc.Comments.Count);
            if (id < 1 || id > count)
            {
                throw new ArgumentException("'comment' must be a comment id from writer.listComments (1.." + count + ")");
            }
            return doc.Comments[id];
        }

        // Word 2013+: Document.Comments có cả trả lời (Ancestor = comment gốc). Bản cũ không có Ancestor -> mọi comment là gốc.
        private static dynamic AncestorOf(dynamic comment)
        {
            try
            {
                return comment.Ancestor;
            }
            catch
            {
                return null;
            }
        }

        private static Dictionary<string, object> WriterListComments(IAppHost host, Dictionary<string, object> p)
        {
            bool includeDone = ParamBool(p, "includeDone", false);
            int max = Math.Max(1, ParamInt(p, "max", 100));
            dynamic doc = ((dynamic)host.Application).ActiveDocument;
            dynamic comments = doc.Comments;
            int count = Convert.ToInt32(comments.Count);
            var byStart = new Dictionary<string, Dictionary<string, object>>();
            var items = new List<object>();
            var replies = new List<KeyValuePair<dynamic, Dictionary<string, object>>>();
            int open = 0;
            for (int i = 1; i <= count; i++)
            {
                dynamic comment = comments[i];
                bool done = false;
                try
                {
                    done = Convert.ToBoolean(comment.Done);
                }
                catch
                {
                }
                var entry = new Dictionary<string, object>
                {
                    { "id", i }, { "author", Convert.ToString(comment.Author) ?? "" }, { "text", RangeText(comment.Range, 500) },
                    { "done", done }
                };
                try
                {
                    entry["date"] = Convert.ToDateTime(comment.Date).ToString("yyyy-MM-dd HH:mm");
                }
                catch
                {
                }
                dynamic ancestor = AncestorOf(comment);
                if (ancestor != null)
                {
                    replies.Add(new KeyValuePair<dynamic, Dictionary<string, object>>(ancestor, entry));
                    continue;
                }
                dynamic scope = comment.Scope;
                entry["anchor"] = RangeText(scope, 200);
                entry["paragraph"] = ParagraphNumberAt(doc, Convert.ToInt32(scope.Start));
                entry["replies"] = new List<object>();
                byStart[Convert.ToString(scope.Start) + ":" + i] = entry;
                if (!done)
                {
                    open++;
                }
                if ((includeDone || !done) && items.Count < max)
                {
                    items.Add(entry);
                }
            }
            // Gắn trả lời vào comment gốc (so theo vùng neo: Ancestor là một Comment, không có chỉ số trực tiếp).
            foreach (var pair in replies)
            {
                int start = -1;
                try
                {
                    start = Convert.ToInt32(pair.Key.Scope.Start);
                }
                catch
                {
                }
                string prefix = Convert.ToString(start) + ":";
                Dictionary<string, object> parent = byStart.Where(kv => kv.Key.StartsWith(prefix, StringComparison.Ordinal))
                    .Select(kv => kv.Value).FirstOrDefault();
                if (parent != null)
                {
                    ((List<object>)parent["replies"]).Add(pair.Value);
                }
            }
            return new Dictionary<string, object> { { "comments", items }, { "open", open }, { "total", count - replies.Count } };
        }

        private static Dictionary<string, object> WriterAddComment(IAppHost host, Dictionary<string, object> p)
        {
            string text = (ParamString(p, "text", null) ?? "").Trim();
            if (text.Length == 0)
            {
                throw new ArgumentException("'text' is required");
            }
            dynamic doc = ((dynamic)host.Application).ActiveDocument;
            dynamic anchor;
            string find = ParamString(p, "find", null);
            int paragraph = ParamInt(p, "paragraph", 0);
            if (!string.IsNullOrEmpty(find))
            {
                anchor = doc.Content;
                dynamic search = anchor.Find;
                search.ClearFormatting();
                if (find.Length > 255 || !Convert.ToBoolean(search.Execute(find, false, false, false, false, false, true, 0)))
                {
                    throw new ArgumentException("'find' text was not found (max 255 characters, exact match)");
                }
            }
            else if (paragraph > 0)
            {
                int count = Convert.ToInt32(doc.Paragraphs.Count);
                if (paragraph > count)
                {
                    throw new ArgumentException("'paragraph' must be between 1 and " + count);
                }
                anchor = doc.Paragraphs.Item(paragraph).Range;
            }
            else
            {
                throw new ArgumentException("give 'paragraph' (from writer.outline) or 'find' (text to anchor the comment to)");
            }
            using (new UndoRecordScope(host.Application, "AI: add comment"))
            {
                doc.Comments.Add(anchor, AiPrefix + text);
            }
            return new Dictionary<string, object>
            {
                { "comment", Convert.ToInt32(doc.Comments.Count) }, { "paragraph", ParagraphNumberAt(doc, Convert.ToInt32(anchor.Start)) }
            };
        }

        private static Dictionary<string, object> WriterReplyComment(IAppHost host, Dictionary<string, object> p)
        {
            string text = (ParamString(p, "text", null) ?? "").Trim();
            if (text.Length == 0)
            {
                throw new ArgumentException("'text' is required");
            }
            dynamic doc = ((dynamic)host.Application).ActiveDocument;
            dynamic comment = CommentById(doc, p);
            if (AncestorOf(comment) != null)
            {
                comment = AncestorOf(comment);
            }
            using (new UndoRecordScope(host.Application, "AI: reply to comment"))
            {
                // Replies.Add(Range, Text) - Word 2013+. Bản cũ không có luồng trả lời: báo rõ thay vì thêm comment mới.
                try
                {
                    comment.Replies.Add(comment.Scope, AiPrefix + text);
                }
                catch (Exception ex)
                {
                    var com = ex as System.Runtime.InteropServices.COMException;
                    if (com != null && IsRetryableComError(com))
                    {
                        throw;
                    }
                    throw new InvalidOperationException("cannot reply in the comment thread (needs Word 2013 or later): " + ex.Message);
                }
            }
            return new Dictionary<string, object> { { "comment", ParamInt(p, "comment", 0) }, { "replied", true } };
        }

        private static Dictionary<string, object> WriterResolveComment(IAppHost host, Dictionary<string, object> p)
        {
            bool done = ParamBool(p, "done", true);
            dynamic doc = ((dynamic)host.Application).ActiveDocument;
            dynamic comment = CommentById(doc, p);
            if (AncestorOf(comment) != null)
            {
                // Microsoft: đặt Done trên MỘT trả lời không hiện gì trong giao diện comment mới - đặt trên comment gốc.
                comment = AncestorOf(comment);
            }
            using (new UndoRecordScope(host.Application, "AI: resolve comment"))
            {
                comment.Done = done;
            }
            return new Dictionary<string, object> { { "comment", ParamInt(p, "comment", 0) }, { "done", done } };
        }

        private static Dictionary<string, object> WriterEditCommentScope(IAppHost host, Dictionary<string, object> p)
        {
            string text = ParamString(p, "text", null);
            if (text == null)
            {
                throw new ArgumentException("'text' is required");
            }
            if (text.Trim().Length == 0)
            {
                throw new ArgumentException("'text' must be non-empty (delete the passage via writer.replaceAll instead)");
            }
            int id = ParamInt(p, "comment", 0);
            dynamic doc = ((dynamic)host.Application).ActiveDocument;
            dynamic comment = CommentById(doc, p);
            if (AncestorOf(comment) != null)
            {
                comment = AncestorOf(comment);
            }
            dynamic scope = comment.Scope;
            string before = RangeText(scope, 200);
            int start = Convert.ToInt32(scope.Start);
            int end = Convert.ToInt32(scope.End);
            if (end <= start)
            {
                throw new InvalidOperationException("this comment is not anchored to any text; edit the paragraph instead");
            }

            // Word XOÁ comment khi toàn bộ chữ được neo bị thay (Microsoft xác nhận: comment chỉ còn sống nếu còn
            // ít nhất 1 ký tự neo gốc; đã đo trên Word 16.0.20527: scope.Text = ..., Delete+Insert, Selection.TypeText
            // đều giết comment; Find/Replace giữ được comment nhưng thu hẹp neo về ký tự dôi ra; lỗi COM "Object has
            // been deleted" nổ ngay khi đọc lại Scope). Vậy nên: chụp metadata luồng (text gốc, các trả lời, cờ done)
            // -> DeleteRecursively xoá cả luồng -> thay chữ trên Range đã nắm [start..end] -> Comments.Add lại trên
            // chuỗi chữ mới -> Replies.Add lại từng trả lời -> khôi phục Done. Author là tài khoản máy hiện tại
            // (add-in vốn thêm comment bằng tài khoản đó); thời điểm đổi thành bây giờ.
            string rootText;
            bool rootDone;
            try
            {
                rootText = RangeText(comment.Range, 500);
                rootDone = Convert.ToBoolean(comment.Done);
            }
            catch
            {
                rootText = before;
                rootDone = false;
            }
            var replies = new List<string>();
            int total = Convert.ToInt32(doc.Comments.Count);
            for (int i = 1; i <= total; i++)
            {
                if (i == id)
                {
                    continue;
                }
                dynamic member = doc.Comments[i];
                dynamic ancestor = AncestorOf(member);
                if (ancestor == null)
                {
                    continue;
                }
                int ancestorStart;
                int ancestorEnd;
                try
                {
                    ancestorStart = Convert.ToInt32(ancestor.Scope.Start);
                    ancestorEnd = Convert.ToInt32(ancestor.Scope.End);
                }
                catch
                {
                    continue;
                }
                if (ancestorStart == start && ancestorEnd == end)
                {
                    replies.Add(RangeText(member.Range, 500));
                }
            }

            using (new UndoRecordScope(host.Application, "AI: edit commented text"))
            {
                try
                {
                    comment.DeleteRecursively();
                }
                catch (System.Runtime.InteropServices.COMException ex)
                {
                    if (IsRetryableComError(ex))
                    {
                        throw;
                    }
                    throw new InvalidOperationException("cannot clear this comment thread (needs Word 2013 or later): " + ex.Message);
                }
                dynamic target = doc.Range(start, end);
                target.Text = text.Replace("\r\n", "\r").Replace('\n', '\r');
                try
                {
                    dynamic freshRoot = doc.Comments.Add(target, rootText);
                    if (rootDone)
                    {
                        try { freshRoot.Done = true; } catch { }
                    }
                    foreach (string reply in replies)
                    {
                        try { freshRoot.Replies.Add(freshRoot.Scope, reply); } catch { }
                    }
                }
                catch (System.Runtime.InteropServices.COMException ex)
                {
                    if (IsRetryableComError(ex))
                    {
                        throw;
                    }
                    throw new InvalidOperationException("cannot re-anchor the comment on the replaced text: " + ex.Message);
                }
            }
            // Chỉ số comment gốc mới: Comments sắp theo vị trí tài liệu, re-add có thể đổi chỉ số.
            int newId = 0;
            int afterCount = Convert.ToInt32(doc.Comments.Count);
            for (int i = 1; i <= afterCount; i++)
            {
                dynamic member = doc.Comments[i];
                if (AncestorOf(member) != null)
                {
                    continue;
                }
                int memberStart;
                try
                {
                    memberStart = Convert.ToInt32(member.Scope.Start);
                }
                catch
                {
                    continue;
                }
                if (memberStart == start)
                {
                    newId = i;
                    break;
                }
            }
            return new Dictionary<string, object>
            {
                { "comment", newId }, { "before", before }, { "after", RangeText(((dynamic)host.Application).ActiveDocument.Comments[newId].Scope, 200) }
            };
        }

        // Mọi story có revision: thân bài, header/footer từng section, chú thích... (Revisions.Count của tài liệu chỉ đếm thân bài).
        private static List<dynamic> RevisionStories(dynamic doc)
        {
            var stories = new List<dynamic>();
            foreach (dynamic first in doc.StoryRanges)
            {
                dynamic story = first;
                while (story != null)
                {
                    stories.Add(story);
                    try
                    {
                        story = story.NextStoryRange;
                    }
                    catch
                    {
                        story = null;
                    }
                }
            }
            return stories;
        }

        private static string StoryName(int storyType)
        {
            switch (storyType)
            {
                case 1: return "main";
                case 2: return "footnotes";
                case 3: return "endnotes";
                case 4: return "comments";
                case 5: return "textFrame";
                case 6: case 7: case 10: return "header";
                case 8: case 9: case 11: return "footer";
                default: return "other";
            }
        }

        private static string RevisionTypeName(int type)
        {
            switch (type)
            {
                case 1: return "insert";
                case 2: return "delete";
                case 3: return "format";
                case 4: return "numbering";
                case 8: return "style";
                case 10: return "paragraphFormat";
                case 11: return "tableFormat";
                case 12: return "sectionFormat";
                case 14: return "movedFrom";
                case 15: return "movedTo";
                case 16: return "cellInsert";
                case 17: return "cellDelete";
                default: return "other";
            }
        }

        // Từng revision kèm id "<story>:<chỉ số>" - ổn định cho tới khi có revision bị chấp nhận / từ chối.
        private static List<KeyValuePair<string, dynamic>> AllRevisions(dynamic doc)
        {
            var list = new List<KeyValuePair<string, dynamic>>();
            int storyIndex = 0;
            foreach (dynamic story in RevisionStories(doc))
            {
                storyIndex++;
                dynamic revisions = story.Revisions;
                int count = Convert.ToInt32(revisions.Count);
                for (int i = 1; i <= count; i++)
                {
                    list.Add(new KeyValuePair<string, dynamic>(storyIndex + ":" + i, revisions[i]));
                }
            }
            return list;
        }

        private static Dictionary<string, object> WriterListRevisions(IAppHost host, Dictionary<string, object> p)
        {
            string author = (ParamString(p, "author", null) ?? "").Trim();
            int max = Math.Max(1, ParamInt(p, "max", 200));
            dynamic doc = ((dynamic)host.Application).ActiveDocument;
            var items = new List<object>();
            var authors = new Dictionary<string, int>(StringComparer.CurrentCultureIgnoreCase);
            int total = 0;
            List<KeyValuePair<string, dynamic>> revisions = AllRevisions(doc);
            foreach (KeyValuePair<string, dynamic> pair in revisions)
            {
                dynamic revision = pair.Value;
                string who = Convert.ToString(revision.Author) ?? "";
                int seen;
                authors.TryGetValue(who, out seen);
                authors[who] = seen + 1;
                if (author.Length > 0 && !string.Equals(who, author, StringComparison.CurrentCultureIgnoreCase))
                {
                    continue;
                }
                total++;
                if (items.Count >= max)
                {
                    continue;
                }
                dynamic range = revision.Range;
                int storyType = 1;
                try
                {
                    storyType = Convert.ToInt32(range.StoryType);
                }
                catch
                {
                }
                var entry = new Dictionary<string, object>
                {
                    { "id", pair.Key }, { "type", RevisionTypeName(Convert.ToInt32(revision.Type)) }, { "author", who },
                    { "text", RangeText(range, 200) }, { "story", StoryName(storyType) }
                };
                try
                {
                    entry["date"] = Convert.ToDateTime(revision.Date).ToString("yyyy-MM-dd HH:mm");
                }
                catch
                {
                }
                if (storyType == 1)
                {
                    entry["paragraph"] = ParagraphNumberAt(doc, Convert.ToInt32(range.Start));
                }
                items.Add(entry);
            }
            return new Dictionary<string, object>
            {
                { "revisions", items }, { "total", total }, { "authors", authors }, { "tracking", Convert.ToBoolean(doc.TrackRevisions) }
            };
        }

        private static Dictionary<string, object> WriterReviewRevisions(IAppHost host, Dictionary<string, object> p)
        {
            string action = (ParamString(p, "action", "") ?? "").Trim().ToLowerInvariant();
            if (action != "accept" && action != "reject")
            {
                throw new ArgumentException("'action' must be 'accept' or 'reject'");
            }
            List<string> ids = ParamStrings(p, "ids");
            string author = (ParamString(p, "author", null) ?? "").Trim();
            bool all = ParamBool(p, "all", false);
            DateTime since = DateTime.MinValue;
            string sinceText = ParamString(p, "since", null);
            if (!string.IsNullOrWhiteSpace(sinceText))
            {
                if (!DateTime.TryParse(sinceText, System.Globalization.CultureInfo.InvariantCulture,
                    System.Globalization.DateTimeStyles.AssumeUniversal | System.Globalization.DateTimeStyles.AdjustToUniversal, out since))
                {
                    throw new ArgumentException("'since' must be an ISO 8601 time, e.g. 2026-10-27T08:30:00Z");
                }
                // Revision.Date là giờ máy, làm tròn tới phút: so theo giờ máy, lùi một phút cho chắc.
                since = since.ToLocalTime().AddMinutes(-1);
            }
            if ((ids == null || ids.Count == 0) && author.Length == 0 && !all)
            {
                throw new ArgumentException("give 'ids' (from writer.listRevisions), 'author', or 'all': true");
            }
            dynamic doc = ((dynamic)host.Application).ActiveDocument;
            var wanted = ids == null ? new HashSet<string>() : new HashSet<string>(ids.Select(id => id.Trim()));
            // Duyệt NGƯỢC: chấp nhận / từ chối làm dịch chỉ số các revision phía sau trong cùng story.
            // (Gán vào biến có kiểu trước: gọi với tham số dynamic thì kết quả là dynamic, không dùng LINQ + lambda được.)
            List<KeyValuePair<string, dynamic>> revisions = AllRevisions(doc);
            var targets = new List<KeyValuePair<string, dynamic>>();
            foreach (KeyValuePair<string, dynamic> pair in revisions)
            {
                string who = Convert.ToString(pair.Value.Author) ?? "";
                bool byAuthor = author.Length > 0 && string.Equals(who, author, StringComparison.CurrentCultureIgnoreCase);
                if (byAuthor && since > DateTime.MinValue)
                {
                    DateTime when = DateTime.MinValue;
                    try
                    {
                        when = Convert.ToDateTime(pair.Value.Date);
                    }
                    catch
                    {
                    }
                    byAuthor = when >= since;
                }
                if (all || wanted.Contains(pair.Key) || byAuthor)
                {
                    targets.Add(pair);
                }
            }
            targets.Reverse();
            var missing = wanted.Where(id => !targets.Any(t => t.Key == id)).ToList();
            if (missing.Count > 0 && !all)
            {
                throw new ArgumentException("unknown revision ids: " + string.Join(", ", missing) + " (read them again with writer.listRevisions)");
            }
            bool tracking = Convert.ToBoolean(doc.TrackRevisions);
            int done = 0;
            using (new UndoRecordScope(host.Application, "AI: " + action + " revisions"))
            {
                // Đang bật theo dõi thì chấp nhận/từ chối không được tạo thêm revision.
                doc.TrackRevisions = false;
                try
                {
                    foreach (var pair in targets)
                    {
                        if (action == "accept")
                        {
                            pair.Value.Accept();
                        }
                        else
                        {
                            pair.Value.Reject();
                        }
                        done++;
                    }
                }
                finally
                {
                    doc.TrackRevisions = tracking;
                }
            }
            return new Dictionary<string, object> { { "action", action }, { "count", done } };
        }
    }
}
