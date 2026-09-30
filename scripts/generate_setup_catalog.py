"""Sinh catalog cho wizard thiet lap tu `catalog/setup.json` (nguon duy nhat ve chu + preset).

Ra ba file, KHONG sua tay:
  - `src/AxiomOffice/Setup/SetupCatalog.cs`   - add-in net48 (build.ps1 bien dich ca thu muc src\\AxiomOffice)
                                                va Agent Core (csproj Compile Include) dung chung.
                                                Phai la cu phap C# 7.3 (add-in build bang /langversion:7.3).
  - `src/AxiomOffice.LibreOffice/python/pythonpath/axiom/setup_catalog.py` - extension LibreOffice
                                                (nam trong pythonpath nen scripts/package_oxt.py dong goi luon).
  - `core-go/internal/setup/catalog_gen.go`  - Agent Core ban Go (JSON nhung san, giai ma luc khoi dong).

    python scripts/generate_setup_catalog.py [--check]
"""
from __future__ import annotations

import argparse
import io
import json
import os
import pprint
import sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
SOURCE = os.path.join(ROOT, "catalog", "setup.json")
CSHARP = os.path.join(ROOT, "src", "AxiomOffice", "Setup", "SetupCatalog.cs")
PYTHON = os.path.join(ROOT, "src", "AxiomOffice.LibreOffice", "python", "pythonpath", "axiom", "setup_catalog.py")
GO = os.path.join(ROOT, "core-go", "internal", "setup", "catalog_gen.go")


def load() -> dict:
    with io.open(SOURCE, encoding="utf-8") as handle:
        return json.load(handle)


def cs_string(value: str) -> str:
    return '"' + (value or "").replace("\\", "\\\\").replace('"', '\\"') + '"'


def cs_array(values) -> str:
    return "new string[] { " + ", ".join(cs_string(v) for v in values) + " }"


