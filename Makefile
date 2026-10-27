.PHONY: help test test-go test-lo test-bench test-catalog test-wps test-wps-e2e test-core test-mcp test-oracle test-install test-live-format core-build check-gen package clean fmt lint lint-go lint-py lint-js

GOBIN := $(shell go env GOPATH)/bin
RUFF ?= $(shell command -v ruff 2>/dev/null || echo python3 -m ruff)
STATICCHECK ?= $(shell command -v staticcheck 2>/dev/null || echo $(GOBIN)/staticcheck)
ESLINT ?= $(shell command -v eslint 2>/dev/null || echo npx --no-install eslint)

help:
	@echo "Danh sach dich:"
	@echo "  help        - Hien thi danh sach dich kem mo ta"
	@echo "  test        - Chay toan bo kiem thu khong can app that"
	@echo "  test-go     - Kiem thu Go (cd core-go && go test ./...)"
	@echo "  test-lo     - Kiem thu extension LibreOffice"
	@echo "  test-wps    - Kiem thu add-in WPS"
	@echo "  test-wps-e2e- E2e kenh WPS + pane tren Core that (khong can WPS)"
	@echo "  test-live-format - Hoi quy dinh dang Word that (can Office x64 + add-in)"
	@echo "  test-core   - Build Agent Core Go va kiem thu e2e"
	@echo "  test-mcp    - Kiem thu MCP portable tren binary vua build"
	@echo "  test-oracle - Kiem thu ban doi chieu (oracle) tests/oracle/*-mcp"
	@echo "  test-install- Kiem thu bo cai dat Linux (install.sh)"
	@echo "  core-build  - Build binary Agent Core Go vao dist/core/"
	@echo "  check-gen   - Kiem tra file sinh khong lech nguon"
	@echo "  package     - Dong goi tarball Linux"
	@echo "  clean       - Xoa thu muc dist/"
	@echo "  fmt         - Dinh dang cay Go (gofmt -w)"
	@echo "  lint        - Chay lint-go + lint-py + lint-js"
	@echo "  lint-go     - Kiem dinh dang Go + go vet + staticcheck"
	@echo "  lint-py     - Kiem loi Python (ruff check)"
	@echo "  lint-js     - Kiem loi JS (ESLint: add-in WPS ES5 + tests modern)"

test: test-go test-lo test-bench test-catalog test-wps test-wps-e2e test-core test-mcp test-oracle test-install test-package

# -race: Execute ghi trang thai luot tren goroutine rieng trong khi API doc (agent.Run co khoa rieng).
test-go:
	cd core-go && go test -race ./...

test-lo:
	python3 -m unittest discover -s tests/lo -v

# Bo cham benchmark slide (docs/plans/18): thuan, khong can PowerPoint. Chay bo brief that: tests/bench/slides/run.py.
test-bench:
	python3 -m unittest discover -s tests/bench/slides -v

# Catalog lenh bridge (catalog/live-commands.json - nguon chuan): dinh dang, tham so chon tai lieu, khop ban C# (khi da build).
test-catalog:
	python3 -m unittest discover -s tests/catalog -v

test-wps:
	python3 tests/wps/test_install_jsaddon.py
	node --test tests/wps/test_addin_fake.mjs tests/wps/test_addin.mjs tests/wps/test_addin_et.mjs tests/wps/test_addin_wpp.mjs tests/wps/test_addin_writer.mjs tests/wps/test_addin_goto.mjs tests/wps/test_probe.mjs tests/wps/test_pane.mjs tests/wps/test_pane_theme.mjs tests/wps/test_pane_view.mjs tests/wps/test_mini_view.mjs

# Hai bo e2e duoi day chay Core THAT qua HTTP nhung khong can WPS: chung dong vai add-in bang
# HTTP (poll/result) va LLM gia. Truoc day chung khong nam trong dich nao nen khong ai chay.
test-wps-e2e: core-build
	AXIOM_E2E_CORE_EXE=$(CURDIR)/dist/core/axiom-core python3 tests/wps/test_wps_bridge_e2e.py
	AXIOM_E2E_CORE_EXE=$(CURDIR)/dist/core/axiom-core python3 tests/wps/test_wps_pane_e2e.py

core-build:
	mkdir -p dist/core
	cd core-go && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o ../dist/core/axiom-core ./cmd/axiom-core
	rm -rf dist/core/skills
	cp -r skills dist/core/skills

test-core: core-build
	AXIOM_E2E_CORE_EXE=$(CURDIR)/dist/core/axiom-core python3 tests/core/test_core_e2e.py --only fake_bridge,guards,skills,confirm,cross_app,memory,mcp,visual,setup,shutdown,anthropic,embeddings,summarize,cancel,mcp_http,verify

test-mcp: core-build
	python3 tests/mcp-host/test_mcp_portable.py $(CURDIR)/dist/core/axiom-core

test-oracle:
	cd tests/oracle/word-mcp && python3 -m unittest discover -s tests
	cd tests/oracle/excel-mcp && python3 -m unittest discover -s tests
	cd tests/oracle/ppt-mcp && python3 -m unittest discover -s tests

test-install:
	python3 tests/linux/test_install_sh.py

# Can Microsoft Office x64 + add-in da cai; khong nam trong `test` vi CI khong co Office.
test-live-format:
	python3 tests/live/test_writer_format.py

# Chay package.sh that (build Core + .oxt) roi soi tarball.
test-package:
	python3 tests/linux/test_package.py

check-gen:
	python3 scripts/generate_setup_catalog.py --check
	python3 scripts/generate_mcp_commands.py --check
	python3 scripts/check_version.py
	python3 scripts/check_i18n.py

package:
	bash scripts/linux/package.sh

clean:
	rm -rf dist/

fmt:
	cd core-go && gofmt -l -w .

lint: lint-go lint-py lint-js

lint-go:
	cd core-go && test -z "$$(gofmt -l .)" || (gofmt -l .; exit 1)
	cd core-go && go vet ./...
	cd core-go && $(STATICCHECK) ./...

lint-py:
	$(RUFF) check .

lint-js:
	$(ESLINT) .
