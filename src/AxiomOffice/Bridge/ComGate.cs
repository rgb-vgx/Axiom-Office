using System;
using System.Threading;

namespace AxiomOffice.Bridge
{
    // Mọi thread của bridge (Pump HTTP, agent của pane, heartbeat session, poller SSE) gọi object model
    // của host qua cổng này để tại một thời điểm chỉ một thread của ta ở trong host. Object model Office
    // không an toàn đa luồng: Word đã crash (AV trong wwlib.dll) khi heartbeat đọc ActiveDocument đúng lúc
    // ai.ask đang ghi — cuộc gọi thứ hai được dispatch reentrant trong vòng message nội bộ của Word.
    internal static class ComGate
    {
        private static readonly object Gate = new object();

        // Chờ tới lượt (lệnh /cmd, tool của agent).
        public static T Run<T>(Func<T> action)
        {
            lock (Gate)
            {
                return action();
            }
        }

        // Chỉ chạy nếu cổng rảnh trong timeoutMs (heartbeat, poller): host đang bận thì bỏ qua lượt này.
        public static bool TryRun(int timeoutMs, Action action)
        {
            if (!Monitor.TryEnter(Gate, timeoutMs))
            {
                return false;
            }
            try
            {
                action();
                return true;
            }
            finally
            {
                Monitor.Exit(Gate);
            }
        }
    }
}
