using System;
using System.Diagnostics;
using System.Runtime.InteropServices;
using System.Windows.Forms;
using AxiomOffice.Ai;
using AxiomOffice.Bridge;
using AxiomOffice.Interop;
using AxiomOffice.Ribbon;

namespace AxiomOffice
{
    [ComVisible(true)]
    [Guid("BDB3732A-A479-4A24-AD64-D35952035BBA")]
    [ProgId("AxiomOffice.Connect")]
    [ClassInterface(ClassInterfaceType.AutoDispatch)]
    public class Connect : IDTExtensibility2, IRibbonExtensibility, ICustomTaskPaneConsumer, IAppHost
    {
        private object _application;
        private HttpBridge _bridge;
        private string _appKind;
        private object _ctpFactory;
        private object _taskPane;
        private Control _uiInvoker;

        public Connect()
        {
            Logger.Info("Connect constructor: instance created from " + typeof(Connect).Assembly.Location);
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

        public void CTPFactoryAvailable(object CTPFactoryInst)
        {
            _ctpFactory = CTPFactoryInst;
            Logger.Info("CTPFactoryAvailable: task pane factory ready");
        }

        // Chạy trên UI thread chính của host. ui.askpane đến từ thread HTTP: tạo CTP ở đó thì RCW của
        // pane thuộc apartment của thread HTTP, mọi lần gọi pane sau này từ UI thread phải vòng qua nó.
        public T OnUiThread<T>(Func<T> action)
        {
            Control invoker = _uiInvoker;
            if (invoker == null || !invoker.IsHandleCreated || !invoker.InvokeRequired)
            {
                return action();
            }
            return (T)invoker.Invoke(action);
        }

        public bool TryShowTaskPane()
        {
            return OnUiThread(ShowTaskPaneCore);
        }

        // Log độ rộng thực của CTP và nới ra nếu host bóp hẹp (WPS: bbs.wps.cn/topic/54548).
        // CTP Width có thể là point hoặc pixel tùy host nên quy đổi theo tỉ lệ đo được.
        public bool AdjustTaskPaneWidth(int controlWidthPx, int minWidthPx, int targetWidthPx)
        {
            if (_taskPane == null)
            {
                return true;
            }
            try
            {
                dynamic pane = _taskPane;
                int ctpWidth = Convert.ToInt32(pane.Width);
                Logger.Info("Task pane width: ctp=" + ctpWidth + " control=" + controlWidthPx + "px (min " + minWidthPx + "px)");
                if (controlWidthPx <= 0 || ctpWidth <= 0 || controlWidthPx >= minWidthPx)
                {
                    return true;
                }
                int wanted = (int)Math.Round(targetWidthPx * ((double)ctpWidth / controlWidthPx));
                pane.Width = wanted;
                Logger.Info("Task pane width set to " + wanted + ", read back " + Convert.ToInt32(pane.Width));
                return false;
            }
            catch (Exception ex)
            {
                Logger.Error("AdjustTaskPaneWidth failed", ex);
                return true;
            }
        }

        private bool ShowTaskPaneCore()
        {
            try
            {
                AskAiPane.CurrentHost = this;
                if (_taskPane == null && _ctpFactory != null)
                {
                    try
                    {
                        dynamic factory = _ctpFactory;
                        _taskPane = factory.CreateCTP("AxiomOffice.AskAiPane", "Axiom Office", Type.Missing);
                        Logger.Info("Task pane created via CTP factory");
                    }
                    catch (Exception ex)
                    {
                        Logger.Error("CreateCTP failed", ex);
                        _taskPane = null;
                    }
                }
                if (_taskPane != null)
                {
                    dynamic pane = _taskPane;
                    pane.Visible = true;
                    return true;
                }
            }
            catch (Exception ex)
            {
                Logger.Error("TryShowTaskPane failed", ex);
            }
            return false;
        }

        public void ShowAskAiPane()
        {
            if (TryShowTaskPane())
            {
                return;
            }
            Logger.Info("Task pane unavailable; opening floating dialog");
            using (var form = new Ai.AskAiHostForm(this))
            {
                form.ShowDialog();
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
                CreateUiInvoker();
                LogDocumentState("OnConnection");
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
                if (_uiInvoker != null)
                {
                    _uiInvoker.Dispose();
                    _uiInvoker = null;
                }
            }
            catch (Exception ex)
            {
                Logger.Error("OnDisconnection failed", ex);
            }
            finally
            {
                ReleaseComReferences();
            }
        }

        // Excel (và các host khác) không thoát khi add-in .NET còn giữ RCW: trước đây chỉ gán null nên RCW chờ
        // finalizer, EXCEL.EXE chạy ngầm mãi sau khi người dùng đóng (đo 30/09: không add-in thoát sau 2,4s,
        // có add-in còn chạy sau 20s). Nhả hẳn các RCW đang giữ rồi ép GC dọn RCW tạm (dynamic) còn sót.
        private void ReleaseComReferences()
        {
            object pane = _taskPane;
            object factory = _ctpFactory;
            object application = _application;
            _taskPane = null;
            _ctpFactory = null;
            _application = null;
            AskAiPane.CurrentHost = null;
            try
            {
                if (pane != null)
                {
                    try
                    {
                        ((dynamic)pane).Delete();
                    }
                    catch (Exception)
                    {
                    }
                }
                FinalRelease(pane);
                FinalRelease(factory);
                FinalRelease(application);
                GC.Collect();
                GC.WaitForPendingFinalizers();
                GC.Collect();
                GC.WaitForPendingFinalizers();
                Logger.Info("OnDisconnection: COM references released");
            }
            catch (Exception ex)
            {
                Logger.Error("ReleaseComReferences failed", ex);
            }
        }

        private static void FinalRelease(object value)
        {
            if (value != null && Marshal.IsComObject(value))
            {
                try
                {
                    Marshal.FinalReleaseComObject(value);
                }
                catch (Exception)
                {
                }
            }
        }

        public void OnAddInsUpdate(ref object custom)
        {
            Logger.Info("OnAddInsUpdate (host call)");
        }

        public void OnStartupComplete(ref object custom)
        {
            Logger.Info("OnStartupComplete (host call)");
            LogDocumentState("OnStartupComplete");
        }

        private void CreateUiInvoker()
        {
            try
            {
                _uiInvoker = new Control();
                IntPtr handle = _uiInvoker.Handle;
                Logger.Info("OnConnection: UI invoker ready on thread " + System.Threading.Thread.CurrentThread.ManagedThreadId);
            }
            catch (Exception ex)
            {
                Logger.Error("OnConnection: UI invoker failed", ex);
                _uiInvoker = null;
            }
        }

        private void LogDocumentState(string when)
        {
            if (_application == null)
            {
                return;
            }
            try
            {
                Logger.Info(when + ": " + HostProbe.Describe(this));
            }
            catch (Exception ex)
            {
                Logger.Error(when + ": document state unavailable", ex);
            }
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
