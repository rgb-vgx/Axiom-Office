using AxiomOffice.Core.Config;

namespace AxiomOffice.Core.Api;

// Cung quy tac voi bridge (New_arch.md muc 7.2): chan request co header Origin (403), moi endpoint
// tru kiem tra song /health deu can X-Auth-Token (401) khi Token duoc cau hinh, va request co body
// phai dung Content-Type application/json (415).
public static class CoreApiGuard
{
    private static readonly string[] BodyMethods = ["POST", "PUT", "PATCH"];

    public static void Use(WebApplication app, CoreConfig config)
    {
        app.Use(async (context, next) =>
        {
            if (!string.IsNullOrEmpty(context.Request.Headers.Origin))
            {
                await ApiJson.WriteErrorAsync(context, 403, "requests with an Origin header are not allowed");
                return;
            }

            if (IsHealth(context))
            {
                await next(context);
                return;
            }

            if (!string.IsNullOrEmpty(config.Token)
                && !string.Equals(context.Request.Headers["X-Auth-Token"], config.Token, StringComparison.Ordinal))
            {
                await ApiJson.WriteErrorAsync(context, 401, "unauthorized");
                return;
            }

            if (HasBody(context) && !IsJson(context.Request.ContentType))
            {
                await ApiJson.WriteErrorAsync(context, 415, "Content-Type must be application/json");
                return;
            }

            await next(context);
        });
    }

    private static bool IsHealth(HttpContext context)
    {
        return HttpMethods.IsGet(context.Request.Method)
            && string.Equals(context.Request.Path.Value?.TrimEnd('/'), "/health", StringComparison.Ordinal);
    }

    private static bool HasBody(HttpContext context)
    {
        return Array.Exists(BodyMethods, m => string.Equals(context.Request.Method, m, StringComparison.OrdinalIgnoreCase))
            && (context.Request.ContentLength > 0 || context.Request.Headers.TransferEncoding.Count > 0);
    }

    private static bool IsJson(string? contentType)
    {
        return contentType != null && contentType.Contains("application/json", StringComparison.OrdinalIgnoreCase);
    }
}
