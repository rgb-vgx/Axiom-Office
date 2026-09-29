using System.Collections.Concurrent;
using System.Diagnostics;
using System.Net.Http.Headers;
using System.Text;
using System.Text.Json.Nodes;
using AxiomOffice.Core.Logging;

namespace AxiomOffice.Core.Mcp;

// Kenh JSON-RPC 2.0 toi mot MCP server (New_arch.md muc 8.7).
public interface IMcpTransport : IDisposable
{
    bool IsAlive { get; }

    Task<JsonNode?> RequestAsync(string method, JsonNode? parameters, CancellationToken cancel);

    Task NotifyAsync(string method, JsonNode? parameters, CancellationToken cancel);
}

public sealed class McpException(string message) : Exception(message);

// stdio: moi thong diep JSON mot dong tren stdin/stdout cua process con (chuan MCP stdio transport).
public sealed class StdioMcpTransport : IMcpTransport
{
    private readonly Process _process;
    private readonly ConcurrentDictionary<long, TaskCompletionSource<JsonNode>> _pending = new();
    private readonly SemaphoreSlim _writeLock = new(1, 1);
    private readonly string _name;
    private long _nextId;

    public StdioMcpTransport(string name, string command, IReadOnlyList<string> args, IReadOnlyDictionary<string, string> env, string? workingDirectory)
    {
        _name = name;
        var start = new ProcessStartInfo(command)
        {
            UseShellExecute = false,
            RedirectStandardInput = true,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            CreateNoWindow = true,
            StandardOutputEncoding = new UTF8Encoding(false),
            StandardInputEncoding = new UTF8Encoding(false),
            WorkingDirectory = workingDirectory ?? "",
        };
        foreach (string arg in args)
        {
            start.ArgumentList.Add(arg);
        }

        foreach ((string key, string value) in env)
        {
            start.Environment[key] = value;
        }

        _process = Process.Start(start) ?? throw new McpException("cannot start " + command);
        _ = Task.Run(ReadLoopAsync);
        _ = Task.Run(async () =>
        {
            // stderr cua server: log vai dong dau de chan doan, khong de day pipe.
            int lines = 0;
            while (await _process.StandardError.ReadLineAsync().ConfigureAwait(false) is { } line)
            {
                if (lines++ < 20)
                {
                    CoreLog.Info($"mcp {_name} stderr: {line}");
                }
            }
        });
    }

    public bool IsAlive => !_process.HasExited;

    public async Task<JsonNode?> RequestAsync(string method, JsonNode? parameters, CancellationToken cancel)
    {
        long id = Interlocked.Increment(ref _nextId);
        var waiter = new TaskCompletionSource<JsonNode>(TaskCreationOptions.RunContinuationsAsynchronously);
        _pending[id] = waiter;
        try
        {
            await WriteAsync(new JsonObject { ["jsonrpc"] = "2.0", ["id"] = id, ["method"] = method, ["params"] = parameters?.DeepClone() }, cancel).ConfigureAwait(false);
            using (cancel.Register(() => waiter.TrySetCanceled(cancel)))
            {
                JsonNode response = await waiter.Task.ConfigureAwait(false);
                if (response["error"] is JsonObject error)
                {
                    throw new McpException(error["message"]?.ToString() ?? error.ToJsonString());
                }

                return response["result"];
            }
        }
        finally
        {
            _pending.TryRemove(id, out _);
        }
    }

    public Task NotifyAsync(string method, JsonNode? parameters, CancellationToken cancel)
    {
        return WriteAsync(new JsonObject { ["jsonrpc"] = "2.0", ["method"] = method, ["params"] = parameters?.DeepClone() }, cancel);
    }

    private async Task WriteAsync(JsonObject message, CancellationToken cancel)
    {
        if (message["params"] == null)
        {
            message.Remove("params");
        }

        await _writeLock.WaitAsync(cancel).ConfigureAwait(false);
        try
        {
            if (_process.HasExited)
            {
                throw new McpException($"mcp server '{_name}' has exited (code {_process.ExitCode})");
            }

            await _process.StandardInput.WriteLineAsync(message.ToJsonString()).ConfigureAwait(false);
            await _process.StandardInput.FlushAsync(cancel).ConfigureAwait(false);
        }
        finally
        {
            _writeLock.Release();
        }
    }

