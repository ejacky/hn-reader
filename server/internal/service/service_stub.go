//go:build !windows

package service

import (
	"errors"
	"net/http"
)

func Install() error {
	return errors.New("service install is only supported on Windows")
}

func Uninstall() error {
	return errors.New("service uninstall is only supported on Windows")
}

func Run(handler http.Handler, addr string) error {
	return errors.New("service run is only supported on Windows")
}
