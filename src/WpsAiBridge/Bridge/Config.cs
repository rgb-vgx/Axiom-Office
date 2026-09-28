using System;
using Microsoft.Win32;

namespace WpsAiBridge.Bridge
{
    internal static class Config
    {
        private const string KeyPath = @"Software\WpsAiBridge";
        public const int DefaultPort = 47821;

        public static int Port
        {
            get { return ReadInt("Port", DefaultPort); }
        }

        public static int PortForKind(string appKind)
        {
            int port = Port;
            if (appKind == "et")
            {
                return port + 1;
            }
            if (appKind == "wpp")
            {
                return port + 2;
            }
            return port;
        }

        public static string Token
        {
            get { return ReadString("Token", ""); }
        }

        public static bool Enabled
        {
            get { return ReadInt("Enabled", 1) != 0; }
        }

        public static string LlmProvider
        {
            get { return ReadString("LlmProvider", "openai"); }
        }

        public static string LlmEndpoint
        {
            get { return ReadString("LlmEndpoint", ""); }
        }

        public static string LlmApiKey
        {
            get { return ReadString("LlmApiKey", ""); }
        }

        public static string LlmModel
        {
            get { return ReadString("LlmModel", ""); }
        }

        public static void WriteString(string name, string value)
        {
            try
            {
                using (RegistryKey key = Registry.CurrentUser.CreateSubKey(KeyPath))
                {
                    if (key != null)
                    {
                        key.SetValue(name, value ?? "", RegistryValueKind.String);
                    }
                }
            }
            catch
            {
            }
        }

        private static int ReadInt(string name, int fallback)
        {
            try
            {
                using (RegistryKey key = Registry.CurrentUser.OpenSubKey(KeyPath))
                {
                    if (key != null)
                    {
                        object value = key.GetValue(name);
                        if (value is int)
                        {
                            return (int)value;
                        }
                        int parsed;
                        if (value != null && int.TryParse(Convert.ToString(value), out parsed))
                        {
                            return parsed;
                        }
                    }
                }
            }
            catch
            {
            }
            return fallback;
        }

        private static string ReadString(string name, string fallback)
        {
            try
            {
                using (RegistryKey key = Registry.CurrentUser.OpenSubKey(KeyPath))
                {
                    if (key != null)
                    {
                        object value = key.GetValue(name);
                        if (value != null)
                        {
                            return Convert.ToString(value);
                        }
                    }
                }
            }
            catch
            {
            }
            return fallback;
        }
    }
}
