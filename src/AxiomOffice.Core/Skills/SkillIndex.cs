using AxiomOffice.Core.Logging;

namespace AxiomOffice.Core.Skills;

// Mot nguon skill: ten hien thi (builtin / org / user) + thu muc.
public sealed record SkillSource(string Name, string Directory);

// Chi muc skill (New_arch.md muc 8.4.4): quet cac nguon theo thu tu uu tien, nguon sau ghi de nguon truoc
// khi trung ten. Thu muc bat dau bang '_' (vd _design) la goi tai nguyen dung chung (tokens.json...), khong
// phai skill: doc qua read_skill_file. Snapshot bat bien nen doc tu nhieu run cung luc an toan.
public sealed class SkillIndex : IDisposable
{
    private sealed record Snapshot(
        IReadOnlyDictionary<string, SkillDefinition> Skills,
        IReadOnlyDictionary<string, string> Resources,
        IReadOnlyList<SkillError> Errors);

    public static readonly TimeSpan WatchDebounce = TimeSpan.FromSeconds(2);

    private readonly IReadOnlyList<SkillSource> _sources;
    private readonly List<FileSystemWatcher> _watchers = [];
    private readonly object _gate = new();
    private Snapshot _snapshot = new(new Dictionary<string, SkillDefinition>(), new Dictionary<string, string>(), []);
    private Timer? _debounce;

    public SkillIndex(IReadOnlyList<SkillSource> sources)
    {
        _sources = sources;
        Reload();
    }

    // builtin (skills\ canh exe) -> SkillDirs cua to chuc -> %LOCALAPPDATA%\AxiomOffice\skills cua nguoi dung.
    public static IReadOnlyList<SkillSource> DefaultSources(string builtinDirectory, IEnumerable<string> organizationDirectories, string userDirectory)
    {
        var sources = new List<SkillSource> { new("builtin", builtinDirectory) };
        sources.AddRange(organizationDirectories.Select(d => new SkillSource("org", d)));
        sources.Add(new SkillSource("user", userDirectory));
        return sources;
    }

    public IReadOnlyList<SkillSource> Sources => _sources;

    public IReadOnlyList<SkillDefinition> All => _snapshot.Skills.Values.OrderBy(s => s.Name, StringComparer.Ordinal).ToList();

    public IReadOnlyList<SkillError> Errors => _snapshot.Errors;

    public IReadOnlyList<SkillDefinition> ForApp(string appKind) => All.Where(s => s.AppliesTo(appKind)).ToList();

    public SkillDefinition? Find(string name)
    {
        return _snapshot.Skills.TryGetValue(name.Trim(), out SkillDefinition? skill) ? skill : null;
    }

    // Thu muc goi tai nguyen (vd "_design"), theo cung thu tu uu tien voi skill.
    public string? ResourceDirectory(string name)
    {
        return _snapshot.Resources.TryGetValue(name.Trim(), out string? directory) ? directory : null;
    }

    public void Reload()
    {
        var skills = new Dictionary<string, SkillDefinition>(StringComparer.Ordinal);
        var resources = new Dictionary<string, string>(StringComparer.Ordinal);
        var errors = new List<SkillError>();
        foreach (SkillSource source in _sources)
        {
            if (!Directory.Exists(source.Directory))
            {
                continue;
            }

            IEnumerable<string> directories;
            try
            {
                directories = Directory.GetDirectories(source.Directory).OrderBy(d => d, StringComparer.OrdinalIgnoreCase).ToList();
            }
            catch (Exception ex)
            {
                errors.Add(new SkillError(source.Directory, source.Name, "cannot list directory: " + ex.Message));
                continue;
            }

            foreach (string directory in directories)
            {
                string folder = Path.GetFileName(directory);
                if (folder.StartsWith('_'))
                {
                    resources[folder] = directory;
                    continue;
                }

                if (!File.Exists(Path.Combine(directory, SkillLoader.SkillFileName)))
                {
                    continue;
                }

                (SkillDefinition? skill, SkillError? error) = SkillLoader.Load(directory, source.Name);
                if (skill != null)
                {
                    skills[skill.Name] = skill;
                }
                else if (error != null)
                {
                    errors.Add(error);
                }
            }
        }

        lock (_gate)
        {
            _snapshot = new Snapshot(skills, resources, errors);
        }

        CoreLog.Info($"skills loaded: {skills.Count} ok, {errors.Count} error(s)"
            + (errors.Count > 0 ? " - " + string.Join("; ", errors.Select(e => Path.GetFileName(e.Directory) + ": " + e.Error)) : ""));
    }

    // Quet lai khi thu muc doi (debounce 2s).
    public void Watch()
    {
        foreach (SkillSource source in _sources)
        {
            if (!Directory.Exists(source.Directory))
            {
                continue;
            }

            try
            {
                var watcher = new FileSystemWatcher(source.Directory)
                {
                    IncludeSubdirectories = true,
                    NotifyFilter = NotifyFilters.FileName | NotifyFilters.DirectoryName | NotifyFilters.LastWrite | NotifyFilters.Size,
                };
                watcher.Changed += (_, _) => ScheduleReload();
                watcher.Created += (_, _) => ScheduleReload();
                watcher.Deleted += (_, _) => ScheduleReload();
                watcher.Renamed += (_, _) => ScheduleReload();
                watcher.EnableRaisingEvents = true;
                _watchers.Add(watcher);
            }
            catch (Exception ex)
            {
                CoreLog.Info("skills: cannot watch " + source.Directory + ": " + ex.Message);
            }
        }
    }

    private void ScheduleReload()
    {
        lock (_gate)
        {
            _debounce ??= new Timer(_ => Reload(), null, Timeout.Infinite, Timeout.Infinite);
            _debounce.Change(WatchDebounce, Timeout.InfiniteTimeSpan);
        }
    }

    public void Dispose()
    {
        foreach (FileSystemWatcher watcher in _watchers)
        {
            watcher.Dispose();
        }

        _debounce?.Dispose();
    }
}
