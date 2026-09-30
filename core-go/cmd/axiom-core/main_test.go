package main

import (
	"net"
	"strconv"
	"strings"
	"testing"

	"axiomoffice/core/internal/config"
)

// Cung ky vong voi CoreRuntimeTests.cs cua ban .NET: Core thu CorePort..CorePort+9 (muc 7.1).
func TestListenPicksNextFreePort(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	port := busy.Addr().(*net.TCPAddr).Port

	listener, err := listen(config.Config{CorePort: port})
	if err != nil {
		t.Fatalf("phai tim duoc port ke tiep: %v", err)
	}
	defer listener.Close()

	picked := listener.Addr().(*net.TCPAddr).Port
	if picked != port+1 {
		t.Fatalf("port dang ban %d -> phai chon %d, nhan duoc %d", port, port+1, picked)
	}
}

func TestListenDynamicPort(t *testing.T) {
	listener, err := listen(config.Config{CorePort: 0})
	if err != nil {
		t.Fatalf("CorePort = 0 phai de he dieu hanh cap port: %v", err)
	}
	defer listener.Close()
	if port := listener.Addr().(*net.TCPAddr).Port; port <= 0 {
		t.Fatalf("port he dieu hanh cap phai > 0, nhan duoc %d", port)
	}
}

func TestListenFailsWhenRangeExhausted(t *testing.T) {
	// Port cuoi cung cua dai: ban chinh no thi khong con port nao trong khoang 65535..65544.
	last := 65535
	busy, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(last))
	if err != nil {
		t.Skipf("port %d dang ban san, bo qua: %v", last, err)
	}
	defer busy.Close()

	_, err = listen(config.Config{CorePort: last})
	if err == nil {
		t.Fatal("het khoang port thi phai loi")
	}
	if !strings.Contains(err.Error(), "No free port in range 65535..65544") {
		t.Fatalf("thong bao loi phai giong ban .NET: %v", err)
	}
}
