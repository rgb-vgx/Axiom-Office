.PHONY: help test test-lo test-wps test-core check-gen package clean

help:
	@echo "Danh sach dich:"
	@echo "  help      - Hien thi danh sach dich kem mo ta"
	@echo "  test      - Chay toan bo kiem thu (test-lo, test-wps, test-core)"
	@echo "  test-lo   - Kiem thu extension LibreOffice"
	@echo "  test-wps  - Kiem thu add-in WPS"
	@echo "  test-core - Build Agent Core Go va kiem thu e2e"
	@echo "  check-gen - Kiem tra file sinh khong lech nguon"
	@echo "  package   - Dong goi tarball Linux"
	@echo "  clean     - Xoa thu muc dist/"

test: test-lo test-wps test-core

test-lo:
	python3 -m unittest discover -s tests/lo -v

test-wps:
	python3 tests/wps/test_install_jsaddon.py
	node --test tests/wps/test_addin_fake.mjs tests/wps/test_addin.mjs tests/wps/test_addin_et.mjs tests/wps/test_addin_wpp.mjs tests/wps/test_probe.mjs tests/wps/test_pane.mjs tests/wps/test_pane_theme.mjs

test-core:
	mkdir -p dist/core
	cd core-go && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o ../dist/core/axiom-core ./cmd/axiom-core
	rm -rf dist/core/skills
	cp -r skills dist/core/skills
	AXIOM_E2E_CORE_EXE=$(CURDIR)/dist/core/axiom-core python3 tests/core/test_core_e2e.py --only fake_bridge,guards,skills,confirm,memory,mcp,visual,setup,shutdown,anthropic,embeddings,summarize,cancel,mcp_http,verify

check-gen:
	python3 scripts/generate_setup_catalog.py --check
	python3 scripts/generate_mcp_commands.py --check

package:
	bash scripts/linux/package.sh

clean:
	rm -rf dist/
