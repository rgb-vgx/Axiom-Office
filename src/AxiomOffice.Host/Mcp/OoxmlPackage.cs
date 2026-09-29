using System;
using System.Collections.Generic;
using System.IO;
using System.IO.Compression;
using System.Linq;
using System.Text;
using System.Xml;
using System.Xml.Linq;

namespace AxiomOffice.Host.Mcp
{
    // Gói OOXML (docx/xlsx/pptx) đọc/ghi thẳng qua ZipArchive + XLinq, không cần thư viện ngoài.
    // Chỉ các part được sửa mới được ghi lại, nên chart/ảnh/pivot/macro trong file được giữ nguyên.
    // Tên part dùng dạng "xl/workbook.xml" (không có "/" đầu).
    internal sealed class OoxmlPackage : IDisposable
    {
        public static readonly XNamespace RelNs = "http://schemas.openxmlformats.org/package/2006/relationships";
        public static readonly XNamespace CtNs = "http://schemas.openxmlformats.org/package/2006/content-types";
        public static readonly XNamespace OfficeRelNs = "http://schemas.openxmlformats.org/officeDocument/2006/relationships";
        public const string RelTypeBase = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/";

        private readonly ZipArchive _zip;
        private readonly Stream _stream;
        private readonly Dictionary<string, ZipArchiveEntry> _entries = new Dictionary<string, ZipArchiveEntry>(StringComparer.OrdinalIgnoreCase);
        private readonly Dictionary<string, XDocument> _xml = new Dictionary<string, XDocument>(StringComparer.OrdinalIgnoreCase);
        private readonly HashSet<string> _dirty = new HashSet<string>(StringComparer.OrdinalIgnoreCase);
        private readonly bool _writable;

        private OoxmlPackage(Stream stream, bool writable)
        {
            _stream = stream;
            _writable = writable;
            _zip = new ZipArchive(stream, writable ? ZipArchiveMode.Update : ZipArchiveMode.Read, false, Encoding.UTF8);
            foreach (ZipArchiveEntry entry in _zip.Entries)
            {
                _entries[Normalize(entry.FullName)] = entry;
            }
        }

        public static OoxmlPackage OpenRead(string path)
        {
            var stream = new FileStream(path, FileMode.Open, FileAccess.Read, FileShare.ReadWrite | FileShare.Delete);
            try
            {
                return new OoxmlPackage(stream, false);
            }
            catch (InvalidDataException)
            {
                stream.Dispose();
                throw new InvalidDataException("'" + path + "' is not a valid Office Open XML file (zip)");
            }
        }

        private static OoxmlPackage OpenUpdate(string path)
        {
            return new OoxmlPackage(new FileStream(path, FileMode.Open, FileAccess.ReadWrite, FileShare.None), true);
        }

        public IEnumerable<string> PartNames
        {
            get { return _entries.Keys.ToList(); }
        }

        public bool Exists(string part)
        {
            part = Normalize(part);
            return _xml.ContainsKey(part) || _entries.ContainsKey(part);
        }

        public XDocument Xml(string part)
        {
            part = Normalize(part);
            XDocument doc;
            if (_xml.TryGetValue(part, out doc))
            {
                return doc;
            }
            ZipArchiveEntry entry;
            if (!_entries.TryGetValue(part, out entry))
            {
                throw new FileNotFoundException("part not found in package: " + part);
            }
            using (Stream stream = entry.Open())
            {
                doc = XDocument.Load(stream, LoadOptions.PreserveWhitespace);
            }
            _xml[part] = doc;
            return doc;
        }

        public XDocument XmlOrNull(string part)
        {
            return Exists(part) ? Xml(part) : null;
        }

        // Đánh dấu part XML đã sửa (hoặc tạo mới) để ghi lại khi Save().
        public void Put(string part, XDocument doc)
        {
            part = Normalize(part);
            _xml[part] = doc;
            _dirty.Add(part);
        }

        public void Touch(string part)
        {
            Put(part, Xml(part));
        }

        // Stream đọc part chưa sửa (đọc streaming sheet lớn mà không dựng XDocument).
        public Stream OpenStream(string part)
        {
            ZipArchiveEntry entry;
            if (!_entries.TryGetValue(Normalize(part), out entry))
            {
                throw new FileNotFoundException("part not found in package: " + part);
            }
            return entry.Open();
        }

