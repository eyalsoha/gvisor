// Copyright 2019 The gVisor Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package linux

import (
	"strings"

	"gvisor.dev/gvisor/pkg/abi/linux"
	"gvisor.dev/gvisor/pkg/sentry/arch"
	"gvisor.dev/gvisor/pkg/sentry/fs"
	"gvisor.dev/gvisor/pkg/sentry/kernel"
	"gvisor.dev/gvisor/pkg/sentry/usermem"
	"gvisor.dev/gvisor/pkg/syserror"
)

// GetXattr implements linux syscall getxattr(2).
func GetXattr(t *kernel.Task, args arch.SyscallArguments) (uintptr, *kernel.SyscallControl, error) {
	return getXattrFromPath(t, args, true)
}

// LGetXattr implements linux syscall lgetxattr(2).
func LGetXattr(t *kernel.Task, args arch.SyscallArguments) (uintptr, *kernel.SyscallControl, error) {
	return getXattrFromPath(t, args, false)
}

// FGetXattr implements linux syscall fgetxattr(2).
func FGetXattr(t *kernel.Task, args arch.SyscallArguments) (uintptr, *kernel.SyscallControl, error) {
	fd := args[0].Int()
	nameAddr := args[1].Pointer()
	valueAddr := args[2].Pointer()
	size := uint64(args[3].SizeT())

	// TODO(b/113957122): Return EBADF if the fd was opened with O_PATH.
	f := t.GetFile(fd)
	if f == nil {
		return 0, nil, syserror.EBADF
	}
	defer f.DecRef()

	n, value, err := getXattr(t, f.Dirent, nameAddr, size)
	if err != nil {
		return 0, nil, err
	}

	if _, err = t.CopyOutBytes(valueAddr, []byte(value)); err != nil {
		return 0, nil, err
	}
	return uintptr(n), nil, nil
}

func getXattrFromPath(t *kernel.Task, args arch.SyscallArguments, resolveSymlink bool) (uintptr, *kernel.SyscallControl, error) {
	pathAddr := args[0].Pointer()
	nameAddr := args[1].Pointer()
	valueAddr := args[2].Pointer()
	size := uint64(args[3].SizeT())

	path, dirPath, err := copyInPath(t, pathAddr, false /* allowEmpty */)
	if err != nil {
		return 0, nil, err
	}

	valueLen := 0
	err = fileOpOn(t, linux.AT_FDCWD, path, resolveSymlink, func(root *fs.Dirent, d *fs.Dirent, _ uint) error {
		if dirPath && !fs.IsDir(d.Inode.StableAttr) {
			return syserror.ENOTDIR
		}

		n, value, err := getXattr(t, d, nameAddr, size)
		valueLen = n
		if err != nil {
			return err
		}

		_, err = t.CopyOutBytes(valueAddr, []byte(value))
		return err
	})
	if err != nil {
		return 0, nil, err
	}
	return uintptr(valueLen), nil, nil
}

// getXattr implements getxattr(2) from the given *fs.Dirent.
func getXattr(t *kernel.Task, d *fs.Dirent, nameAddr usermem.Addr, size uint64) (int, string, error) {
	if err := checkXattrPermissions(t, d.Inode, fs.PermMask{Read: true}); err != nil {
		return 0, "", err
	}

	name, err := copyInXattrName(t, nameAddr)
	if err != nil {
		return 0, "", err
	}

	if !strings.HasPrefix(name, linux.XATTR_USER_PREFIX) {
		return 0, "", syserror.EOPNOTSUPP
	}

	// If getxattr(2) is called with size 0, the size of the value will be
	// returned successfully even if it is nonzero. In that case, we need to
	// retrieve the entire attribute value so we can return the correct size.
	requestedSize := size
	if size == 0 || size > linux.XATTR_SIZE_MAX {
		requestedSize = linux.XATTR_SIZE_MAX
	}

	value, err := d.Inode.GetXattr(t, name, requestedSize)
	if err != nil {
		return 0, "", err
	}
	n := len(value)
	if uint64(n) > requestedSize {
		return 0, "", syserror.ERANGE
	}

	// Don't copy out the attribute value if size is 0.
	if size == 0 {
		return n, "", nil
	}
	return n, value, nil
}