def render_csharp(data: dict) -> str:
    lines = [
        "// SINH TU DONG tu catalog/setup.json bang scripts/generate_setup_catalog.py - KHONG sua tay.",
        "// Chu cua wizard + preset nha cung cap cho add-in Windows (net48).",
        "using System;",
        "using System.Collections.Generic;",
        "",
        "namespace AxiomOffice.Setup",
        "{",
        "    internal sealed class SetupProviderInfo",
        "    {",
        "        public readonly string Id;",
        "        public readonly string Group;",
        "        public readonly string Label;",
        "        public readonly string Description;",
        "        public readonly string Endpoint;",
        "        public readonly bool NeedsKey;",
        "        public readonly string KeyUrl;",
        "        public readonly string Codec;",
        "        public readonly string[] SuggestedModels;",
        "",
        "        public SetupProviderInfo(string id, string group, string label, string description, string endpoint,",
        "            bool needsKey, string keyUrl, string codec, string[] suggestedModels)",
        "        {",
        "            Id = id;",
        "            Group = group;",
        "            Label = label;",
        "            Description = description;",
        "            Endpoint = endpoint;",
        "            NeedsKey = needsKey;",
        "            KeyUrl = keyUrl;",
        "            Codec = codec;",
        "            SuggestedModels = suggestedModels;",
        "        }",
        "    }",
        "",
        "    internal sealed class SetupFeatureInfo",
        "    {",
        "        public readonly string Key;",
        "        public readonly string Label;",
        "        public readonly string Description;",
        "        public readonly bool Recommended;",
        "",
        "        public SetupFeatureInfo(string key, string label, string description, bool recommended)",
        "        {",
        "            Key = key;",
        "            Label = label;",
        "            Description = description;",
        "            Recommended = recommended;",
        "        }",
        "    }",
        "",
        "    internal sealed class SetupCheckInfo",
        "    {",
        "        public readonly string Id;",
        "        public readonly string Label;",
        "        public readonly bool Fixable;",
        "        public readonly string FixLabel;",
        "        public readonly string Help;",
        "",
        "        public SetupCheckInfo(string id, string label, bool fixable, string fixLabel, string help)",
        "        {",
        "            Id = id;",
        "            Label = label;",
        "            Fixable = fixable;",
        "            FixLabel = fixLabel;",
        "            Help = help;",
        "        }",
        "    }",
        "",
        "    internal sealed class SetupStepInfo",
        "    {",
        "        public readonly string Id;",
        "        public readonly string Title;",
        "        public readonly string Subtitle;",
        "",
        "        public SetupStepInfo(string id, string title, string subtitle)",
        "        {",
        "            Id = id;",
        "            Title = title;",
        "            Subtitle = subtitle;",
        "        }",
        "    }",
        "",
        "    internal static class SetupCatalog",
        "    {",
        "        public const int Version = %d;" % data["version"],
        "",
        "        public static readonly IReadOnlyList<SetupStepInfo> Steps = new[]",
        "        {",
    ]
    for step in data["steps"]:
        lines.append("            new SetupStepInfo(%s, %s, %s)," % (cs_string(step["id"]), cs_string(step["title"]),
                                                                   cs_string(step["subtitle"])))
    lines += ["        };", "", "        public static readonly IReadOnlyList<SetupProviderInfo> Providers = new[]", "        {"]
    for provider in data["providers"]:
        lines.append("            new SetupProviderInfo(%s, %s, %s, %s, %s, %s, %s, %s, %s)," % (
            cs_string(provider["id"]), cs_string(provider["group"]), cs_string(provider["label"]),
            cs_string(provider["description"]), cs_string(provider["endpoint"]),
            "true" if provider["needsKey"] else "false", cs_string(provider["keyUrl"]),
            cs_string(provider["codec"]), cs_array(provider["suggestedModels"])))
    lines += ["        };", "", "        public static readonly IReadOnlyList<SetupFeatureInfo> Features = new[]", "        {"]
    for feature in data["features"]:
        lines.append("            new SetupFeatureInfo(%s, %s, %s, %s)," % (
            cs_string(feature["key"]), cs_string(feature["label"]), cs_string(feature["description"]),
            "true" if feature["recommended"] else "false"))
    lines += ["        };", "", "        public static readonly IReadOnlyList<SetupCheckInfo> Checks = new[]", "        {"]
    for check in data["checks"]:
        lines.append("            new SetupCheckInfo(%s, %s, %s, %s, %s)," % (
            cs_string(check["id"]), cs_string(check["label"]), "true" if check["fixable"] else "false",
            cs_string(check["fixLabel"]), cs_string(check["help"])))
    lines += [
        "        };",
        "",
        "        public static SetupProviderInfo FindProvider(string id)",
        "        {",
        "            if (string.IsNullOrWhiteSpace(id))",
        "            {",
        "                return null;",
        "            }",
        "",
        "            string wanted = id.Trim();",
        "            foreach (SetupProviderInfo provider in Providers)",
        "            {",
        "                if (string.Equals(provider.Id, wanted, StringComparison.OrdinalIgnoreCase))",
        "                {",
        "                    return provider;",
        "                }",
        "            }",
        "",
        "            return null;",
        "        }",
        "",
        "        public static SetupCheckInfo FindCheck(string id)",
        "        {",
        "            foreach (SetupCheckInfo check in Checks)",
        "            {",
        "                if (string.Equals(check.Id, id, StringComparison.OrdinalIgnoreCase))",
        "                {",
        "                    return check;",
        "                }",
        "            }",
        "",
        "            return null;",
        "        }",
        "",
        "        // Doan nha cung cap tu dia chi dang cau hinh (mo lai wizard thi chon dung lua chon cu).",
        "        public static string GuessProviderId(string endpoint, string codec)",
        "        {",
        "            string value = (endpoint ?? string.Empty).Trim();",
        "            if (value.Length == 0)",
        "            {",
        "                return \"company\";",
        "            }",
        "",
        "            if (value.IndexOf(\"api.openai.com\", StringComparison.OrdinalIgnoreCase) >= 0)",
        "            {",
        "                return \"openai\";",
        "            }",
        "",
        "            if (value.IndexOf(\"anthropic.com\", StringComparison.OrdinalIgnoreCase) >= 0)",
        "            {",
        "                return \"anthropic\";",
        "            }",
        "",
        "            if (value.IndexOf(\"generativelanguage.googleapis.com\", StringComparison.OrdinalIgnoreCase) >= 0)",
        "            {",
        "                return \"gemini\";",
        "            }",
        "",
        "            return \"company\";",
        "        }",
        "    }",
        "}",
        "",
    ]
    return "\n".join(lines)


