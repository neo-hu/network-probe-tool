// +build linux,amd64

package _select

import "syscall"

func SysSelect(n int, r *syscall.FdSet, w *syscall.FdSet, e *syscall.FdSet, timeout *syscall.Timeval) error {
	_, err := syscall.Select(n, r, nil, nil, timeout)
	return err
}
