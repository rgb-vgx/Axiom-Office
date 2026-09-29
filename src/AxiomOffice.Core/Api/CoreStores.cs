using AxiomOffice.Core.Config;
using AxiomOffice.Core.Logging;
using AxiomOffice.Core.Memory;

namespace AxiomOffice.Core.Api;

// Cac kho du lieu cua Core. Migration loi -> memory "unavailable" (Core van chay agent) - muc 8.5.1.
public sealed class CoreStores
{
    public CoreStores(CorePaths paths)
    {
        Db = new CoreDb(paths.DatabaseFile);
        Conversations = new ConversationStore(Db);
        Runs = new RunStore(Db);

        try
        {
            Db.Migrate();
            Available = true;
        }
        catch (Exception ex)
        {
            Available = false;
            Error = ex.Message;
            CoreLog.Error("memory unavailable: " + paths.DatabaseFile, ex);
        }
    }

    public CoreDb Db { get; }

    public ConversationStore Conversations { get; }

    public RunStore Runs { get; }

    public bool Available { get; }

    public string? Error { get; }

    // Gia tri cho /health: "on" | "off" | "unavailable".
    public string Status(bool memoryEnabled)
    {
        if (!memoryEnabled)
        {
            return "off";
        }

        return Available ? "on" : "unavailable";
    }
}
