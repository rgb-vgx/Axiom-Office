using System.Text.Json.Nodes;

namespace AxiomOffice.Core.Models;

// Danh sach model cua mot may chu AI: GET {endpoint}/models. Dung cho wizard thiet lap de nguoi dung
// CHON model thay vi tu go ten (ca OpenAI, Anthropic va cac proxy tuong thich deu tra ve {"data":[{"id":...}]}).
public static class ModelCatalog
{
    public const int DefaultTimeoutMs = 20_000;
    private const int MaxModels = 300;

    // Loi tra ve co cung hinh dang voi ModelClient ("HTTP 401: ...", "request timed out after Ns",
    // "HttpRequestException: ...") de Setup/LlmErrors dich sang cau tieng Viet nhu nhau.
    public static async Task<(List<string>? Models, string? Error)> ListAsync(
        HttpClient http, string provider, string endpoint, string apiKey, CancellationToken cancel, int timeoutMs = DefaultTimeoutMs)
    {
        string baseUrl = ModelClient.NormalizeEndpoint(endpoint);
        if (baseUrl.Length == 0)
        {
            return (null, "Endpoint is not configured");
        }

        using var cts = CancellationTokenSource.CreateLinkedTokenSource(cancel);
        cts.CancelAfter(timeoutMs);
        try
        {
            using var request = new HttpRequestMessage(HttpMethod.Get, ModelClient.BuildUrl(baseUrl, "/models"));
            ModelClient.ApplyAuth(request, provider, apiKey);
            using HttpResponseMessage response = await http.SendAsync(request, cts.Token).ConfigureAwait(false);
            string text = await response.Content.ReadAsStringAsync(CancellationToken.None).ConfigureAwait(false);
            if (!response.IsSuccessStatusCode)
            {
                return (null, $"HTTP {(int)response.StatusCode}: {Truncate(text, 300)}");
            }

            return (Parse(text), null);
        }
        catch (OperationCanceledException) when (!cancel.IsCancellationRequested)
        {
            return (null, $"request timed out after {timeoutMs / 1000}s");
        }
        catch (OperationCanceledException)
        {
            return (null, "cancelled");
        }
        catch (Exception ex)
        {
            return (null, ex.GetType().Name + ": " + ex.Message);
        }
    }

    // {"data":[{"id":"gpt-4o-mini",...}, ...]} -> ten model, bo trung, sap xep. Khong co "data" -> loi dinh dang.
    public static List<string> Parse(string json)
    {
        JsonNode? root;
        try
        {
            root = JsonNode.Parse(json);
        }
        catch (Exception)
        {
            return [];
        }

        if (root?["data"] is not JsonArray array)
        {
            return [];
        }

        var names = new List<string>();
        foreach (JsonNode? item in array)
        {
            string? id = item?["id"]?.GetValue<string>();
            if (!string.IsNullOrWhiteSpace(id))
            {
                names.Add(id.Trim());
            }
        }

        return names.Distinct(StringComparer.Ordinal).OrderBy(n => n, StringComparer.Ordinal).Take(MaxModels).ToList();
    }

    private static string Truncate(string? text, int limit)
    {
        string value = (text ?? "").Trim();
        return value.Length <= limit ? value : value[..limit];
    }
}
