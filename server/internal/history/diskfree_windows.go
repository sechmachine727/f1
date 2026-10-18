//go:build windows

package history

import "golang.org/x/sys/windows"

func realDiskFree(path string) (int64, int64, error) {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, err
	}
	var free, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(ptr, &free, &total, &totalFree); err != nil {
		return 0, 0, err
	}
	return int64(free), int64(total), nil
}
