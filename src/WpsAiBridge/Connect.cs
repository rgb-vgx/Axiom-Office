using System;
using System.Diagnostics;
using System.Runtime.InteropServices;
using WpsAiBridge.Bridge;
using WpsAiBridge.Interop;
using WpsAiBridge.Ribbon;

namespace WpsAiBridge
{
    [ComVisible(true)]
    [Guid("F4524DFD-C4F6-4027-8CA6-08B7F7DB4C44")]
    [ProgId("WpsAiBridge.Connect")]
    [ClassInterface(ClassInterfaceType.AutoDispatch)]
    public class Connect : IDTExtensibility2, IRibbonExtensibility, IAppHost
    {
        private object _application;
        private HttpBridge _bridge;
        private string _appKind;

        public Connect()
        {
            Logger.Info("Connect constructor: instance created");
        }

        public string GetCustomUI(string RibbonID)
        {
            Logger.Info("GetCustomUI called: " + RibbonID);
            return RibbonUi.RibbonXml;
        }

        public void OnButtonAction(object control)
        {
            string tag = "";
            try
            {
                dynamic c = control;
                tag = Convert.ToString(c.Tag);
            }
            catch (Exception ex)
            {
                Logger.Error("OnButtonAction: cannot read Tag", ex);
            }
            Logger.Info("OnButtonAction: tag=" + tag);
            try
            {
                RibbonUi.HandleButton(this, tag);
            }
            catch (Exception ex)
            {
                Logger.Error("OnButtonAction failed", ex);
            }
        }

        public object Application
        {
            get { return _application; }
        }

        public string AppKind
        {
            get
            {
                if (_appKind == null)
                {
                    _appKind = DetectAppKind(_application);
                    Logger.Info("AppKind detected: " + _appKind);
                }
                return _appKind;
            }
        }

        public bool IsOfficeHost
        {
            get { return DetectOfficeHost(); }
        }

        public void OnConnection(object Application, ext_ConnectMode ConnectMode, object AddInInst, ref Array custom)
        {
            Logger.Info("OnConnection entered: mode=" + ConnectMode + " host=" + HostProcessName());
            try
            {
                _application = Application;
                Logger.Info("OnConnection: application reference stored");
                _bridge = new HttpBridge(this);
                _bridge.Start();
                Logger.Info("OnConnection: bridge started, done");
            }
            catch (Exception ex)
            {
                Logger.Error("OnConnection failed", ex);
            }
            finally
            {
                if (custom == null)
                {
                    custom = new object[0];
                }
            }
        }

        public void OnDisconnection(ext_DisconnectMode RemoveMode, ref object custom)
        {
            Logger.Info("OnDisconnection (host call): mode=" + RemoveMode);
            try
            {
                if (_bridge != null)
                {
                    _bridge.Stop();
                    _bridge = null;
                }
            }
            catch (Exception ex)
            {
                Logger.Error("OnDisconnection failed", ex);
            }
            finally
            {
                _application = null;
            }
        }

        public void OnAddInsUpdate(ref object custom)
        {
            Logger.Info("OnAddInsUpdate (host call)");
        }

        public void OnStartupComplete(ref object custom)
        {
            Logger.Info("OnStartupComplete (host call)");
        }

        public void OnBeginShutdown(ref object custom)
        {
            Logger.Info("OnBeginShutdown (host call)");
        }

        private static string HostProcessName()
        {
            try
            {
                return System.Diagnostics.Process.GetCurrentProcess().MainModule.ModuleName;
            }
            catch
            {
                return "unknown";
            }
        }

        private static bool DetectOfficeHost()
        {
            string name = HostProcessName().ToLowerInvariant();
            return name.StartsWith("winword") || name.StartsWith("excel") || name.StartsWith("powerpnt");
        }

        private static string DetectAppKind(object application)
        {
            if (application != null)
            {
                try
                {
                    dynamic app = application;
                    try
                    {
                        object probe = app.Documents;
                        if (probe != null)
                        {
                            return "wps";
                        }
                    }
                    catch
                    {
                    }
                    try
                    {
                        object probe = app.Workbooks;
                        if (probe != null)
                        {
                            return "et";
                        }
                    }
                    catch
                    {
                    }
                    try
                    {
                        object probe = app.Presentations;
                        if (probe != null)
                        {
                            return "wpp";
                        }
                    }
                    catch
                    {
                    }
                }
                catch (Exception ex)
                {
                    Logger.Error("DetectAppKind COM probe failed", ex);
                }
            }

            string name = HostProcessName().ToLowerInvariant();
            if (name.StartsWith("wps"))
            {
                return "wps";
            }
            if (name.StartsWith("et"))
            {
                return "et";
            }
            if (name.StartsWith("wpp"))
            {
                return "wpp";
            }
            return "unknown";
        }
    }
}
