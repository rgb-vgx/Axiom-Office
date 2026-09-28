using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.Diagnostics;
using System.Net;
using System.Runtime.InteropServices;
using System.Text;
using System.Threading;
using System.Web.Script.Serialization;

namespace WpsAiBridge.Bridge
{
    // GET /events: Server-Sent Events báo thay đổi vùng chọn / tài liệu.
    // V1 dùng poll-diff 500ms (không cần COM event sink nên chạy giống nhau trên Office và WPS).
    // Mỗi subscriber có thread ghi riêng; Pump chính chỉ đăng ký rồi quay lại nhận request khác.
    internal sealed class EventStream
    {
        private const int MaxSubscribers = 5;
        private const int PollMs = 500;
        private const int PingMs = 15000;
        private const int MaxText = 200;
        private const int MaxSelectionReadChars = 5000;

        private readonly IAppHost _host;
        private readonly int _port;
        private readonly object _sync = new object();
        private readonly List<Subscriber> _subscribers = new List<Subscriber>();
        private readonly Dictionary<string, DateTime> _lastErrorLog = new Dictionary<string, DateTime>();
        private readonly JavaScriptSerializer _json = new JavaScriptSerializer();
        private Thread _poller;
        private bool _stopped;
        private int _nextId;
        private string _documentKey;
        private string _documentEvent;
        private string _selectionKey;
        private string _selectionEvent;

        public EventStream(IAppHost host, int port)
        {
            _host = host;
            _port = port;
        }

        // Gọi trên thread Pump: không block, không ghi gì ngoài lỗi 503.
        public void Accept(HttpListenerContext context)
        {
            Subscriber subscriber;
            lock (_sync)
            {
                if (_stopped || _subscribers.Count >= MaxSubscribers)
                {
                    RejectBusy(context, _stopped ? "event stream stopped" : "too many subscribers (max " + MaxSubscribers + ")");
                    return;
                }
                subscriber = new Subscriber(++_nextId, context);
                _subscribers.Add(subscriber);
                subscriber.Enqueue(Format("hello", new Dictionary<string, object>
                {
                    { "app", _host.AppKind },
                    { "family", _host.IsOfficeHost ? "office" : "wps" },
                    { "port", _port },
                    { "pid", Process.GetCurrentProcess().Id },
                    { "subscribers", _subscribers.Count }
                }));
                if (_documentEvent != null)
                {
                    subscriber.Enqueue(_documentEvent);
                }
                if (_selectionEvent != null)
                {
                    subscriber.Enqueue(_selectionEvent);
                }
                if (_poller == null)
                {
                    _poller = new Thread(Poll);
                    _poller.IsBackground = true;
                    _poller.Name = "WpsAiBridge.EventPoll";
                    _poller.Start();
                }
            }
            var writer = new Thread(delegate() { Serve(subscriber); });
            writer.IsBackground = true;
            writer.Name = "WpsAiBridge.Events#" + subscriber.Id;
            writer.Start();
            Logger.Info("Events: subscriber #" + subscriber.Id + " connected (" + SubscriberCount() + " active)");
        }

        public void Stop()
        {
            List<Subscriber> subscribers;
            lock (_sync)
            {
                _stopped = true;
                subscribers = new List<Subscriber>(_subscribers);
            }
            foreach (Subscriber subscriber in subscribers)
            {
                subscriber.Close();
            }
        }

        private int SubscriberCount()
        {
            lock (_sync)
            {
                return _subscribers.Count;
            }
        }

        private void Serve(Subscriber subscriber)
        {
            try
            {
                HttpListenerResponse response = subscriber.Context.Response;
                response.StatusCode = 200;
                response.ContentType = "text/event-stream; charset=utf-8";
                response.SendChunked = true;
                response.Headers["Cache-Control"] = "no-cache";
                response.Headers["X-Accel-Buffering"] = "no";
                var output = response.OutputStream;
                DateTime nextPing = DateTime.UtcNow.AddMilliseconds(PingMs);
                while (true)
                {
                    int wait = (int)Math.Max(0, (nextPing - DateTime.UtcNow).TotalMilliseconds);
                    string message;
                    if (!subscriber.Queue.TryTake(out message, wait))
                    {
                        if (subscriber.Queue.IsAddingCompleted)
                        {
                            break;
                        }
                        message = Format("ping", new Dictionary<string, object>
                        {
                            { "time", DateTime.UtcNow.ToString("yyyy-MM-ddTHH:mm:ss.fffZ") },
                            { "subscribers", SubscriberCount() }
                        });
                        nextPing = DateTime.UtcNow.AddMilliseconds(PingMs);
                    }
                    byte[] bytes = Encoding.UTF8.GetBytes(message);
                    output.Write(bytes, 0, bytes.Length);
                    output.Flush();
                }
            }
            catch (Exception ex)
            {
                // Client ngắt kết nối: lần ghi kế tiếp ném HttpListenerException/IOException.
                if (!(ex is HttpListenerException) && !(ex is System.IO.IOException) && !(ex is ObjectDisposedException) && !(ex is InvalidOperationException))
                {
                    Logger.Error("Events: subscriber #" + subscriber.Id + " failed", ex);
                }
            }
            finally
            {
                lock (_sync)
                {
                    _subscribers.Remove(subscriber);
                }
                subscriber.Close();
                Logger.Info("Events: subscriber #" + subscriber.Id + " disconnected (" + SubscriberCount() + " active)");
            }
        }