// SetXattr implements linux syscall setxattr(2).
func SetXattr(t *kernel.Task, args arch.SyscallArguments) (uintptr, *kernel.SyscallControl, error) {
	return setXattrFromPath(t, args, true)
}

// LSetXattr implements linux syscall lsetxattr(2).
func LSetXattr(t *kernel.Task, args arch.SyscallArguments) (uintptr, *kernel.SyscallControl, error) {
	return setXattrFromPath(t, args, false)
}

// FSetXattr implements linux syscall fsetxattr(2).
func FSetXattr(t *kernel.Task, args arch.SyscallArguments) (uintptr, *kernel.SyscallControl, error) {
	fd := args[0].Int()
	nameAddr := args[1].Pointer()
	valueAddr := args[2].Pointer()
	size := uint64(args[3].SizeT())
	flags := args[4].Uint()

	// TODO(b/113957122): Return EBADF if the fd was opened with O_PATH.
	f := t.GetFile(fd)
	if f == nil {
		return 0, nil, syserror.EBADF
	}
	defer f.DecRef()

	return 0, nil, setXattr(t, f.Dirent, nameAddr, valueAddr, uint64(size), flags)
}

func setXattrFromPath(t *kernel.Task, args arch.SyscallArguments, resolveSymlink bool) (uintptr, *kernel.SyscallControl, error) {
	pathAddr := args[0].Pointer()
	nameAddr := args[1].Pointer()
	valueAddr := args[2].Pointer()
	size := uint64(args[3].SizeT())
	flags := args[4].Uint()

	path, dirPath, err := copyInPath(t, pathAddr, false /* allowEmpty */)
	if err != nil {
		return 0, nil, err
	}

	return 0, nil, fileOpOn(t, linux.AT_FDCWD, path, resolveSymlink, func(root *fs.Dirent, d *fs.Dirent, _ uint) error {
		if dirPath && !fs.IsDir(d.Inode.StableAttr) {
			return syserror.ENOTDIR
		}

		return setXattr(t, d, nameAddr, valueAddr, uint64(size), flags)
	})
}

// setXattr implements setxattr(2) from the given *fs.Dirent.
func setXattr(t *kernel.Task, d *fs.Dirent, nameAddr, valueAddr usermem.Addr, size uint64, flags uint32) error {
	if flags&^(linux.XATTR_CREATE|linux.XATTR_REPLACE) != 0 {
		return syserror.EINVAL
	}

	if err := checkXattrPermissions(t, d.Inode, fs.PermMask{Write: true}); err != nil {
		return err
	}

	name, err := copyInXattrName(t, nameAddr)
	if err != nil {
		return err
	}

	if size > linux.XATTR_SIZE_MAX {
		return syserror.E2BIG
	}
	buf := make([]byte, size)
	if _, err = t.CopyInBytes(valueAddr, buf); err != nil {
		return err
	}
	value := string(buf)

	if !strings.HasPrefix(name, linux.XATTR_USER_PREFIX) {
		return syserror.EOPNOTSUPP
	}

	return d.Inode.SetXattr(t, d, name, value, flags)
}

func copyInXattrName(t *kernel.Task, nameAddr usermem.Addr) (string, error) {
	name, err := t.CopyInString(nameAddr, linux.XATTR_NAME_MAX+1)
	if err != nil {
		if err == syserror.ENAMETOOLONG {
			return "", syserror.ERANGE
		}
		return "", err
	}
	if len(name) == 0 {
		return "", syserror.ERANGE
	}
	return name, nil
}

func checkXattrPermissions(t *kernel.Task, i *fs.Inode, perms fs.PermMask) error {
	// Restrict xattrs to regular files and directories.
	//
	// In Linux, this restriction technically only applies to xattrs in the
	// "user.*" namespace, but we don't allow any other xattr prefixes anyway.
	if !fs.IsRegular(i.StableAttr) && !fs.IsDir(i.StableAttr) {
		if perms.Write {
			return syserror.EPERM
		}
		return syserror.ENODATA
	}

	return i.CheckPermission(t, perms)
}
