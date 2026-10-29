package forge

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
)

// InitrdEntry is one entry inside a gzip-compressed newc CPIO initrd.
type InitrdEntry struct {
	Name      string
	Mode      int64
	UID       int64
	GID       int64
	MTime     int64
	RdevMajor int64
	RdevMinor int64
	Data      []byte
}

// ReadInitrdEntries decompresses a gzip initrd and parses its newc CPIO entries.
func ReadInitrdEntries(initrdPath string) ([]InitrdEntry, error) {
	f, err := os.Open(initrdPath)
	if err != nil {
		return nil, fmt.Errorf("open initrd: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("open gzip: %w", err)
	}
	defer func() {
		_ = gz.Close()
	}()

	cpioBytes, err := io.ReadAll(gz)
	if err != nil {
		return nil, fmt.Errorf("read CPIO: %w", err)
	}

	entries, err := parseNewc(cpioBytes)
	if err != nil {
		return nil, fmt.Errorf("parse CPIO: %w", err)
	}
	return entries, nil
}

func parseNewc(data []byte) ([]InitrdEntry, error) {
	var entries []InitrdEntry
	pos := 0
	for {
		if pos+110 > len(data) {
			return nil, io.ErrUnexpectedEOF
		}
		header := data[pos : pos+110]
		if string(header[:6]) != "070701" {
			return nil, errInvalidNewcMagic
		}
		pos += 110

		var mode, uid, gid, mtime, fileSize, rdevMajor, rdevMinor, nameSize int64
		for _, field := range []struct {
			start, end int
			dst        *int64
		}{
			{14, 22, &mode},
			{22, 30, &uid},
			{30, 38, &gid},
			{46, 54, &mtime},
			{54, 62, &fileSize},
			{78, 86, &rdevMajor},
			{86, 94, &rdevMinor},
			{94, 102, &nameSize},
		} {
			value, err := parseNewcHex(header[field.start:field.end])
			if err != nil {
				return nil, err
			}
			*field.dst = value
		}

		nameEnd := pos + int(nameSize)
		if nameEnd > len(data) || nameSize == 0 {
			return nil, io.ErrUnexpectedEOF
		}
		name := string(data[pos : nameEnd-1])
		pos = align4(nameEnd)

		dataEnd := pos + int(fileSize)
		if dataEnd > len(data) {
			return nil, io.ErrUnexpectedEOF
		}
		entryData := append([]byte(nil), data[pos:dataEnd]...)
		pos = align4(dataEnd)

		entries = append(entries, InitrdEntry{
			Name:      name,
			Mode:      mode,
			UID:       uid,
			GID:       gid,
			MTime:     mtime,
			RdevMajor: rdevMajor,
			RdevMinor: rdevMinor,
			Data:      entryData,
		})
		if name == "TRAILER!!!" {
			return entries, nil
		}
	}
}

var errInvalidNewcMagic = errors.New("invalid newc magic")

func parseNewcHex(raw []byte) (int64, error) {
	return strconv.ParseInt(string(raw), 16, 64)
}

func align4(value int) int {
	return (value + 3) &^ 3
}
