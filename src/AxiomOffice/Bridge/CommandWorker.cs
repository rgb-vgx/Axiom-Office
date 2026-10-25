using System;
using System.Collections.Concurrent;
using System.Threading;

namespace AxiomOffice.Bridge
{
    // Chạy lệnh /cmd trên MỘT thread STA riêng, để thread Pump của HttpBridge không bị kẹt theo host.
    //
    // Trước 25/10/2026 Pump tự chạy lệnh: PowerPoint hiện hộp thoại modal ("An error occurred while PowerPoint was
    // saving the file") thì lời gọi COM đứng đó, Pump đứng theo, và mọi request sau - kể cả /health - treo cho tới khi
    // có người bấm OK (docs/plans/19-h0 H2). Giờ Pump chỉ chờ trong thời hạn rồi trả HOST_BUSY kèm nội dung hộp
    // thoại; lệnh đang chạy dở không huỷ được (COM), nhưng lệnh còn xếp hàng thì bị bỏ, không chạy muộn sau khi
    // client đã được báo lỗi. Lệnh vẫn tuần tự trên một thread STA như khi chạy trên Pump.
    internal sealed class CommandWorker : IDisposable
    {
        public const int TimeoutMs = 120000;

        private sealed class Job
        {
            public Func<object> Work;
            public string Action;
            public object Result;
            public string Failure;
            public int State; // 0 chờ, 1 đang chạy, 2 xong, 3 bị bỏ
            public readonly ManualResetEventSlim Done = new ManualResetEventSlim(false);
        }

        private readonly BlockingCollection<Job> _queue = new BlockingCollection<Job>();
        private readonly Thread _thread;
        private volatile string _running;

        public CommandWorker()
        {
            _thread = new Thread(Loop);
            _thread.IsBackground = true;
            _thread.Name = "AxiomOffice.Commands";
            _thread.SetApartmentState(ApartmentState.STA);
            _thread.Start();
        }

        // Chạy `work` trên thread lệnh; quá timeoutMs thì trả lỗi HOST_BUSY (lệnh chưa chạy thì không bao giờ chạy nữa).
        public object Run(string action, Func<object> work, int timeoutMs, Func<string, object> error)
        {
            var job = new Job { Work = work, Action = action };
            _queue.Add(job);
            if (job.Done.Wait(timeoutMs))
            {
                return job.Failure == null ? job.Result : error("internal error: " + job.Failure);
            }
            bool started = Interlocked.CompareExchange(ref job.State, 3, 0) != 0;
            if (started && job.Done.Wait(0))
            {
                // Xong đúng lúc vừa quá hạn: trả kết quả thật.
                return job.Failure == null ? job.Result : error("internal error: " + job.Failure);
            }
            string running = _running;
            string message = started
                ? "HOST_BUSY: the host has not finished '" + action + "' after " + (timeoutMs / 1000) + " s (it is still running)."
                : "HOST_BUSY: '" + action + "' was not run because '" + (running ?? "another command") + "' is still running in the host.";
            string dialog = HostDialogs.Describe();
            if (dialog != null)
            {
                message += " The host is showing a dialog: " + dialog + ". Ask the user to close it, then try again.";
            }
            Logger.Error(message, null);
            return error(message);
        }

        private void Loop()
        {
            foreach (Job job in _queue.GetConsumingEnumerable())
            {
                if (Interlocked.CompareExchange(ref job.State, 1, 0) != 0)
                {
                    Logger.Info("CommandWorker: skipped '" + job.Action + "' (the client was already told HOST_BUSY)");
                    continue;
                }
                _running = job.Action;
                try
                {
                    job.Result = job.Work();
                }
                catch (Exception ex)
                {
                    Logger.Error("CommandWorker: '" + job.Action + "' failed", ex);
                    job.Failure = ex.GetType().Name + ": " + ex.Message;
                }
                finally
                {
                    _running = null;
                    job.State = 2;
                    job.Done.Set();
                }
            }
        }

        public void Dispose()
        {
            _queue.CompleteAdding();
        }
    }
}