    private async Task ReadLoopAsync()
    {
        try
        {
            while (await _process.StandardOutput.ReadLineAsync().ConfigureAwait(false) is { } line)
            {
                if (string.IsNullOrWhiteSpace(line))
                {
                    continue;
                }

                JsonNode? message;
                try
                {
                    message = JsonNode.Parse(line);
                }
                catch (System.Text.Json.JsonException)
                {
                    continue;
                }

                if (message?["id"] is JsonValue idValue && message["method"] == null && idValue.TryGetValue(out long id))
                {
                    if (_pending.TryGetValue(id, out TaskCompletionSource<JsonNode>? waiter))
                    {
                        waiter.TrySetResult(message);
                    }
                }
                else if (message?["id"] != null && message["method"] != null)
                {
                    // Server hoi nguoc client (sampling, roots...): chua ho tro.
                    await WriteAsync(new JsonObject
                    {
                        ["jsonrpc"] = "2.0",
                        ["id"] = message["id"]!.DeepClone(),
                        ["error"] = new JsonObject { ["code"] = -32601, ["message"] = "method not supported by Axiom Office" },
                    }, CancellationToken.None).ConfigureAwait(false);
                }
            }
        }
        catch (Exception ex)
        {
            CoreLog.Info($"mcp {_name} read loop ended: {ex.Message}");
        }
        finally
        {
            foreach (TaskCompletionSource<JsonNode> waiter in _pending.Values)
            {
                waiter.TrySetException(new McpException($"mcp server '{_name}' closed the connection"));
            }
        }
    }

    public void Dispose()
    {
        try
        {
            _process.StandardInput.Close();
            if (!_process.WaitForExit(2000))
            {
                _process.Kill(entireProcessTree: true);
            }
        }
        catch (Exception)
        {
        }

        _process.Dispose();
    }
}

// Streamable HTTP: POST JSON-RPC, tra ve JSON hoac SSE (data: {...}); giu Mcp-Session-Id sau initialize.
public sealed class HttpMcpTransport(string name, string url, IReadOnlyDictionary<string, string> headers, HttpClient http) : IMcpTransport
{
    private long _nextId;
    private string? _sessionId;

    public bool IsAlive => true;

    public async Task<JsonNode?> RequestAsync(string method, JsonNode? parameters, CancellationToken cancel)
    {
        long id = Interlocked.Increment(ref _nextId);
        JsonNode? response = await PostAsync(new JsonObject { ["jsonrpc"] = "2.0", ["id"] = id, ["method"] = method, ["params"] = parameters?.DeepClone() }, id, cancel).ConfigureAwait(false);
        if (response?["error"] is JsonObject error)
        {
            throw new McpException(error["message"]?.ToString() ?? error.ToJsonString());
        }

        return response?["result"];
    }

    public async Task NotifyAsync(string method, JsonNode? parameters, CancellationToken cancel)
    {
        await PostAsync(new JsonObject { ["jsonrpc"] = "2.0", ["method"] = method, ["params"] = parameters?.DeepClone() }, null, cancel).ConfigureAwait(false);
    }

    private async Task<JsonNode?> PostAsync(JsonObject message, long? id, CancellationToken cancel)
    {
        if (message["params"] == null)
        {
            message.Remove("params");
        }

        using var request = new HttpRequestMessage(HttpMethod.Post, url)
        {
            Content = new StringContent(message.ToJsonString(), Encoding.UTF8, "application/json"),
        };
        request.Headers.Accept.Add(new MediaTypeWithQualityHeaderValue("application/json"));
        request.Headers.Accept.Add(new MediaTypeWithQualityHeaderValue("text/event-stream"));
        request.Headers.TryAddWithoutValidation("MCP-Protocol-Version", McpServer.ProtocolVersion);
        if (_sessionId != null)
        {
            request.Headers.TryAddWithoutValidation("Mcp-Session-Id", _sessionId);
        }

        foreach ((string key, string value) in headers)
        {
            request.Headers.TryAddWithoutValidation(key, value);
        }

        using HttpResponseMessage response = await http.SendAsync(request, HttpCompletionOption.ResponseHeadersRead, cancel).ConfigureAwait(false);
        if (response.Headers.TryGetValues("Mcp-Session-Id", out IEnumerable<string>? session))
        {
            _sessionId = session.FirstOrDefault() ?? _sessionId;
        }

        if (id == null)
        {
            return null;
        }

        string body = await response.Content.ReadAsStringAsync(cancel).ConfigureAwait(false);
        if (!response.IsSuccessStatusCode)
        {
            throw new McpException($"mcp server '{name}' HTTP {(int)response.StatusCode}: {Models.ModelClient.Truncate(body, 200)}");
        }

        if (response.Content.Headers.ContentType?.MediaType == "text/event-stream")
        {
            foreach (string line in body.Split('\n'))
            {
                if (!line.StartsWith("data:", StringComparison.Ordinal))
                {
                    continue;
                }

                JsonNode? node = JsonNode.Parse(line[5..].Trim());
                if (node?["id"] is JsonValue value && value.TryGetValue(out long got) && got == id)
                {
                    return node;
                }
            }

            throw new McpException($"mcp server '{name}' sent no response for request {id}");
        }

        return JsonNode.Parse(body);
    }

    public void Dispose()
    {
    }
}