        private void Poll()
        {
            while (true)
            {
                lock (_sync)
                {
                    if (_stopped || _subscribers.Count == 0)
                    {
                        _poller = null;
                        _documentKey = null;
                        _documentEvent = null;
                        _selectionKey = null;
                        _selectionEvent = null;
                        return;
                    }
                }
                try
                {
                    // Bridge đang chạy lệnh trong host thì bỏ qua vòng này (không chen cuộc gọi COM thứ hai).
                    ComGate.TryRun(0, delegate
                    {
                        PollDocument();
                        PollSelection();
                    });
                }
                catch (Exception ex)
                {
                    LogThrottled(ex);
                }
                Thread.Sleep(PollMs);
            }
        }

        private void PollDocument()
        {
            Dictionary<string, object> doc;
            try
            {
                doc = HostProbe.ActiveDocument(_host);
            }
            catch (COMException ex)
            {
                if (HostProbe.IsRetryable(ex))
                {
                    return;
                }
                throw;
            }
            string key = doc == null ? "" : Convert.ToString(doc["fullName"]);
            if (key == _documentKey)
            {
                return;
            }
            _documentKey = key;
            var data = new Dictionary<string, object>
            {
                { "app", _host.AppKind },
                { "name", doc == null ? null : doc["name"] },
                { "fullName", doc == null ? null : doc["fullName"] }
            };
            Broadcast("document", data, true);
        }

        private void PollSelection()
        {
            if (string.IsNullOrEmpty(_documentKey))
            {
                return;
            }
            Dictionary<string, object> data;
            string key;
            try
            {
                if (_host.AppKind == "et")
                {
                    data = ReadSpreadsheetSelection(out key);
                }
                else if (_host.AppKind == "wpp")
                {
                    data = ReadPresentationSelection(out key);
                }
                else
                {
                    data = ReadWriterSelection(out key);
                }
            }
            catch (COMException ex)
            {
                if (HostProbe.IsRetryable(ex))
                {
                    return;
                }
                throw;
            }
            if (data == null || key == _selectionKey)
            {
                return;
            }
            _selectionKey = key;
            Broadcast("selection", data, false);
        }

        private Dictionary<string, object> ReadWriterSelection(out string key)
        {
            dynamic app = _host.Application;
            dynamic selection = app.Selection;
            int start = Convert.ToInt32(selection.Start);
            int end = Convert.ToInt32(selection.End);
            key = _documentKey + "|" + start + "|" + end;
            if (key == _selectionKey)
            {
                return new Dictionary<string, object>();
            }
            string text = "";
            if (end > start)
            {
                // Vùng chọn rất lớn: chỉ đọc đoạn đầu thay vì kéo cả tài liệu qua COM mỗi 500ms.
                text = end - start > MaxSelectionReadChars
                    ? Convert.ToString(app.ActiveDocument.Range(start, start + MaxText + 1).Text)
                    : Convert.ToString(selection.Text);
            }
            return new Dictionary<string, object>
            {
                { "app", _host.AppKind },
                { "text", Clip(text) },
                { "start", start },
                { "end", end }
            };
        }

        private Dictionary<string, object> ReadSpreadsheetSelection(out string key)
        {
            dynamic app = _host.Application;
            dynamic selection = app.Selection;
            string address;
            try
            {
                address = Convert.ToString(selection.Address);
            }
            catch (COMException ex)
            {
                if (HostProbe.IsRetryable(ex))
                {
                    throw;
                }
                // Đang chọn chart/shape, không phải ô.
                address = null;
            }
            catch (Microsoft.CSharp.RuntimeBinder.RuntimeBinderException)
            {
                address = null;
            }
            string sheet = Convert.ToString(app.ActiveSheet.Name);
            string text = "";
            if (address != null)
            {
                // Giá trị ô trên-trái (chọn cả cột/cả sheet thì Count có thể tràn, nên không dùng Value2 của cả vùng).
                text = Convert.ToString(selection.Cells[1, 1].Value2);
            }
            key = _documentKey + "|" + sheet + "|" + address + "|" + text;
            return new Dictionary<string, object>
            {
                { "app", _host.AppKind },
                { "sheet", sheet },
                { "address", address },
                { "text", Clip(text) },
                { "start", null },
                { "end", null }
            };
        }

