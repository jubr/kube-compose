package fs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// FileDescriptor is an abstraction of os.File to improve testability of code.
type FileDescriptor interface {
	io.ReadCloser
	Readdir(n int) ([]os.FileInfo, error)
}

// VirtualFileSystem is an abstraction of the file system to improve testability of code.
type VirtualFileSystem interface {
	Abs(name string) (string, error)
	Chdir(dir string) error
	EvalSymlinks(path string) (string, error)
	Getwd() (string, error)
	Mkdir(name string, perm os.FileMode) error
	MkdirAll(name string, perm os.FileMode) error
	Lstat(name string) (os.FileInfo, error)
	Open(name string) (FileDescriptor, error)
	Readlink(name string) (string, error)
	Stat(name string) (os.FileInfo, error)
}

type osFileSystem struct {
}

func (fs *osFileSystem) Abs(name string) (string, error) {
	return filepath.Abs(name)
}

func (fs *osFileSystem) EvalSymlinks(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}

func (fs *osFileSystem) Getwd() (string, error) {
	return os.Getwd()
}

func (fs *osFileSystem) Mkdir(name string, perm os.FileMode) error {
	return os.Mkdir(name, perm)
}

func (fs *osFileSystem) MkdirAll(name string, perm os.FileMode) error {
	return os.MkdirAll(name, perm)
}

func (fs *osFileSystem) Lstat(name string) (os.FileInfo, error) {
	return os.Lstat(name)
}

func (fs *osFileSystem) Open(name string) (FileDescriptor, error) {
	return os.Open(name)
}

func (fs *osFileSystem) Readlink(name string) (string, error) {
	return os.Readlink(name)
}

func (fs *osFileSystem) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

// OS is a VirtualFileSystem that relays directly to Go's "os" and "file/filepath" packages. OS can be replaced by a mock VirtualFileSystem
// to improve testability of code using OS.
var OS VirtualFileSystem = &osFileSystem{}

// InMemoryFileSystem is a VirtualFileSystem with additional fields and functions useful for testing.
type InMemoryFileSystem struct {
	AbsError   error
	cwd        string
	GetwdError error
	root       *node
}

var (
	errBadMode           = fmt.Errorf("file has a bad mode (or operation is not supported on this file)")
	errIsDirDisagreement = fmt.Errorf("data contains a name X that is not a directory, but another name Y indicates " +
		"that X must be a directory")
	errTooManyLinks = fmt.Errorf("too many links")
)

func (fs *InMemoryFileSystem) abs(name string) string {
	if name == "" || name[0] != '/' {
		return fs.cwd + name
	}
	return name
}

type findHelper struct {
	fs                   *InMemoryFileSystem
	ignoreInjectedFaults bool
	links                int
	nameRem              string
	n                    *node
	resolveSymlinks      bool
}

func (f *findHelper) getChildN(nameComp string) (*node, error) {
	var childN *node
	if nameComp != "" {
		validateNameComp(nameComp)
		if (f.n.mode & os.ModeDir) == 0 {
			return nil, syscall.ENOTDIR
		}
		childN = f.n.dirLookup(nameComp)
		if childN == nil {
			return nil, os.ErrNotExist
		}
	}
	return childN, nil
}

func (f *findHelper) getNameComp(slashPos int) string {
	if slashPos < 0 {
		return f.nameRem
	}
	return f.nameRem[:slashPos]
}

func (f *findHelper) run() error {
	for f.nameRem != "" {
		if !f.ignoreInjectedFaults && f.n.err != nil {
			return f.n.err
		}
		slashPos := strings.IndexByte(f.nameRem, '/')
		nameComp := f.getNameComp(slashPos)
		childN, err := f.getChildN(nameComp)
		if err != nil {
			return err
		}
		f.updateNameRemFromSlashPos(slashPos)
		if nameComp != "" {
			err := f.updateFromChildN(childN)
			if err != nil {
				return err
			}
		}
	}
	if !f.ignoreInjectedFaults && f.n.err != nil {
		return f.n.err
	}
	return nil
}

func (f *findHelper) updateFromChildN(childN *node) error {
	if (childN.mode & os.ModeSymlink) != 0 {
		if f.resolveSymlinks {
			f.links++
			if f.links > 255 {
				return errTooManyLinks
			}
			target := childN.extra.([]byte)
			j := 0
			if len(target) > 0 && target[0] == '/' {
				// Absolute path
				j = 1
				f.n = f.fs.root
			}
			f.nameRem = string(target)[j:] + "/" + f.nameRem
		}
	} else {
		f.n = childN
	}
	return nil
}

func (f *findHelper) updateNameRemFromSlashPos(slashPos int) {
	if slashPos < 0 {
		f.nameRem = ""
	} else {
		f.nameRem = f.nameRem[slashPos+1:]
	}
}

func (fs *InMemoryFileSystem) find(
	name string,
	ignoreInjectedFaults, resolveSymlinks bool) (n *node, nameRem string, err error) {
	f := findHelper{
		fs:                   fs,
		ignoreInjectedFaults: ignoreInjectedFaults,
		nameRem:              fs.abs(name)[1:],
		n:                    fs.root,
		resolveSymlinks:      resolveSymlinks,
	}
	err = f.run()
	n = f.n
	nameRem = f.nameRem
	return
}

func validateNameComp(nameComp string) {
	if nameComp == "." || nameComp == ".." {
		panic(fmt.Errorf("name must not contain '//' and must not have a path component that is one of  '..' and '.'"))
	}
}

