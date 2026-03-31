package provider

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// InvalidChecksumTypeError is returned when the passed checksum algorithm
// type is not supported
type InvalidChecksumTypeError string

func (e InvalidChecksumTypeError) Error() string {
	return fmt.Sprintf("invalid checksum algorithm: %q", string(e))
}

//nolint:unused
type image struct {
	// Image URL where to download from
	URL string
	// Checksum of the image, used to check integrity after downloading it
	Checksum string
	// Algorithm use to check the checksum
	ChecksumType string
	// Internal file reference
	file io.ReadSeeker
}

func unpackImage(ctx context.Context, imagePath, toDir string) error {
	_, err := os.Stat(toDir)
	finfo, _ := os.ReadDir(toDir)
	dirEmpty := len(finfo) == 0
	if !os.IsNotExist(err) && !dirEmpty {
		return nil // Already unpacked
	}

	if err := os.MkdirAll(toDir, 0740); err != nil {
		return fmt.Errorf("unable to create %s directory: %w", toDir, err)
	}

	tflog.Debug(ctx, "unpacking gold virtual image", map[string]any{
		"image": imagePath,
		"toDir": toDir,
	})

	// Try tar with auto-detect compression first (works on most systems)
	// Use -a flag for auto-detection, fall back to explicit flags
	// Ensure absolute paths for tar
	absImage, err := filepath.Abs(imagePath)
	if err != nil {
		absImage = imagePath
	}
	absDir, err := filepath.Abs(toDir)
	if err != nil {
		absDir = toDir
	}

	// Convert Windows paths to POSIX format for tar compatibility
	// C:\Users\... -> /c/Users/... (required by Git Bash's tar on Windows)
	tarImage := toTarPath(absImage)
	tarDir := toTarPath(absDir)

	for _, tarArgs := range [][]string{
		{"tar", "-xvf", tarImage, "-C", tarDir},  // auto-detect
		{"tar", "-xzvf", tarImage, "-C", tarDir},  // gzip
		{"tar", "-xjvf", tarImage, "-C", tarDir},  // bzip2
		{"tar", "-xJvf", tarImage, "-C", tarDir},  // xz
	} {
		cmd := exec.Command(tarArgs[0], tarArgs[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err == nil {
			// Check if extraction produced any files
			extracted, _ := os.ReadDir(toDir)
			if len(extracted) > 0 {
				return nil
			}
		}
	}

	return fmt.Errorf("error unpacking gold image %s: all tar extraction methods failed", imagePath)
}

// toTarPath converts a Windows path for tar compatibility.
// Under MSYS/Git Bash: C:\Users\... -> /c/Users/... (Git Bash tar needs POSIX paths)
// Under native Windows: C:\Users\... -> C:/Users/... (Windows tar needs forward slashes only)
func toTarPath(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	// Only convert drive letters to /x/ format when running under MSYS (Git Bash)
	if os.Getenv("MSYSTEM") != "" || os.Getenv("MINGW_PREFIX") != "" {
		if len(p) >= 2 && p[1] == ':' {
			drive := strings.ToLower(string(p[0]))
			p = "/" + drive + p[2:]
		}
	}
	return p
}

func gatherDisks(path string) ([]string, error) {
	var disks []string
	err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext == ".vdi" || ext == ".vmdk" {
			disks = append(disks, p)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error walking path %q: %w", path, err)
	}
	if len(disks) == 0 {
		return nil, fmt.Errorf("no VM disk files (*.vdi, *.vmdk) found in path %q", path)
	}
	sort.Sort(ByDiskPriority(disks))
	return disks, nil
}

// ByDiskPriority adds a simple sort to make sure that configdisk is not first
// in the returned boot order.
type ByDiskPriority []string

func (ss ByDiskPriority) Len() int      { return len(ss) }
func (ss ByDiskPriority) Swap(i, j int) { ss[i], ss[j] = ss[j], ss[i] }
func (ss ByDiskPriority) Less(i, j int) bool {
	return !strings.Contains(ss[i], "configdrive")
}

func (img *image) verify(ctx context.Context) error {
	tflog.Debug(ctx, "verifying image checksum")
	var hasher hash.Hash

	switch img.ChecksumType {
	case "md5":
		hasher = md5.New()
	case "sha1":
		hasher = sha1.New()
	case "sha256":
		hasher = sha256.New()
	case "sha512":
		hasher = sha512.New()
	default:
		return InvalidChecksumTypeError(img.ChecksumType)
	}

	// Makes sure the file cursor is positioned at the beginning of the file
	if _, err := img.file.Seek(0, 0); err != nil {
		return fmt.Errorf("can't seek image file: %w", err)
	}

	if _, err := io.Copy(hasher, img.file); err != nil {
		return fmt.Errorf("cannot hash image file: %w", err)
	}

	result := fmt.Sprintf("%x", hasher.Sum(nil))
	if result != img.Checksum {
		return fmt.Errorf("checksum does not match\n Result: %s\n Expected: %s", result, img.Checksum)
	}

	return nil
}
