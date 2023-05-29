package types

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

var logFormatter log.Formatter
var LogOutput *lumberjack.Logger //io.Writer
var logLevel log.Level

func SetDefaultLoggerConfig(cfgMode string, cfgFolder string, cfgLevel log.Level, maxSize int, maxFiles int, maxAge int, compress *bool, forceColors bool) error {
	/*Configure logs*/
	if cfgMode == "file" {
		_maxsize := 500
		if maxSize != 0 {
			_maxsize = maxSize
		}
		_maxfiles := 3
		if maxFiles != 0 {
			_maxfiles = maxFiles
		}
		_maxage := 28
		if maxAge != 0 {
			_maxage = maxAge
		}
		_compress := true
		if compress != nil {
			_compress = *compress
		}
		/*cf. https://github.com/natefinch/lumberjack/issues/82
		let's create the file beforehand w/ the right perms */
		fname := cfgFolder + "/crowdsec.log"
		// check if file exists
		_, err := os.Stat(fname)
		// create file if not exists, purposefully ignore errors
		if os.IsNotExist(err) {
			file, _ := os.OpenFile(fname, os.O_RDWR|os.O_CREATE, 0600)
			file.Close()
		}

		LogOutput = &lumberjack.Logger{
			Filename:   fname,
			MaxSize:    _maxsize,
			MaxBackups: _maxfiles,
			MaxAge:     _maxage,
			Compress:   _compress,
		}
		log.SetOutput(LogOutput)
	} else if cfgMode != "stdout" {
		return fmt.Errorf("log mode '%s' unknown", cfgMode)
	}
	logLevel = cfgLevel
	log.SetLevel(logLevel)
	logFormatter = &log.TextFormatter{TimestampFormat: "02-01-2006 15:04:05", FullTimestamp: true, ForceColors: forceColors}
	log.SetFormatter(logFormatter)
	return nil
}

func ConfigureLogger(clog *log.Logger) error {
	/*Configure logs*/
	if LogOutput != nil {
		clog.SetOutput(LogOutput)
	}

	if logFormatter != nil {
		clog.SetFormatter(logFormatter)
	}
	clog.SetLevel(logLevel)
	return nil
}

func Clone(a, b interface{}) error {
	buff := new(bytes.Buffer)
	enc := gob.NewEncoder(buff)
	dec := gob.NewDecoder(buff)
	if err := enc.Encode(a); err != nil {
		return fmt.Errorf("failed cloning %T", a)
	}
	if err := dec.Decode(b); err != nil {
		return fmt.Errorf("failed cloning %T", b)
	}
	return nil
}

func ParseDuration(d string) (time.Duration, error) {
	durationStr := d
	if strings.HasSuffix(d, "d") {
		days := strings.Split(d, "d")[0]
		if len(days) == 0 {
			return 0, fmt.Errorf("'%s' can't be parsed as duration", d)
		}
		daysInt, err := strconv.Atoi(days)
		if err != nil {
			return 0, err
		}
		durationStr = strconv.Itoa(daysInt*24) + "h"
	}
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		return 0, err
	}
	return duration, nil
}

/*help to copy the file, ioutil doesn't offer the feature*/

func copyFileContents(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}

/*copy the file, ioutile doesn't offer the feature*/
func CopyFile(sourceSymLink, destinationFile string) (err error) {
	sourceFile, err := filepath.EvalSymlinks(sourceSymLink)
	if err != nil {
		log.Infof("Not a symlink : %s", err)
		sourceFile = sourceSymLink
	}

	sourceFileStat, err := os.Stat(sourceFile)
	if err != nil {
		return
	}
	if !sourceFileStat.Mode().IsRegular() {
		// cannot copy non-regular files (e.g., directories,
		// symlinks, devices, etc.)
		return fmt.Errorf("copyFile: non-regular source file %s (%q)", sourceFileStat.Name(), sourceFileStat.Mode().String())
	}
	destinationFileStat, err := os.Stat(destinationFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return
		}
	} else {
		if !(destinationFileStat.Mode().IsRegular()) {
			return fmt.Errorf("copyFile: non-regular destination file %s (%q)", destinationFileStat.Name(), destinationFileStat.Mode().String())
		}
		if os.SameFile(sourceFileStat, destinationFileStat) {
			return
		}
	}
	if err = os.Link(sourceFile, destinationFile); err != nil {
		err = copyFileContents(sourceFile, destinationFile)
	}
	return
}

func StrPtr(s string) *string {
	return &s
}

func IntPtr(i int) *int {
	return &i
}

