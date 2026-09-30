// axiom-core - Agent Core viet bang Go (ban thay the dan cho AxiomOffice.Core cua .NET, xem core-go/README.md):
// mot tien trinh cho moi nguoi dung, chi nghe 127.0.0.1; add-in/extension tim no qua core.json.
package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"axiomoffice/core/internal/agent"
	"axiomoffice/core/internal/api"
	"axiomoffice/core/internal/config"
	"axiomoffice/core/internal/corefile"
	"axiomoffice/core/internal/corelog"
	"axiomoffice/core/internal/instance"
	"axiomoffice/core/internal/model"
	"axiomoffice/core/internal/office"
	"axiomoffice/core/internal/skills"
	"axiomoffice/core/internal/store"
)

// version gan luc build: go build -ldflags "-X main.version=1.2.3" (build.ps1 dung chung so voi add-in).
var version = "0.0.0"

func main() {
	os.Exit(run())
}

func run() int {
	cfg := config.Load()
	paths := config.NewPaths(cfg.DataDirOverride)
	if err := paths.EnsureDirectories(); err != nil {
		fmt.Fprintln(os.Stderr, "cannot create data directory:", err)
	}
	corelog.Init(paths.LogFile)

	// Mot Core cho moi nguoi dung: instance thu hai thoat ngay (New_arch.md muc 4.3).
	if cfg.SingleInstance {
		lock, acquired, err := instance.Acquire(cfg.MutexName)
		if err != nil {
			corelog.Error("single-instance lock failed: %v", err)
		} else if !acquired {
			corelog.Info("Agent Core is already running; this instance exits.")
			return 0
		}
		defer lock.Release()
	}

	// Port: uu tien CorePort, ban thi thu 9 port ke tiep; 0 = de he dieu hanh cap (test).
	listener, err := listen(cfg)
	if err != nil {
		corelog.Error("%s", err.Error())
		fmt.Fprintln(os.Stderr, err.Error())
		return 4
	}
	port := listener.Addr().(*net.TCPAddr).Port

	runtime := &api.Runtime{Port: port, Started: time.Now().UTC(), Version: version}
	stores := store.OpenStores(paths.DatabaseFile)
	httpClient := &http.Client{} // khong timeout chung: moi request tu dat han bang context
	models := model.NewSource(httpClient, config.Load)
	current := models.Current()
	corelog.Info("Agent Core starting: port=%d pid=%d version=%s singleInstance=%t dataDir=%s sessionDir=%s provider=%s model=%s",
		port, os.Getpid(), version, cfg.SingleInstance, paths.Root, sessionDir(cfg), current.Codec.Name(), current.Model)

	// Skill: skills/ canh binary -> SkillDirs cua to chuc -> thu muc skills cua nguoi dung (muc 8.4.4).
	exe, _ := os.Executable()
	watchCtx, stopWatch := context.WithCancel(context.Background())
	defer stopWatch()
	skillIndex := skills.New(skills.DefaultSources(
		filepath.Join(filepath.Dir(exe), "skills"), cfg.SkillDirs, paths.SkillsDir))
	skillIndex.Watch(watchCtx)

	// Bridge + session registry: noi Core dieu khien add-in/extension dang chay.
	bridge := office.NewBridgeClient(httpClient, cfg.Token)
	sessions := office.NewDirectory(cfg.SessionDirectoryOverride)
	manager := agent.NewManager()
	orchestrator := &agent.Orchestrator{
		Bridge: bridge, Sessions: sessions,
		Conversations: stores.Convs, Runs: stores.Runs,
		Models: models, Skills: skillIndex, Config: config.Load, Assembler: agent.NewContextAssembler(),
	}

	server := &http.Server{ReadHeaderTimeout: 10 * time.Second}
	var stopOnce sync.Once
	stop := func() {
		stopOnce.Do(func() {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if server.Shutdown(ctx) != nil {
					_ = server.Close()
				}
			}()
		})
	}

	mux := http.NewServeMux()
	server.Handler = api.Handler(&api.Deps{
		Config:  cfg,
		Paths:   paths,
		Runtime: runtime,
		HTTP:    httpClient,
		Stores:  stores,
		Skills:  skillIndex,

		Manager:      manager,
		Orchestrator: orchestrator,

		Stop: stop,
	}, mux)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signals
		corelog.Info("shutdown requested by signal")
		stop()
	}()

	corefile.Write(paths.CoreJSON, corefile.Info{
		Pid: os.Getpid(), Port: port, Version: version, Started: runtime.StartedText(), Protocol: api.Protocol, Exe: exe,
	})
	corelog.Info("Agent Core ready: port=%d pid=%d version=%s protocol=%d dataDir=%s memory=%s",
		port, os.Getpid(), version, api.Protocol, paths.Root, stores.Status(cfg.MemoryEnabled))

	serveErr := server.Serve(listener)
	// Dong ket noi dang nho (keep-alive toi bridge) truoc khi thoat: ban .NET lam viec nay bang
	// HttpClient.Dispose(); thoat tien trinh khi con socket mo lam phia bridge nhan RST thay vi dong em.
	httpClient.CloseIdleConnections()
	corefile.Delete(paths.CoreJSON)
	if stores.DB != nil {
		_ = stores.DB.Close()
	}
	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		corelog.Error("Agent Core crashed: %v", serveErr)
		return 5
	}
	corelog.Info("Agent Core stopped")
	return 0
}

// listen giu luon listener cua port tim duoc (ban .NET tha port roi Kestrel bind lai - co khe ho tranh chap).
func listen(cfg config.Config) (net.Listener, error) {
	if cfg.DynamicPort() {
		return net.Listen("tcp", "127.0.0.1:0")
	}
	for i := 0; i < 10; i++ {
		port := cfg.CorePort + i
		if port < 1 || port > 65535 {
			break
		}
		if listener, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port)); err == nil {
			return listener, nil
		}
	}
	return nil, fmt.Errorf("No free port in range %d..%d; Agent Core cannot start.", cfg.CorePort, cfg.CorePort+9)
}

func sessionDir(cfg config.Config) string {
	if cfg.SessionDirectoryOverride != "" {
		return cfg.SessionDirectoryOverride
	}
	return config.DefaultSessionDir()
}
