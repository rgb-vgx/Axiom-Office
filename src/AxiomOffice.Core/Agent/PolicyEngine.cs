using System.Text.Json.Nodes;
using System.Text.RegularExpressions;
using AxiomOffice.Core.Office;

namespace AxiomOffice.Core.Agent;

// Ket qua policy cho mot lenh: can hoi nguoi dung truoc khi chay hay khong (New_arch.md muc 8.6).
public sealed record PolicyDecision(bool NeedsConfirmation, string Reason)
{
    public static readonly PolicyDecision Allow = new(false, "");

    public static PolicyDecision Ask(string reason) => new(true, reason);
}

// Quy tac xac nhan cho office_action (allowlist da loc truoc do):
// - luu/xuat file ma cau yeu cau khong co y luu/xuat ("khong tu luu" - kiem tra mem sau system prompt);
// - *.saveAs ghi de file da ton tai;
// - writer.replaceAll khi tai lieu > 20.000 ky tu;
// - wpp.deleteSlide.
public static partial class PolicyEngine
{
    public const int LargeDocumentChars = 20_000;
    public static readonly TimeSpan DefaultConfirmTimeout = TimeSpan.FromSeconds(120);

    public static bool IsSaveOrExport(string action)
    {
        return action.EndsWith(".save", StringComparison.Ordinal)
            || action.EndsWith(".saveAs", StringComparison.Ordinal)
            || action.EndsWith(".exportPdf", StringComparison.Ordinal);
    }

    // Tu khoa y dinh luu/xuat (tieng Viet co/khong dau + tieng Anh).
    [GeneratedRegex(@"(lưu|luu|save|xuất|xuat|export|pdf|ghi\s*file|ghi\s*ra\s*file)", RegexOptions.IgnoreCase)]
    private static partial Regex SaveIntent();

    public static bool PromptAsksToSave(string prompt) => SaveIntent().IsMatch(prompt ?? "");

    public static async Task<PolicyDecision> EvaluateAsync(
        string action,
        JsonNode? parameters,
        string prompt,
        Func<CancellationToken, Task<int?>> documentLength,
        CancellationToken cancel)
    {
        if (IsSaveOrExport(action) && !PromptAsksToSave(prompt))
        {
            return PolicyDecision.Ask("AI muốn lưu/xuất file dù yêu cầu của bạn không nhắc tới việc lưu");
        }

        if (action.EndsWith(".saveAs", StringComparison.Ordinal) || action.EndsWith(".exportPdf", StringComparison.Ordinal))
        {
            string? path = parameters?["path"] is JsonValue value && value.TryGetValue(out string? text) ? text : null;
            if (!string.IsNullOrWhiteSpace(path) && File.Exists(path.Trim()))
            {
                return PolicyDecision.Ask("Ghi đè file đã có: " + path.Trim());
            }
        }

        if (action == "wpp.deleteSlide")
        {
            return PolicyDecision.Ask("Xoá slide khỏi bài thuyết trình");
        }

        if (action == "writer.replaceAll")
        {
            int? length = await documentLength(cancel).ConfigureAwait(false);
            if (length is > LargeDocumentChars)
            {
                return PolicyDecision.Ask($"Thay thế toàn bộ trong tài liệu dài ({length:N0} ký tự)");
            }
        }

        return PolicyDecision.Allow;
    }

    // Do dai tai lieu Word qua bridge (writer.getText maxChars=1 tra totalChars).
    public static async Task<int?> WordLengthAsync(BridgeClient bridge, int port, CancellationToken cancel)
    {
        try
        {
            BridgeResult result = await bridge.CommandAsync(port, "writer.getText", new JsonObject { ["maxChars"] = 1 }, cancel).ConfigureAwait(false);
            return result.Ok ? result.Result?["totalChars"]?.GetValue<int>() : null;
        }
        catch (Exception)
        {
            return null;
        }
    }
}

// Cho xac nhan cua nguoi dung trong mot run: confirm.required -> POST /v1/runs/{id}/confirm; het gio = tu choi.
public sealed class ConfirmationBroker
{
    private readonly System.Collections.Concurrent.ConcurrentDictionary<string, TaskCompletionSource<bool>> _pending = new(StringComparer.Ordinal);

    public int PendingCount => _pending.Count;

    public async Task<bool> RequestAsync(RunEventStream events, string action, string reason, string? paramsPreview, TimeSpan timeout, CancellationToken cancel)
    {
        string id = "cf_" + Guid.NewGuid().ToString("N")[..12];
        var waiter = new TaskCompletionSource<bool>(TaskCreationOptions.RunContinuationsAsynchronously);
        _pending[id] = waiter;
        events.Publish("confirm.required", new JsonObject
        {
            ["confirmationId"] = id,
            ["action"] = action,
            ["reason"] = reason,
            ["paramsPreview"] = paramsPreview,
            ["timeoutSeconds"] = (int)timeout.TotalSeconds,
        });

        bool approved;
        string how;
        try
        {
            Task finished = await Task.WhenAny(waiter.Task, Task.Delay(timeout, cancel)).ConfigureAwait(false);
            if (finished == waiter.Task)
            {
                approved = waiter.Task.Result;
                how = "user";
            }
            else
            {
                approved = false;
                how = cancel.IsCancellationRequested ? "cancelled" : "timeout";
            }
        }
        catch (OperationCanceledException)
        {
            approved = false;
            how = "cancelled";
        }
        finally
        {
            _pending.TryRemove(id, out _);
        }

        events.Publish("confirm.resolved", new JsonObject { ["confirmationId"] = id, ["approved"] = approved, ["by"] = how });
        return approved;
    }

    public bool Resolve(string confirmationId, bool approved)
    {
        return _pending.TryGetValue(confirmationId, out TaskCompletionSource<bool>? waiter) && waiter.TrySetResult(approved);
    }
}