func Int32Ptr(i int32) *int32 {
	return &i
}

func BoolPtr(b bool) *bool {
	return &b
}

func InSlice(str string, slice []string) bool {
	for _, item := range slice {
		if str == item {
			return true
		}
	}
	return false
}

func UtcNow() time.Time {
	return time.Now().UTC()
}

func GetLineCountForFile(filepath string) int {
	f, err := os.Open(filepath)
	if err != nil {
		log.Fatalf("unable to open log file %s : %s", filepath, err)
	}
	defer f.Close()
	lc := 0
	fs := bufio.NewScanner(f)
	for fs.Scan() {
		lc++
	}
	return lc
}

// from https://github.com/acarl005/stripansi
var reStripAnsi = regexp.MustCompile("[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))")

func StripAnsiString(str string) string {
	// the byte version doesn't strip correctly
	return reStripAnsi.ReplaceAllString(str, "")
}

// Generated with `man statfs | grep _MAGIC | awk '{split(tolower($1),a,"_"); print $2 ": \"" a[1] "\","}'`
// ext2/3/4 duplicates removed to just have ext4
// XIAFS removed as well
var fsTypeMapping map[int]string = map[int]string{
	0xadf5:     "adfs",
	0xadff:     "affs",
	0x5346414f: "afs",
	0x09041934: "anon",
	0x0187:     "autofs",
	0x62646576: "bdevfs",
	0x42465331: "befs",
	0x1badface: "bfs",
	0x42494e4d: "binfmtfs",
	0xcafe4a11: "bpf",
	0x9123683e: "btrfs",
	0x73727279: "btrfs",
	0x27e0eb:   "cgroup",
	0x63677270: "cgroup2",
	0xff534d42: "cifs",
	0x73757245: "coda",
	0x012ff7b7: "coh",
	0x28cd3d45: "cramfs",
	0x64626720: "debugfs",
	0x1373:     "devfs",
	0x1cd1:     "devpts",
	0xf15f:     "ecryptfs",
	0xde5e81e4: "efivarfs",
	0x00414a53: "efs",
	0x137d:     "ext",
	0xef51:     "ext2",
	0xef53:     "ext4",
	0xf2f52010: "f2fs",
	0x65735546: "fuse",
	0xbad1dea:  "futexfs",
	0x4244:     "hfs",
	0x00c0ffee: "hostfs",
	0xf995e849: "hpfs",
	0x958458f6: "hugetlbfs",
	0x9660:     "isofs",
	0x72b6:     "jffs2",
	0x3153464a: "jfs",
	0x137f:     "minix",
	0x138f:     "minix",
	0x2468:     "minix2",
	0x2478:     "minix2",
	0x4d5a:     "minix3",
	0x19800202: "mqueue",
	0x4d44:     "msdos",
	0x11307854: "mtd",
	0x564c:     "ncp",
	0x6969:     "nfs",
	0x3434:     "nilfs",
	0x6e736673: "nsfs",
	0x5346544e: "ntfs",
	0x7461636f: "ocfs2",
	0x9fa1:     "openprom",
	0x794c7630: "overlayfs",
	0x50495045: "pipefs",
	0x9fa0:     "proc",
	0x6165676c: "pstorefs",
	0x002f:     "qnx4",
	0x68191122: "qnx6",
	0x858458f6: "ramfs",
	0x52654973: "reiserfs",
	0x7275:     "romfs",
	0x73636673: "securityfs",
	0xf97cff8c: "selinux",
	0x43415d53: "smack",
	0x517b:     "smb",
	0xfe534d42: "smb2",
	0x534f434b: "sockfs",
	0x73717368: "squashfs",
	0x62656572: "sysfs",
	0x012ff7b6: "sysv2",
	0x012ff7b5: "sysv4",
	0x01021994: "tmpfs",
	0x74726163: "tracefs",
	0x15013346: "udf",
	0x00011954: "ufs",
	0x9fa2:     "usbdevice",
	0x01021997: "v9fs",
	0xa501fcf5: "vxfs",
	0xabba1974: "xenfs",
	0x012ff7b4: "xenix",
	0x58465342: "xfs",
}

func GetFSType(path string) (string, error) {
	var buf syscall.Statfs_t

	err := syscall.Statfs(path, &buf)

	if err != nil {
		return "", err
	}

	fsType, ok := fsTypeMapping[int(buf.Type)]
	if !ok {
		return "", fmt.Errorf("unknown fstype %d", buf.Type)
	}

	return fsType, nil
}
