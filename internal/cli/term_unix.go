//go:build unix

package cli

import (
	"fmt"
	"io"
	"os"
	"syscall"
	"unsafe"
)

func makeRawInput(r io.Reader) (func() error, error) {
	f, ok := r.(*os.File)
	if !ok {
		return nil, nil
	}
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if (info.Mode() & os.ModeCharDevice) == 0 {
		return nil, nil
	}

	fd := int(f.Fd())
	orig, err := getTermios(fd)
	if err != nil {
		return nil, fmt.Errorf("read terminal state: %w", err)
	}
	raw := *orig
	raw.Lflag &^= syscall.ICANON | syscall.ECHO
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0
	if err := setTermios(fd, &raw); err != nil {
		return nil, fmt.Errorf("enable raw terminal input: %w", err)
	}
	return func() error {
		if err := setTermios(fd, orig); err != nil {
			return fmt.Errorf("restore terminal state: %w", err)
		}
		return nil
	}, nil
}

func getTermios(fd int) (*syscall.Termios, error) {
	t := &syscall.Termios{}
	_, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(syscall.TCGETS),
		uintptr(unsafe.Pointer(t)),
		0, 0, 0,
	)
	if errno != 0 {
		return nil, errno
	}
	return t, nil
}

func setTermios(fd int, t *syscall.Termios) error {
	_, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(syscall.TCSETS),
		uintptr(unsafe.Pointer(t)),
		0, 0, 0,
	)
	if errno != 0 {
		return errno
	}
	return nil
}
