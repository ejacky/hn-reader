//go:build windows

package service

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	name        = "HNReader"
	displayName = "HN Reader Server"
	description = "HN Reader - Hacker News summarization and recommendation server"
)

func Install() error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to SCM: %w", err)
	}
	defer m.Disconnect()

	s, err := m.CreateService(name, execPath, mgr.Config{
		DisplayName: displayName,
		Description: description,
		StartType:   mgr.StartAutomatic,
		ErrorControl: mgr.ErrorNormal,
	}, "-run-service")
	if err != nil {
		if isServiceExists(err) {
			fmt.Println("Service already installed.")
			return nil
		}
		return fmt.Errorf("create service: %w", err)
	}
	s.Close()
	fmt.Println("Service installed successfully (Startup: Automatic).")
	return nil
}

func Uninstall() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to SCM: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(name)
	if err != nil {
		if isServiceNotExist(err) {
			fmt.Println("Service is not installed.")
			return nil
		}
		return fmt.Errorf("open service: %w", err)
	}
	defer s.Close()

	// Try to stop the service (ignore errors, it may already be stopped)
	s.Control(svc.Stop)
	time.Sleep(500 * time.Millisecond)

	if err := s.Delete(); err != nil {
		return fmt.Errorf("delete service: %w", err)
	}
	fmt.Println("Service uninstalled successfully.")
	return nil
}

func Run(handler http.Handler, addr string) error {
	return svc.Run(name, &hnReaderService{handler: handler, addr: addr})
}

type hnReaderService struct {
	handler http.Handler
	addr    string
}

func (h *hnReaderService) Execute(args []string, requests <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	changes <- svc.Status{State: svc.StartPending}

	srv := &http.Server{
		Addr:    h.addr,
		Handler: h.handler,
	}

	startErr := make(chan error, 1)
	go func() {
		startErr <- srv.ListenAndServe()
	}()

	// Give the server a moment to start, then check for immediate failures
	select {
	case err := <-startErr:
		log.Printf("server failed to start: %v", err)
		changes <- svc.Status{State: svc.Stopped}
		return false, 1
	case <-time.After(500 * time.Millisecond):
	}

	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	for req := range requests {
		switch req.Cmd {
		case svc.Stop, svc.Shutdown:
			changes <- svc.Status{State: svc.StopPending}
			log.Println("shutting down server...")
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			srv.Shutdown(ctx)
			return false, 0

		case svc.Interrogate:
			changes <- req.CurrentStatus

		default:
			changes <- req.CurrentStatus
		}
	}

	return false, 0
}

func isServiceExists(err error) bool {
	// ERROR_SERVICE_EXISTS = 1073
	return errnoCode(err) == 1073
}

func isServiceNotExist(err error) bool {
	// ERROR_SERVICE_DOES_NOT_EXIST = 1060
	return errnoCode(err) == 1060
}

func errnoCode(err error) uint32 {
	type errno interface {
		Errno() uintptr
	}
	if e, ok := err.(errno); ok {
		return uint32(e.Errno())
	}
	return 0
}