def render_python(data: dict) -> str:
    # pprint sinh literal Python hop le (True/False/None) va giu nguyen thu tu khoa; json.dumps thi khong.
    body = pprint.pformat(data, width=118, sort_dicts=False)
    return (
        '"""SINH TU DONG tu catalog/setup.json bang scripts/generate_setup_catalog.py - KHONG sua tay.\n\n'
        "Chu cua wizard thiet lap + preset nha cung cap AI, dung chung voi ban Windows (SetupCatalog.cs).\n"
        '"""\n'
        "from __future__ import annotations\n\n"
        "CATALOG = " + body + "\n\n"
        "STEPS = CATALOG[\"steps\"]\n"
        "PROVIDERS = CATALOG[\"providers\"]\n"
        "FEATURES = CATALOG[\"features\"]\n"
        "CHECKS = CATALOG[\"checks\"]\n\n\n"
        "def provider(provider_id: str):\n"
        '    """Preset theo id (None neu khong co)."""\n'
        "    for item in PROVIDERS:\n"
        "        if item[\"id\"] == provider_id:\n"
        "            return item\n"
        "    return None\n\n\n"
        "def check(check_id: str):\n"
        "    for item in CHECKS:\n"
        "        if item[\"id\"] == check_id:\n"
        "            return item\n"
        "    return None\n\n\n"
        "def guess_provider_id(endpoint: str, codec: str = \"openai\") -> str:\n"
        '    """Doan nha cung cap tu dia chi dang cau hinh (mo lai wizard thi chon dung lua chon cu)."""\n'
        "    value = (endpoint or \"\").strip()\n"
        "    if not value:\n"
        "        return \"company\"\n"
        "    lowered = value.lower()\n"
        "    if \"api.openai.com\" in lowered:\n"
        "        return \"openai\"\n"
        "    if \"anthropic.com\" in lowered:\n"
        "        return \"anthropic\"\n"
        "    if \"generativelanguage.googleapis.com\" in lowered:\n"
        "        return \"gemini\"\n"
        "    return \"company\"\n"
    )


def render_go(data: dict) -> str:
    # Chuoi do json.dumps sinh ra cung la literal chuoi Go hop le (cac escape \" \\ \n \uXXXX giong nhau).
    payload = json.dumps(json.dumps(data, ensure_ascii=False, separators=(",", ":")), ensure_ascii=False)
    return (
        "// SINH TU DONG tu catalog/setup.json bang scripts/generate_setup_catalog.py - KHONG sua tay.\n\n"
        "package setup\n\n"
        "const catalogJSON = " + payload + "\n"
    )


def write(path: str, text: str, check: bool) -> bool:
    current = ""
    if os.path.exists(path):
        with io.open(path, encoding="utf-8", newline="") as handle:
            current = handle.read().replace("\r\n", "\n")
    if current == text:
        return False
    if check:
        print("LECH: %s (chay lai scripts/generate_setup_catalog.py)" % os.path.relpath(path, ROOT), file=sys.stderr)
        return True
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with io.open(path, "w", encoding="utf-8", newline="\n") as handle:
        handle.write(text)
    print("da ghi %s" % os.path.relpath(path, ROOT))
    return True


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true", help="chi kiem tra hai file sinh ra co khop catalog khong")
    args = parser.parse_args()
    data = load()
    stale = write(CSHARP, render_csharp(data), args.check)
    stale = write(PYTHON, render_python(data), args.check) or stale
    stale = write(GO, render_go(data), args.check) or stale
    if args.check and stale:
        return 1
    if args.check:
        print("khop: SetupCatalog.cs + setup_catalog.py + catalog_gen.go")
    return 0


if __name__ == "__main__":
    sys.exit(main())
