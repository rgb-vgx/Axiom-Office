using System.Text;
using System.Text.Json;

namespace AxiomOffice.Core.Config;

// Noi dung %LOCALAPPDATA%\AxiomOffice\core.json (New_arch.md muc 7.1): Core ghi khi san sang de
// add-in tim thay, xoa khi thoat. Ghi atomic (.tmp + move) de ben doc khong gap file do dang.
public sealed record CoreFileInfo(int Pid, int Port, string Version, string Started, int Protocol, string Exe);

public static class CoreFile
{
    private static readonly JsonSerializerOptions Json = new(JsonSerializerDefaults.Web)
    {
        WriteIndented = true,
    };

    public static void Write(string path, CoreFileInfo info)
    {
        try
        {
            Directory.CreateDirectory(Path.GetDirectoryName(path)!);
            string temp = path + ".tmp";
            File.WriteAllText(temp, JsonSerializer.Serialize(info, Json), new UTF8Encoding(false));
            File.Move(temp, path, overwrite: true);
        }
        catch (Exception ex)
        {
            // Khong ghi duoc core.json thi add-in se khoi dong Core moi lan - khong lam Core dung.
            Logging.CoreLog.Error("Cannot write " + path, ex);
        }
    }

    public static CoreFileInfo? Read(string path)
    {
        try
        {
            if (!File.Exists(path))
            {
                return null;
            }

            return JsonSerializer.Deserialize<CoreFileInfo>(File.ReadAllText(path, Encoding.UTF8), Json);
        }
        catch
        {
            // File dang duoc ghi lai / hong: coi nhu chua co.
            return null;
        }
    }

    public static void Delete(string path)
    {
        try
        {
            if (File.Exists(path))
            {
                File.Delete(path);
            }
        }
        catch (Exception ex)
        {
            Logging.CoreLog.Error("Cannot delete " + path, ex);
        }
    }
}
