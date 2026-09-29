using System.Net;
using System.Text.Json;
using AxiomOffice.Core.Config;

namespace AxiomOffice.Core.Tests;

// Vong doi va bao mat cua Core API tren tien trinh that (New_arch.md muc 7.1, 7.2, 7.3).
public class CoreProcessTests
{
    private static async Task<JsonElement> ReadJson(HttpResponseMessage response)
    {
        string text = await response.Content.ReadAsStringAsync();
        Assert.False(string.IsNullOrWhiteSpace(text), "response phai co body JSON");
        return JsonDocument.Parse(text).RootElement;
    }

    [Fact]
    public async Task Health_tra_thong_tin_va_khong_can_token()
    {
        using CoreProcess core = CoreProcess.Start();
        using HttpClient client = core.Client();

        HttpResponseMessage response = await client.GetAsync("/health");
        JsonElement json = await ReadJson(response);

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        Assert.True(json.GetProperty("ok").GetBoolean());
        JsonElement result = json.GetProperty("result");
        Assert.Equal("core", result.GetProperty("app").GetString());
        Assert.Equal(core.Pid, result.GetProperty("pid").GetInt32());
        Assert.Equal(core.Port, result.GetProperty("port").GetInt32());
        Assert.Equal(CoreVersion.Value, result.GetProperty("version").GetString());
        Assert.Equal(CoreVersion.Protocol, result.GetProperty("protocol").GetInt32());
        Assert.True(result.GetProperty("uptimeSeconds").GetDouble() >= 0);
        Assert.Equal("on", result.GetProperty("memory").GetString());
    }

    [Fact]
    public void Core_json_khop_voi_tien_trinh_dang_chay()
    {
        using CoreProcess core = CoreProcess.Start();

        CoreFileInfo? info = CoreFile.Read(core.CoreJsonPath);

        Assert.NotNull(info);
        Assert.Equal(core.Pid, info.Pid);
        Assert.Equal(CoreVersion.Value, info.Version);
        Assert.Equal(CoreVersion.Protocol, info.Protocol);
        Assert.Equal(core.ExePath, info.Exe);
        Assert.False(string.IsNullOrWhiteSpace(info.Started));
    }

    [Fact]
    public async Task Thieu_hoac_sai_token_thi_401()
    {
        using CoreProcess core = CoreProcess.Start();

        using HttpClient anonymous = core.Client();
        HttpResponseMessage withoutToken = await anonymous.PostAsync("/v1/admin/shutdown", null);
        JsonElement json = await ReadJson(withoutToken);

        Assert.Equal(HttpStatusCode.Unauthorized, withoutToken.StatusCode);
        Assert.False(json.GetProperty("ok").GetBoolean());

        using var wrong = core.Client();
        wrong.DefaultRequestHeaders.Add("X-Auth-Token", "sai-token");
        Assert.Equal(HttpStatusCode.Unauthorized, (await wrong.PostAsync("/v1/admin/shutdown", null)).StatusCode);

        // Core van song sau khi bi tu choi.
        Assert.False(core.HasExited);
        Assert.Equal(HttpStatusCode.OK, (await core.Client().GetAsync("/health")).StatusCode);
    }

    [Fact]
    public async Task Request_co_origin_thi_403_ke_ca_health()
    {
        using CoreProcess core = CoreProcess.Start();
        using var request = new HttpRequestMessage(HttpMethod.Get, "/health");
        request.Headers.Add("Origin", "http://evil.example");

        HttpResponseMessage response = await core.Client().SendAsync(request);

        Assert.Equal(HttpStatusCode.Forbidden, response.StatusCode);
    }

    [Fact]
    public async Task Body_sai_content_type_thi_415()
    {
        using CoreProcess core = CoreProcess.Start();

        HttpResponseMessage response = await core.Client(withToken: true).PostAsync(
            "/v1/khong-ton-tai",
            new StringContent("x=1", System.Text.Encoding.UTF8, "text/plain"));

        Assert.Equal(HttpStatusCode.UnsupportedMediaType, response.StatusCode);
    }

    [Fact]
    public async Task Duong_dan_la_thi_404()
    {
        using CoreProcess core = CoreProcess.Start();

        HttpResponseMessage response = await core.Client(withToken: true).PostAsync("/v1/khong-ton-tai", null);

        Assert.Equal(HttpStatusCode.NotFound, response.StatusCode);
    }

    [Fact]
    public void Instance_thu_hai_thoat_ngay_voi_ma_0()
    {
        using CoreProcess core = CoreProcess.Start(singleInstance: true);

        using System.Diagnostics.Process second = core.StartSecondInstanceAndWait();

        Assert.Equal(0, second.ExitCode);
        Assert.False(core.HasExited, "instance dau tien phai van chay");
        Assert.Contains("already running", File.ReadAllText(core.LogFile));

        // core.json van thuoc instance dau tien (instance hai khong ghi de).
        CoreFileInfo? info = CoreFile.Read(core.CoreJsonPath);
        Assert.NotNull(info);
        Assert.Equal(core.Pid, info.Pid);
    }

    [Fact]
    public async Task Shutdown_dung_token_thi_thoat_va_xoa_core_json()
    {
        CoreProcess core = CoreProcess.Start();
        try
        {
            HttpResponseMessage response = await core.Client(withToken: true).PostAsync("/v1/admin/shutdown", null);
            JsonElement json = await ReadJson(response);

            Assert.Equal(HttpStatusCode.OK, response.StatusCode);
            Assert.True(json.GetProperty("ok").GetBoolean());

            var watch = System.Diagnostics.Stopwatch.StartNew();
            while (!core.HasExited && watch.ElapsedMilliseconds < 10000)
            {
                await Task.Delay(100);
            }

            Assert.True(core.HasExited, "Core phai thoat sau khi shutdown");
            Assert.False(File.Exists(core.CoreJsonPath), "core.json phai duoc xoa khi thoat");
            Assert.Contains("Agent Core stopped", File.ReadAllText(core.LogFile));
        }
        finally
        {
            core.Dispose();
        }
    }
}
