namespace WpsAiBridge.Bridge
{
    internal interface IAppHost
    {
        object Application { get; }
        string AppKind { get; }
        bool IsOfficeHost { get; }
    }
}
