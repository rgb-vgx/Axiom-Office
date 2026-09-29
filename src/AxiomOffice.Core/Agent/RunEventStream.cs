using System.Runtime.CompilerServices;
using System.Text.Json.Nodes;

namespace AxiomOffice.Core.Agent;

// Mot su kien cua luot chay (SSE - New_arch.md muc 7.4). Seq tang dan de client ket noi lai voi ?after=.
public sealed record RunEvent(long Seq, string Type, JsonNode? Data, string At);

// Buffer su kien + phat cho nguoi dang nghe. Nhieu subscriber doc cung mot buffer.
public sealed class RunEventStream(int bufferSize = 1000)
{
    private readonly object _gate = new();
    private readonly List<RunEvent> _buffer = [];
    private TaskCompletionSource<bool> _signal = NewSignal();
    private long _seq;
    private bool _completed;

    public bool Completed
    {
        get
        {
            lock (_gate)
            {
                return _completed;
            }
        }
    }

    public RunEvent Publish(string type, JsonNode? data = null)
    {
        RunEvent item;
        TaskCompletionSource<bool> signal;
        lock (_gate)
        {
            item = new RunEvent(++_seq, type, data, DateTime.UtcNow.ToString("yyyy-MM-ddTHH:mm:ss.fffZ"));
            _buffer.Add(item);
            if (_buffer.Count > bufferSize)
            {
                _buffer.RemoveRange(0, _buffer.Count - bufferSize);
            }

            signal = _signal;
            _signal = NewSignal();
        }

        signal.TrySetResult(true);
        return item;
    }

    public void Complete()
    {
        TaskCompletionSource<bool> signal;
        lock (_gate)
        {
            _completed = true;
            signal = _signal;
            _signal = NewSignal();
        }

        signal.TrySetResult(true);
    }

    public IReadOnlyList<RunEvent> Since(long afterSeq)
    {
        lock (_gate)
        {
            return _buffer.Where(e => e.Seq > afterSeq).ToList();
        }
    }

    // Phat lai su kien cu (neu client ket noi lai) roi tiep tuc nghe su kien moi.
    public async IAsyncEnumerable<RunEvent> SubscribeAsync(long afterSeq, [EnumeratorCancellation] CancellationToken cancel)
    {
        while (true)
        {
            List<RunEvent> pending;
            Task wait;
            bool completed;
            lock (_gate)
            {
                pending = _buffer.Where(e => e.Seq > afterSeq).ToList();
                wait = _signal.Task;
                completed = _completed;
            }

            foreach (RunEvent item in pending)
            {
                afterSeq = item.Seq;
                yield return item;
            }

            if (completed)
            {
                yield break;
            }

            try
            {
                await wait.WaitAsync(cancel).ConfigureAwait(false);
            }
            catch (OperationCanceledException)
            {
                yield break;
            }
        }
    }

    // Cho co su kien moi (hoac het timeout de caller gui ping). false = luot chay da ket thuc.
    public async Task<bool> WaitForChangeAsync(TimeSpan timeout, CancellationToken cancel)
    {
        Task signal;
        lock (_gate)
        {
            if (_completed)
            {
                return false;
            }

            signal = _signal.Task;
        }

        try
        {
            await signal.WaitAsync(timeout, cancel).ConfigureAwait(false);
            return true;
        }
        catch (TimeoutException)
        {
            return true;
        }
        catch (OperationCanceledException)
        {
            return false;
        }
    }

    private static TaskCompletionSource<bool> NewSignal()
    {
        return new TaskCompletionSource<bool>(TaskCreationOptions.RunContinuationsAsynchronously);
    }
}