        public byte[] Bytes(string part)
        {
            ZipArchiveEntry entry;
            if (!_entries.TryGetValue(Normalize(part), out entry))
            {
                throw new FileNotFoundException("part not found in package: " + part);
            }
            using (Stream stream = entry.Open())
            using (var memory = new MemoryStream())
            {
                stream.CopyTo(memory);
                return memory.ToArray();
            }
        }

        public void Delete(string part)
        {
            part = Normalize(part);
            _xml.Remove(part);
            _dirty.Remove(part);
            ZipArchiveEntry entry;
            if (_entries.TryGetValue(part, out entry))
            {
                entry.Delete();
                _entries.Remove(part);
            }
            string rels = RelsPartFor(part);
            if (!string.Equals(rels, part, StringComparison.OrdinalIgnoreCase) && Exists(rels))
            {
                Delete(rels);
            }
            RemoveContentTypeOverride(part);
        }

        // ---- relationships ----

        public static string RelsPartFor(string part)
        {
            part = Normalize(part);
            int slash = part.LastIndexOf('/');
            string dir = slash >= 0 ? part.Substring(0, slash + 1) : "";
            string name = slash >= 0 ? part.Substring(slash + 1) : part;
            return dir + "_rels/" + name + ".rels";
        }

        // Relationships của part ("" = package gốc: _rels/.rels). Target đã quy về tên part tuyệt đối.
        public List<Rel> Rels(string sourcePart)
        {
            var result = new List<Rel>();
            string relsPart = sourcePart.Length == 0 ? "_rels/.rels" : RelsPartFor(sourcePart);
            XDocument doc = XmlOrNull(relsPart);
            if (doc == null)
            {
                return result;
            }
            foreach (XElement rel in doc.Root.Elements(RelNs + "Relationship"))
            {
                string mode = (string)rel.Attribute("TargetMode");
                bool external = string.Equals(mode, "External", StringComparison.OrdinalIgnoreCase);
                string target = (string)rel.Attribute("Target") ?? "";
                result.Add(new Rel
                {
                    Id = (string)rel.Attribute("Id"),
                    Type = (string)rel.Attribute("Type") ?? "",
                    Target = target,
                    External = external,
                    TargetPart = external ? null : ResolveTarget(sourcePart, target)
                });
            }
            return result;
        }

        public Rel RelOfType(string sourcePart, string typeSuffix)
        {
            return Rels(sourcePart).FirstOrDefault(r => r.Type.EndsWith("/" + typeSuffix, StringComparison.Ordinal));
        }

        public string AddRel(string sourcePart, string type, string targetPart)
        {
            string relsPart = sourcePart.Length == 0 ? "_rels/.rels" : RelsPartFor(sourcePart);
            XDocument doc = XmlOrNull(relsPart) ?? new XDocument(new XDeclaration("1.0", "UTF-8", "yes"), new XElement(RelNs + "Relationships"));
            var used = new HashSet<string>(doc.Root.Elements(RelNs + "Relationship").Select(e => (string)e.Attribute("Id")), StringComparer.Ordinal);
            int n = used.Count + 1;
            while (used.Contains("rId" + n))
            {
                n++;
            }
            string id = "rId" + n;
            doc.Root.Add(new XElement(RelNs + "Relationship",
                new XAttribute("Id", id),
                new XAttribute("Type", type.StartsWith("http", StringComparison.Ordinal) ? type : RelTypeBase + type),
                new XAttribute("Target", RelativeTarget(sourcePart, targetPart))));
            Put(relsPart, doc);
            return id;
        }

        public void RemoveRel(string sourcePart, string id)
        {
            string relsPart = sourcePart.Length == 0 ? "_rels/.rels" : RelsPartFor(sourcePart);
            XDocument doc = XmlOrNull(relsPart);
            if (doc == null)
            {
                return;
            }
            foreach (XElement rel in doc.Root.Elements(RelNs + "Relationship").Where(e => (string)e.Attribute("Id") == id).ToList())
            {
                rel.Remove();
            }
            Put(relsPart, doc);
        }

