using System.Text.Encodings.Web;
using System.Text.Json;

namespace AxiomOffice.Core.Api;

// Hinh dang response cua Core giong bridge: {"ok":true,"result":...} hoac
// {"ok":false,"error":"..."} (New_arch.md muc 7.3).
public static class ApiJson
{
    public static readonly JsonSerializerOptions Options = new(JsonSerializerDefaults.Web)
    {
        // Giu tieng Viet doc duoc trong response (giong bridge/JavaScriptSerializer).
        Encoder = JavaScriptEncoder.UnsafeRelaxedJsonEscaping,
    };

    public static IResult Ok(object? result = null)
    {
        return Results.Json(new Dictionary<string, object?> { ["ok"] = true, ["result"] = result }, Options);
    }

    // Loi nghiep vu tra HTTP 200 kem ok:false (giong bridge); statusCode dung cho loi giao thuc.
    public static IResult Error(string message, int statusCode = 200)
    {
        return Results.Json(new Dictionary<string, object?> { ["ok"] = false, ["error"] = message }, Options, statusCode: statusCode);
    }

    public static async Task WriteErrorAsync(HttpContext context, int statusCode, string message)
    {
        context.Response.StatusCode = statusCode;
        context.Response.ContentType = "application/json; charset=utf-8";
        var payload = new Dictionary<string, object?> { ["ok"] = false, ["error"] = message };
        await context.Response.WriteAsync(JsonSerializer.Serialize(payload, Options), context.RequestAborted);
    }
}
