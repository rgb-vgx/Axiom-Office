using AxiomOffice.Core.Config;
using AxiomOffice.Core.Logging;

namespace AxiomOffice.Core.Models;

// ModelClient cho tung luot chay: doc lai cau hinh LLM (HKCU + AXIOM_*) moi luot de doi provider/model/key
// trong Cai dat co hieu luc ngay, khong phai khoi dong lai Core (truoc day chi doc mot lan luc khoi dong).
// Cau hinh khong doi thi dung lai client cu.
public sealed class ModelSource(HttpClient http, Func<CoreConfig> load)
{
    private readonly object _gate = new();
    private ModelClient? _current;
    private string? _signature;

    public ModelClient Current()
    {
        CoreConfig config = load();
        string signature = string.Join("\n", config.LlmProvider, config.LlmEndpoint, config.LlmModel, config.LlmApiKey);
        lock (_gate)
        {
            if (_current == null || signature != _signature)
            {
                if (_current != null)
                {
                    CoreLog.Info($"LLM config changed: provider={config.LlmProvider} model={config.LlmModel}");
                }

                _current = new ModelClient(http, config.LlmProvider, config.LlmEndpoint, config.LlmApiKey, config.LlmModel);
                _signature = signature;
            }

            return _current;
        }
    }
}
