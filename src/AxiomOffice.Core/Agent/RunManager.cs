using System.Collections.Concurrent;
using System.Text.Json.Nodes;
using AxiomOffice.Core.Models;

namespace AxiomOffice.Core.Agent;

public static class RunStatus
{
    public const string Running = "running";
    public const string Completed = "completed";
    public const string Failed = "failed";
    public const string Cancelled = "cancelled";
    public const string TimedOut = "timedout";
    public const string Stopped = "stopped";
}

// Trang thai mot luot chay (New_arch.md muc 7.3, 8.1): duoc giu lai 30 phut sau khi xong de client
// doc ket qua va de SSE ket noi lai.
public sealed class RunState(string id, int port)
{
    public string Id { get; } = id;

    public int Port { get; } = port;

    public string? ConversationId { get; set; }

    public string Status { get; set; } = RunStatus.Running;

    public DateTime StartedUtc { get; } = DateTime.UtcNow;

    public DateTime? FinishedUtc { get; set; }

    public string? Reply { get; set; }

    public string? Error { get; set; }

    public string ErrorKind { get; set; } = "";

    public int Rounds { get; set; }

    public int InputTokens { get; set; }

    public int OutputTokens { get; set; }

    public double Seconds { get; set; }

    public int ToolCalls { get; set; }

    public List<string> Transcript { get; } = [];

    public RunEventStream Events { get; } = new();

    public CancellationTokenSource Cancel { get; } = new();

    public Task? Work { get; set; }

    public JsonNode ToJson()
    {
        var transcript = new JsonArray();
        foreach (string line in Transcript)
        {
            transcript.Add(line);
        }

        return new JsonObject
        {
            ["runId"] = Id,
            ["conversationId"] = ConversationId,
            ["port"] = Port,
            ["status"] = Status,
            ["started"] = StartedUtc.ToString("yyyy-MM-ddTHH:mm:ss.fffZ"),
            ["finished"] = FinishedUtc?.ToString("yyyy-MM-ddTHH:mm:ss.fffZ"),
            ["reply"] = Reply,
            ["error"] = Error,
            ["errorKind"] = ErrorKind.Length == 0 ? null : ErrorKind,
            ["rounds"] = Rounds,
            ["inputTokens"] = InputTokens,
            ["outputTokens"] = OutputTokens,
            ["seconds"] = Math.Round(Seconds, 2),
            ["toolCalls"] = ToolCalls,
            ["lastEvent"] = Events.Since(0).Count == 0 ? 0 : Events.Since(0)[^1].Seq,
            ["transcript"] = transcript,
        };
    }
}

// Quan ly cac luot chay: mot luot dang chay cho moi office port, huy duoc, giu ket qua 30 phut.
public sealed class RunManager
{
    public const int RetentionMinutes = 30;

    private readonly ConcurrentDictionary<string, RunState> _runs = new(StringComparer.Ordinal);
    private readonly Lock _gate = new();

    public (RunState? Run, string? Error) TryStart(int port)
    {
        lock (_gate)
        {
            Cleanup();
            RunState? busy = _runs.Values.FirstOrDefault(r => r.Port == port && r.Status == RunStatus.Running);
            if (busy != null)
            {
                return (null, $"busy: a run is already in progress on port {port} (run {busy.Id})");
            }

            var run = new RunState("r_" + Guid.NewGuid().ToString("N"), port);
            _runs[run.Id] = run;
            return (run, null);
        }
    }

    public RunState? Get(string id)
    {
        return _runs.TryGetValue(id, out RunState? run) ? run : null;
    }

    public bool Cancel(string id)
    {
        RunState? run = Get(id);
        if (run == null || run.Status != RunStatus.Running)
        {
            return false;
        }

        run.Cancel.Cancel();
        return true;
    }

    public IReadOnlyList<RunState> List()
    {
        Cleanup();
        return _runs.Values.OrderByDescending(r => r.StartedUtc).ToList();
    }

    private void Cleanup()
    {
        DateTime cutoff = DateTime.UtcNow.AddMinutes(-RetentionMinutes);
        foreach ((string id, RunState run) in _runs)
        {
            if (run.Status != RunStatus.Running && run.FinishedUtc < cutoff)
            {
                _runs.TryRemove(id, out _);
            }
        }
    }
}
