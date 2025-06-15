package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	sourceDir   = "../detection_as_code/detections/"
	destAppDir  = "../detection_as_code/app/detections_app"
	destConfDir = destAppDir + "/default"
	confFile    = destConfDir + "/savedsearches.conf"
	tarballPath = "../detection_as_code/app/detections_app.tar.gz"
)

func main() {
	fmt.Println("🔨 Building Splunk detection app...")

	if _, err := os.Stat(confFile); err == nil {
		err = os.Remove(confFile)
		check(err)
		fmt.Println("🗑️ Removed existing savedsearches.conf")
	}

	if _, err := os.Stat(tarballPath); err == nil {
		err = os.Remove(tarballPath)
		check(err)
		fmt.Println("🗑️ Removed existing tarball:", tarballPath)
	}

	os.MkdirAll(destConfDir, os.ModePerm)

	var output []string

	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		check(err)
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".det") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("⚠️  Skipping %s: %v\n", path, err)
			return nil
		}
		output = append(output, string(data))
		return nil
	})
	check(err)

	check(os.WriteFile(confFile, []byte(strings.Join(output, "\n")), 0644))
	fmt.Println("✅ savedsearches.conf created.")

	err = packageApp(destAppDir, tarballPath)
	check(err)
	fmt.Println("📦 Packaged app:", tarballPath)
}


func packageApp(srcDir, tarball string) error {
	outFile, err := os.Create(tarball)
	if err != nil {
		return err
	}
	defer outFile.Close()

	gzw := gzip.NewWriter(outFile)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	return filepath.Walk(srcDir, func(file string, fi os.FileInfo, err error) error {
		check(err)
		if fi.IsDir() {
			return nil
		}
		relPath := strings.TrimPrefix(file, filepath.Dir(srcDir)+"/")
		hdr, err := tar.FileInfoHeader(fi, "")
		check(err)
		hdr.Name = relPath

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		f, err := os.Open(file)
		check(err)
		defer f.Close()

		_, err = io.Copy(tw, f)
		return err
	})
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
