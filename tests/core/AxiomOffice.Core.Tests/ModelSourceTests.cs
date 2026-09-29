using AxiomOffice.Core.Config;
using AxiomOffice.Core.Models;

namespace AxiomOffice.Core.Tests;

// Doi provider/model/key trong Cai dat co hieu luc o luot chay tiep theo, khong can khoi dong lai Core.
public class ModelSourceTests
{
    [Fact]
    public void Cau_hinh_khong_doi_thi_dung_lai_client_doi_thi_tao_moi()
    {
        var config = new CoreConfig { LlmProvider = "openai", LlmEndpoint = "http://a/v1", LlmModel = "m1", LlmApiKey = "k" };
        var source = new ModelSource(new HttpClient(), () => config);

        ModelClient first = source.Current();
        Assert.Same(first, source.Current());
        Assert.Equal("m1", first.Model);

        config = new CoreConfig
        {
            LlmProvider = "openai",
            LlmEndpoint = "https://generativelanguage.googleapis.com/v1beta/openai",
            LlmModel = "gemini-2.5-flash",
            LlmApiKey = "k2",
        };
        ModelClient second = source.Current();

        Assert.NotSame(first, second);
        Assert.Equal("gemini-2.5-flash", second.Model);
        Assert.Equal("openai", second.Codec.Name);

        config = new CoreConfig { LlmProvider = "anthropic", LlmEndpoint = "https://api.anthropic.com/v1", LlmModel = "claude", LlmApiKey = "k3" };
        Assert.Equal("anthropic", source.Current().Codec.Name);
    }
}