        public static string ResolveTarget(string sourcePart, string target)
        {
            if (target.StartsWith("/", StringComparison.Ordinal))
            {
                return Normalize(target);
            }
            int slash = Normalize(sourcePart).LastIndexOf('/');
            string baseDir = slash >= 0 ? sourcePart.Substring(0, slash) : "";
            var segments = new List<string>(baseDir.Length == 0 ? new string[0] : baseDir.Split('/'));
            foreach (string segment in target.Replace('\\', '/').Split('/'))
            {
                if (segment == "..")
                {
                    if (segments.Count > 0)
                    {
                        segments.RemoveAt(segments.Count - 1);
                    }
                }
                else if (segment.Length > 0 && segment != ".")
                {
                    segments.Add(segment);
                }
            }
            return string.Join("/", segments);
        }

        public static string RelativeTarget(string sourcePart, string targetPart)
        {
            string[] from = sourcePart.Length == 0 ? new string[0] : Normalize(sourcePart).Split('/');
            string[] to = Normalize(targetPart).Split('/');
            int common = 0;
            while (common < from.Length - 1 && common < to.Length - 1 && string.Equals(from[common], to[common], StringComparison.OrdinalIgnoreCase))
            {
                common++;
            }
            var parts = new List<string>();
            for (int i = common; i < from.Length - 1; i++)
            {
                parts.Add("..");
            }
            for (int i = common; i < to.Length; i++)
            {
                parts.Add(to[i]);
            }
            return string.Join("/", parts);
        }

        // ---- content types ----

        public void SetContentTypeOverride(string part, string contentType)
        {
            XDocument doc = Xml("[Content_Types].xml");
            string name = "/" + Normalize(part);
            XElement existing = doc.Root.Elements(CtNs + "Override")
                .FirstOrDefault(e => string.Equals((string)e.Attribute("PartName"), name, StringComparison.OrdinalIgnoreCase));
            if (existing != null)
            {
                existing.SetAttributeValue("ContentType", contentType);
            }
            else
            {
                doc.Root.Add(new XElement(CtNs + "Override", new XAttribute("PartName", name), new XAttribute("ContentType", contentType)));
            }
            Put("[Content_Types].xml", doc);
        }

        public void RemoveContentTypeOverride(string part)
        {
            if (!Exists("[Content_Types].xml"))
            {
                return;
            }
            XDocument doc = Xml("[Content_Types].xml");
            string name = "/" + Normalize(part);
            var matches = doc.Root.Elements(CtNs + "Override")
                .Where(e => string.Equals((string)e.Attribute("PartName"), name, StringComparison.OrdinalIgnoreCase)).ToList();
            if (matches.Count > 0)
            {
                matches.ForEach(e => e.Remove());
                Put("[Content_Types].xml", doc);
            }
        }

        public string ContentTypeOf(string part)
        {
            XDocument doc = Xml("[Content_Types].xml");
            string name = "/" + Normalize(part);
            XElement over = doc.Root.Elements(CtNs + "Override")
                .FirstOrDefault(e => string.Equals((string)e.Attribute("PartName"), name, StringComparison.OrdinalIgnoreCase));
            if (over != null)
            {
                return (string)over.Attribute("ContentType");
            }
            string ext = Path.GetExtension(part).TrimStart('.');
            XElement def = doc.Root.Elements(CtNs + "Default")
                .FirstOrDefault(e => string.Equals((string)e.Attribute("Extension"), ext, StringComparison.OrdinalIgnoreCase));
            return def == null ? null : (string)def.Attribute("ContentType");
        }

        // Tên part chưa dùng theo mẫu, vd "xl/worksheets/sheet{0}.xml".
        public string NextPartName(string pattern)
        {
            for (int n = 1; ; n++)
            {
                string candidate = string.Format(pattern, n);
                if (!Exists(candidate))
                {
                    return candidate;
                }
            }
        }

