using System;
using System.Collections.Generic;

namespace AxiomOffice.Ai
{
    // Phần Core API cho nhóm "Quản lý" trên ribbon (docs/ribbon-ui.md): kỹ năng, hội thoại, nhật ký hoạt động,
    // trạng thái, tắt Core. Tách khỏi CoreClient.cs cho dễ đọc; cùng một lớp partial.
    internal sealed partial class CoreClient
    {
        // GET /v1/skills?app= : trả cả payload (skills + errors + sources) để thư mục kỹ năng nạp lỗi không bị
        // im lặng bỏ qua (giống skill_listing bên LibreOffice).
        public Dictionary<string, object> SkillListing(string appKind, out string error)
        {
            string path = "/v1/skills" + (string.IsNullOrEmpty(appKind) ? "" : "?app=" + Uri.EscapeDataString(appKind));
            return MemoryCall("GET", path, null, out error);
        }

        // POST /v1/skills/reload: đọc lại thư mục kỹ năng, không khởi động lại Core.
        public bool ReloadSkills(out string error)
        {
            return MemoryCall("POST", "/v1/skills/reload", new Dictionary<string, object>(), out error) != null;
        }

        // GET /v1/mcp: server đã cấu hình (không in env/headers) + tool đang nạp + lỗi khởi động.
        public List<Dictionary<string, object>> McpServers(out string error)
        {
            Dictionary<string, object> result = MemoryCall("GET", "/v1/mcp", null, out error);
            if (result == null)
            {
                return null;
            }
            return AdminText.List(result, "servers");
        }

        // GET /v1/audit: các lệnh AI đã gọi, mới nhất trước.
        public List<Dictionary<string, object>> Audit(int limit, out string error)
        {
            Dictionary<string, object> result = MemoryCall("GET", "/v1/audit?limit=" + limit, null, out error);
            return result == null ? null : Items(result, "calls");
        }

        public List<Dictionary<string, object>> ListConversations(string documentKey, int limit, out string error)
        {
            string path = "/v1/conversations?limit=" + limit;
            if (!string.IsNullOrEmpty(documentKey))
            {
                path += "&documentKey=" + Uri.EscapeDataString(documentKey);
            }
            Dictionary<string, object> result = MemoryCall("GET", path, null, out error);
            return result == null ? null : Items(result, "conversations");
        }

        // GET /v1/conversations/{id}: thông tin hội thoại + "messages".
        public Dictionary<string, object> Conversation(string id, out string error)
        {
            return MemoryCall("GET", "/v1/conversations/" + Uri.EscapeDataString(id), null, out error);
        }

        public bool DeleteConversation(string id, out string error)
        {
            return MemoryCall("DELETE", "/v1/conversations/" + Uri.EscapeDataString(id), null, out error) != null;
        }

        // Như PurgeMemories nhưng chọn phạm vi: "all" | "user" | "document". Xoá cứng, không khôi phục được.
        public bool PurgeMemories(string scope, out string error)
        {
            return MemoryCall("DELETE", "/v1/memory?scope=" + Uri.EscapeDataString(scope) + "&confirm=true", null, out error) != null;
        }

        // GET /v1/memory lọc theo phạm vi: scope rỗng = mọi phạm vi; scopeKey = khoá tài liệu khi scope = document.
        public List<Dictionary<string, object>> ListMemories(string query, string scope, string scopeKey, bool includeDeleted, out string error)
        {
            var parameters = new List<string>();
            if (!string.IsNullOrWhiteSpace(query))
            {
                parameters.Add("q=" + Uri.EscapeDataString(query.Trim()));
            }
            if (!string.IsNullOrEmpty(scope))
            {
                parameters.Add("scope=" + Uri.EscapeDataString(scope));
            }
            if (!string.IsNullOrEmpty(scopeKey))
            {
                parameters.Add("scopeKey=" + Uri.EscapeDataString(scopeKey));
            }
            if (includeDeleted)
            {
                parameters.Add("includeDeleted=true");
            }
            Dictionary<string, object> result = MemoryCall("GET", "/v1/memory" + (parameters.Count > 0 ? "?" + string.Join("&", parameters.ToArray()) : ""), null, out error);
            return result == null ? null : Items(result, "memories");
        }

        // /health của Core đang chạy, KHÔNG khởi động Core (nút Trạng thái chỉ quan sát). Chưa chạy -> null + lý do.
        public Dictionary<string, object> RunningHealth(out string error)
        {
            int port;
            if (!TryReadCoreJson(out port))
            {
                error = "Agent Core chưa chạy (chưa có core.json hoặc tiến trình đã thoát).";
                return null;
            }
            string text = GetText("http://127.0.0.1:" + port + "/health", HealthTimeoutMs, out error);
            if (text == null)
            {
                error = "core.json có nhưng Core không trả lời /health (cổng " + port + "): " + error;
                return null;
            }
            var reply = Deserialize(text) as Dictionary<string, object>;
            var result = reply != null && reply.ContainsKey("result") ? reply["result"] as Dictionary<string, object> : null;
            if (result == null)
            {
                error = "Core trả lời /health không hợp lệ.";
                return null;
            }
            error = null;
            return result;
        }

        // POST /v1/admin/shutdown tới Core đang chạy (không khởi động Core chỉ để tắt). Lượt hỏi sau tự khởi động lại.
        public bool ShutdownCore(out string error)
        {
            int port;
            if (!TryReadCoreJson(out port))
            {
                error = "Agent Core không chạy.";
                return false;
            }
            string text = SendJson("POST", "http://127.0.0.1:" + port + "/v1/admin/shutdown", "{}", 5000, out error);
            lock (_sync)
            {
                // Đừng dùng lại base URL đã nhớ: Core sắp dừng.
                _baseUrl = null;
            }
            return text != null;
        }
    }
}
