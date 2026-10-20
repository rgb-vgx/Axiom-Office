# Ham dung chung cho scripts/libreoffice.sh (chay tu repo) va install.sh (chay tu goi tarball - package.sh chep
# tep nay canh install.sh). Chi dinh nghia ham, khong chay gi khi source.

# LibreOffice cua nguoi dung nay dang chay?
soffice_running() {
    pgrep -u "$(id -u)" -f '[s]office\.bin' >/dev/null 2>&1
}

# Co Python UNO de LibreOffice nap extension (component Python)? Can loader Python cua CHINH LibreOffice
# (pythonloader.py canh soffice: goi python3-uno tren Debian/Ubuntu, co san trong ban TDF). Khong chi dung
# `python3 -c 'import uno'`: python3 dau PATH co the khong phai Python he thong (venv, pyenv, actions/setup-python)
# du LibreOffice van nap duoc extension. Tra 0 = co.
has_python_uno() {
    local soffice
    soffice="$(command -v soffice 2>/dev/null || true)"
    if [ -n "$soffice" ] && [ -f "$(dirname "$(readlink -f "$soffice")")/pythonloader.py" ]; then
        return 0
    fi
    python3 -c 'import uno' >/dev/null 2>&1 || /usr/bin/python3 -c 'import uno' >/dev/null 2>&1
}

PYTHON_UNO_HINT="sudo apt install python3-uno (Debian/Ubuntu) hoac sudo dnf install libreoffice-pyuno (Fedora)."