func (fs *InMemoryFileSystem) createChildren(n *node, nameRem string, vfile *InMemoryFile) {
	for {
		var nameComp string
		slashPos := strings.IndexByte(nameRem, '/')
		if slashPos < 0 {
			nameComp = nameRem
		} else {
			nameComp = nameRem[:slashPos]
		}
		if nameComp != "" {
			validateNameComp(nameComp)
			var childN *node
			if slashPos < 0 {
				// initialize file or directory as per InMemoryFile
				childN = &node{
					err:     vfile.Error,
					errOpen: vfile.OpenError,
					errRead: vfile.ReadError,
					mode:    vfile.Mode,
					name:    nameComp,
				}
				if (vfile.Mode & os.ModeDir) == 0 {
					childN.extra = vfile.Content
				} else {
					childN.extra = []*node{}
				}
				n.dirAppend(childN)
				return
			}
			// initialize directory with defaults
			childN = newDirNode(
				os.ModeDir,
				nameComp,
			)
			n.dirAppend(childN)
			n = childN
		}
		if slashPos < 0 {
			break
		}
		nameRem = nameRem[slashPos+1:]
	}
}

// InMemoryFile is a helper struct used to initialize a file, directory or other type of file in a virtual file system.
// If Error is set then all file system operations will produce an error when the file is accessed. If Mode is a regular
// file then Content is the content of that file. If Mode is Symlink then Content is the location of the Symlink.
type InMemoryFile struct {
	Content   []byte
	Error     error
	Mode      os.FileMode
	OpenError error
	ReadError error
}

// NewInMemoryUnixFileSystem creates a mock file system based on the provided data.
func NewInMemoryUnixFileSystem(data map[string]InMemoryFile) *InMemoryFileSystem {
	fs := &InMemoryFileSystem{
		cwd: "/",
		root: newDirNode(
			0,
			"/",
		),
	}
	for name, vfile := range data {
		// Ignoring pointer to range variable linting error here.
		//nolint
		fs.Set(name, &vfile)
	}
	return fs
}

// Set sets or updates the file at name. If one of the parents of name exists and is not a directory then the error ENOTDIR is returned. If
// a file already exists at name and it is a directory and vfile is not a directory (or vice versa) then an error is thrown. Otherwise, if a
// file already exists at name its attributes, injected fault, symlink target or regular file contents are updated with the values from
// vfile.
func (fs *InMemoryFileSystem) Set(name string, vfile *InMemoryFile) {
	var flag os.FileMode
	switch {
	case vfile.Mode.IsDir():
		flag = os.ModeDir
	case (vfile.Mode & os.ModeSymlink) != 0:
		flag = os.ModeSymlink
	case vfile.Mode.IsRegular():
		flag = 0
	case (vfile.Mode & os.ModeDevice) != 0:
		flag = os.ModeDevice
	}
	if (vfile.Mode & (os.ModeType &^ flag)) != 0 {
		panic(errBadMode)
	}
	n, nameRem, err := fs.find(name, true, false)
	if err == syscall.ENOTDIR {
		panic(errIsDirDisagreement)
	}
	if nameRem != "" {
		fs.createChildren(n, nameRem, vfile)
	} else {
		nodeIsDir := (n.mode & os.ModeDir) != 0
		vfileIsDir := (vfile.Mode & os.ModeDir) != 0
		if nodeIsDir != vfileIsDir {
			panic(errIsDirDisagreement)
		}
		if !vfileIsDir {
			n.extra = vfile.Content
		}
		n.err = vfile.Error
		n.errOpen = vfile.OpenError
		n.errRead = vfile.ReadError
		n.mode = vfile.Mode
	}
}

type virtualFileDescriptor struct {
	node    *node
	readPos int
}

func (r *virtualFileDescriptor) Close() error {
	return nil
}

func (r *virtualFileDescriptor) Read(p []byte) (n int, err error) {
	if !r.node.mode.IsRegular() && (r.node.mode&os.ModeDevice) == 0 {
		err = errBadMode
		return
	}
	if r.node.errRead != nil {
		err = r.node.errRead
		return
	}
	if len(p) > 0 {
		fileContents := r.node.extra.([]byte)
		n = copy(p, fileContents[r.readPos:])
		r.readPos += n
		if n == 0 {
			err = io.EOF
		}
	}
	return
}

func (r *virtualFileDescriptor) Readdir(n int) ([]os.FileInfo, error) {
	if !r.node.mode.IsDir() {
		return nil, syscall.ENOTDIR
	}
	if n > 0 {
		panic(fmt.Errorf("not supported"))
	}
	if r.node.errRead != nil {
		return nil, r.node.errRead
	}
	dir := r.node.extra.([]*node)
	if len(dir) == 0 {
		return nil, nil
	}
	fileInfoSlice := make([]os.FileInfo, len(dir))
	for i := 0; i < len(dir); i++ {
		fileInfoSlice[i] = dir[i]
	}
	return fileInfoSlice, nil
}

func trimTrailingSlashes(name string) string {
	n := len(name)
	for n > 0 && name[n-1] == '/' {
		n--
	}
	return name[:n]
}

func (fs *InMemoryFileSystem) Abs(name string) (string, error) {
	if fs.AbsError != nil {
		return "", fs.AbsError
	}
	return fs.abs(name), nil
}

func (fs *InMemoryFileSystem) Getwd() (string, error) {
	if fs.GetwdError != nil {
		return "", fs.GetwdError
	}
	return fs.cwd, nil
}

func (fs *InMemoryFileSystem) Open(name string) (FileDescriptor, error) {
	node, _, err := fs.find(name, false, true)
	if err != nil {
		return nil, err
	}
	if node.errOpen != nil {
		return nil, node.errOpen
	}
	return &virtualFileDescriptor{
		node: node,
	}, nil
}

func (fs *InMemoryFileSystem) Stat(name string) (os.FileInfo, error) {
	n, _, err := fs.find(name, false, true)
	if err != nil {
		return nil, err
	}
	return n, nil
}