        private Dictionary<string, object> ReadPresentationSelection(out string key)
        {
            dynamic app = _host.Application;
            dynamic window = app.ActiveWindow;
            dynamic selection = window.Selection;
            int type = Convert.ToInt32(selection.Type);
            object slide = null;
            try
            {
                slide = Convert.ToInt32(window.View.Slide.SlideIndex);
            }
            catch (Exception)
            {
            }
            string kind = type == 3 ? "text" : type == 2 ? "shapes" : type == 1 ? "slides" : "none";
            string shape = null;
            string text = "";
            object start = null;
            object end = null;
            if (type == 3)
            {
                dynamic range = selection.TextRange;
                text = Convert.ToString(range.Text);
                int s = Convert.ToInt32(range.Start);
                start = s;
                end = s + Convert.ToInt32(range.Length);
                try
                {
                    shape = Convert.ToString(selection.ShapeRange[1].Name);
                }
                catch (Exception)
                {
                }
            }
            else if (type == 2)
            {
                dynamic first = selection.ShapeRange[1];
                shape = Convert.ToString(first.Name);
                try
                {
                    if (Convert.ToInt32(first.HasTextFrame) != 0)
                    {
                        text = Convert.ToString(first.TextFrame.TextRange.Text);
                    }
                }
                catch (Exception)
                {
                }
            }
            key = _documentKey + "|" + slide + "|" + kind + "|" + shape + "|" + start + "|" + end + "|" + text;
            return new Dictionary<string, object>
            {
                { "app", _host.AppKind },
                { "slide", slide },
                { "type", kind },
                { "shape", shape },
                { "text", Clip(text) },
                { "start", start },
                { "end", end }
            };
        }

        private void Broadcast(string name, Dictionary<string, object> data, bool isDocument)
        {
            string message = Format(name, data);
            lock (_sync)
            {
                if (isDocument)
                {
                    _documentEvent = message;
                    // Tài liệu đổi thì vùng chọn cũ không còn nghĩa; lần poll sau phát lại selection.
                    _selectionKey = null;
                    _selectionEvent = null;
                }
                else
                {
                    _selectionEvent = message;
                }
                foreach (Subscriber subscriber in _subscribers)
                {
                    subscriber.Enqueue(message);
                }
            }
        }

        private string Format(string name, Dictionary<string, object> data)
        {
            return "event: " + name + "\ndata: " + _json.Serialize(data) + "\n\n";
        }

        private static string Clip(string text)
        {
            text = text ?? "";
            return text.Length > MaxText ? text.Substring(0, MaxText) : text;
        }

        // Tối đa 1 dòng log mỗi phút cho mỗi loại lỗi, tránh spam khi host bận lâu.
        private void LogThrottled(Exception ex)
        {
            var com = ex as COMException;
            string kind = ex.GetType().Name + (com != null ? ":" + com.HResult.ToString("X8") : "");
            DateTime now = DateTime.UtcNow;
            DateTime last;
            if (_lastErrorLog.TryGetValue(kind, out last) && (now - last).TotalSeconds < 60)
            {
                return;
            }
            _lastErrorLog[kind] = now;
            Logger.Info("Events: poll skipped (" + kind + "): " + ex.Message);
        }

        private static void RejectBusy(HttpListenerContext context, string message)
        {
            try
            {
                byte[] body = Encoding.UTF8.GetBytes(new JavaScriptSerializer().Serialize(new Dictionary<string, object>
                {
                    { "ok", false },
                    { "error", message }
                }));
                context.Response.StatusCode = 503;
                context.Response.ContentType = "application/json; charset=utf-8";
                context.Response.ContentLength64 = body.Length;
                context.Response.OutputStream.Write(body, 0, body.Length);
                context.Response.OutputStream.Close();
            }
            catch (Exception)
            {
            }
        }

        private sealed class Subscriber
        {
            public readonly int Id;
            public readonly HttpListenerContext Context;
            public readonly BlockingCollection<string> Queue = new BlockingCollection<string>(256);
            private int _closed;

            public Subscriber(int id, HttpListenerContext context)
            {
                Id = id;
                Context = context;
            }

            public void Enqueue(string message)
            {
                // Client đọc chậm: bỏ event thay vì chặn poller.
                try
                {
                    Queue.TryAdd(message);
                }
                catch (InvalidOperationException)
                {
                }
            }

            public void Close()
            {
                if (Interlocked.Exchange(ref _closed, 1) != 0)
                {
                    return;
                }
                try
                {
                    Queue.CompleteAdding();
                }
                catch (Exception)
                {
                }
                try
                {
                    Context.Response.Abort();
                }
                catch (Exception)
                {
                }
            }
        }
    }
}