        public void Save()
        {
            if (!_writable)
            {
                throw new InvalidOperationException("package opened read-only");
            }
            var settings = new XmlWriterSettings { Encoding = new UTF8Encoding(false), Indent = false, NewLineHandling = NewLineHandling.None };
            foreach (string part in _dirty.ToList())
            {
                XDocument doc = _xml[part];
                if (doc.Declaration == null)
                {
                    doc.Declaration = new XDeclaration("1.0", "UTF-8", "yes");
                }
                ZipArchiveEntry old;
                if (_entries.TryGetValue(part, out old))
                {
                    old.Delete();
                }
                ZipArchiveEntry entry = _zip.CreateEntry(part, CompressionLevel.Optimal);
                using (Stream stream = entry.Open())
                using (XmlWriter writer = XmlWriter.Create(stream, settings))
                {
                    doc.Save(writer);
                }
                _entries[part] = entry;
            }
            _dirty.Clear();
        }

        public void Dispose()
        {
            _zip.Dispose();
            _stream.Dispose();
        }

        private static string Normalize(string part)
        {
            return part.Replace('\\', '/').TrimStart('/');
        }

        // ---- lưu atomic: sửa trên bản sao tạm rồi thay file gốc ----

        public static void Edit(string path, Action<OoxmlPackage> edit)
        {
            if (!File.Exists(path))
            {
                throw new FileNotFoundException(path);
            }
            string temp = FileSafety.TempPathFor(path);
            try
            {
                File.Copy(path, temp, true);
                File.SetAttributes(temp, FileAttributes.Normal);
                using (OoxmlPackage package = OpenUpdate(temp))
                {
                    edit(package);
                    package.Save();
                }
            }
            catch (Exception)
            {
                FileSafety.DeleteQuietly(temp);
                throw;
            }
            FileSafety.Replace(temp, path);
        }

        public static void CreateFromTemplate(string path, byte[] template, Action<OoxmlPackage> edit)
        {
            string temp = FileSafety.TempPathFor(path);
            try
            {
                File.WriteAllBytes(temp, template);
                using (OoxmlPackage package = OpenUpdate(temp))
                {
                    edit(package);
                    package.Save();
                }
            }
            catch (Exception)
            {
                FileSafety.DeleteQuietly(temp);
                throw;
            }
            FileSafety.Replace(temp, path);
        }

        public static byte[] EmptyZip()
        {
            using (var memory = new MemoryStream())
            {
                using (new ZipArchive(memory, ZipArchiveMode.Create, true))
                {
                }
                return memory.ToArray();
            }
        }
    }

    internal sealed class Rel
    {
        public string Id;
        public string Type;
        public string Target;
        public string TargetPart;
        public bool External;
    }

    internal static class FileSafety
    {
        public static string TempPathFor(string path)
        {
            string full = Path.GetFullPath(path);
            string directory = Path.GetDirectoryName(full);
            if (!Directory.Exists(directory))
            {
                throw new DirectoryNotFoundException("folder does not exist: " + directory);
            }
            return Path.Combine(directory, ".~" + Path.GetFileName(full) + "." + Guid.NewGuid().ToString("N").Substring(0, 8) + ".tmp");
        }

        public static void Replace(string temp, string path)
        {
            try
            {
                if (File.Exists(path))
                {
                    File.Replace(temp, path, null, true);
                }
                else
                {
                    File.Move(temp, path);
                }
            }
            catch (Exception ex)
            {
                DeleteQuietly(temp);
                if (ex is IOException || ex is UnauthorizedAccessException)
                {
                    throw new IOException("cannot write '" + path + "': file is locked (open in Word/Excel/PowerPoint/WPS?) - close it first or use the live bridge tools", ex);
                }
                throw;
            }
        }

        public static void DeleteQuietly(string path)
        {
            try
            {
                if (File.Exists(path))
                {
                    File.Delete(path);
                }
            }
            catch (Exception)
            {
            }
        }

        public static void RequireFile(string path)
        {
            if (string.IsNullOrEmpty(path))
            {
                throw new ArgumentException("path is required");
            }
            if (!File.Exists(path))
            {
                throw new FileNotFoundException("file not found: " + path);
            }
        }

        public static void RequireNewOrOverwrite(string path, bool overwrite)
        {
            if (File.Exists(path) && !overwrite)
            {
                throw new InvalidOperationException("file already exists - pass overwrite=true to replace it");
            }
        }
    }
}
