package forge

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	CPIOFormat           = "newc"
	DeterministicUID     = 0
	DeterministicGID     = 0
	DeterministicMTime   = 0
	CompressionAlgorithm = "gzip"
	CompressionLevel     = "default"

	cpioModeDir        int64 = 0o040755
	cpioModeFile       int64 = 0o100644
	cpioModeExecutable int64 = 0o100755
	cpioModeSymlink    int64 = 0o120777
	cpioModeCharDevice int64 = 0o020600
)

// DeterministicArchiver packs a rootfs into a gzip-compressed newc CPIO initrd.
type DeterministicArchiver struct {
	rootfsDir string
}

type cpioHeader struct {
	inode     int64
	mode      int64
	fileSize  int64
	rdevMajor int64
	rdevMinor int64
	name      string
}

func NewDeterministicArchiver(rootfsDir string) *DeterministicArchiver {
	return &DeterministicArchiver{rootfsDir: rootfsDir}
}

// BuildInitrd produces a deterministic initrd: UID, GID, and mtime are pinned to 0 and CPIO entries are written in traversal order.
func (a *DeterministicArchiver) BuildInitrd(outputPath string) error {
	fmt.Println("Building initrd")

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}

	gzipWriter := gzip.NewWriter(outFile)
	var inode int64 = 1

	if err := filepath.Walk(a.rootfsDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relPath, err := filepath.Rel(a.rootfsDir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}
		relPath = "./" + filepath.ToSlash(relPath)

		if err := a.writeEntry(gzipWriter, path, relPath, info, inode); err != nil {
			return err
		}
		inode++
		return nil
	}); err != nil {
		_ = gzipWriter.Close()
		_ = outFile.Close()
		return err
	}

	if err := writeCPIOHeader(gzipWriter, cpioHeader{inode: inode, name: "TRAILER!!!"}); err != nil {
		_ = gzipWriter.Close()
		_ = outFile.Close()
		return fmt.Errorf("write CPIO trailer: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		_ = outFile.Close()
		return fmt.Errorf("close gzip writer: %w", err)
	}
	if err := outFile.Close(); err != nil {
		return fmt.Errorf("close output file: %w", err)
	}

	fmt.Printf("initrd written: %s\n", outputPath)
	return nil
}

func (a *DeterministicArchiver) writeEntry(w io.Writer, path, relPath string, info os.FileInfo, inode int64) error {
	header, payload, err := a.entryHeaderAndPayload(path, relPath, info, inode)
	if err != nil {
		return err
	}
	if err := writeCPIOHeader(w, header); err != nil {
		return fmt.Errorf("write CPIO header [%s]: %w", relPath, err)
	}
	if len(payload) > 0 {
		return writePayload(w, payload)
	}
	if header.fileSize == 0 {
		return nil
	}
	if err := writeFilePayload(w, path, header.fileSize); err != nil {
		return fmt.Errorf("write file data [%s]: %w", relPath, err)
	}
	return nil
}

func (a *DeterministicArchiver) entryHeaderAndPayload(path, relPath string, info os.FileInfo, inode int64) (cpioHeader, []byte, error) {
	header := cpioHeader{inode: inode, name: relPath}
	mode := info.Mode()
	switch {
	case relPath == "./dev/console":
		header.mode = cpioModeCharDevice
		header.rdevMajor = 5
		header.rdevMinor = 1
		return header, nil, nil
	case mode.IsDir():
		header.mode = cpioModeDir
		return header, nil, nil
	case mode&os.ModeSymlink != 0:
		target, err := os.Readlink(path)
		if err != nil {
			return cpioHeader{}, nil, err
		}
		header.mode = cpioModeSymlink
		header.fileSize = int64(len(target))
		return header, []byte(target), nil
	case !mode.IsRegular():
		return cpioHeader{}, nil, fmt.Errorf("unsupported special file %s: %s", relPath, mode.Type())
	default:
		header.mode = cpioModeFile
		if relPath == "./init" || strings.HasSuffix(relPath, ".ko") {
			header.mode = cpioModeExecutable
		}
		header.fileSize = info.Size()
		return header, nil, nil
	}
}

func writePayload(w io.Writer, payload []byte) error {
	if _, err := w.Write(payload); err != nil {
		return err
	}
	return writePadding(w, int64(len(payload)))
}

func writeFilePayload(w io.Writer, path string, fileSize int64) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}

	_, copyErr := io.Copy(w, f)
	if closeErr := f.Close(); closeErr != nil && copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		return copyErr
	}

	return writePadding(w, fileSize)
}

func writePadding(w io.Writer, size int64) error {
	if padding := (4 - (size % 4)) % 4; padding > 0 {
		_, err := w.Write(make([]byte, padding))
		return err
	}
	return nil
}

// writeCPIOHeader writes an SVR4 newc header; fields are fixed-width ASCII hexadecimal.
func writeCPIOHeader(w io.Writer, header cpioHeader) error {
	nameWithNull := header.name + "\x00"
	nameSize := int64(len(nameWithNull))

	rawHeader := fmt.Sprintf(
		"070701"+
			"%08X%08X"+
			"0000000000000000"+
			"00000001"+
			"00000000"+
			"%08X"+
			"0000000000000000"+
			"%08X%08X"+
			"%08X"+
			"00000000",
		header.inode, header.mode, header.fileSize, header.rdevMajor, header.rdevMinor, nameSize,
	)

	if _, err := io.WriteString(w, rawHeader); err != nil {
		return err
	}
	if _, err := io.WriteString(w, nameWithNull); err != nil {
		return err
	}

	totalHeaderSize := 110 + nameSize
	return writePadding(w, totalHeaderSize)
}
