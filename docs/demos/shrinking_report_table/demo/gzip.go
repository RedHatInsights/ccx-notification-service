package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
)

func gzipBytes(src []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)

	_, err := zw.Write(src)
	if err != nil {
		return nil, err
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func gzipString(src string) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)

	_, err := zw.Write([]byte(src))
	if err != nil {
		return nil, err
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func percentage(l1, l2 int) string {
	p := 100.0 * float32(l1-l2) / float32(l1)
	return fmt.Sprintf("%.0f%%", p)
}

func gzipFile(filename string) {
	fmt.Printf("Filename: %s\n", filename)

	content, err := os.ReadFile(filename)
	if err != nil {
		panic(err)
	}

	input := string(content)

	gzippedBytes, err := gzipString(input)
	if err != nil {
		panic(err)
	}

	l1 := len(input)
	l2 := len(gzippedBytes)

	fmt.Printf("String: %d  Zipped out: %d  Stripped: %s\n", l1, l2, percentage(l1, l2))

	bytes := []byte(input)
	gzippedBytes, err = gzipBytes(bytes)
	if err != nil {
		panic(err)
	}

	l3 := len(gzippedBytes)
	fmt.Printf("String: %d  Zipped out: %d  Stripped: %s\n\n", l1, l3, percentage(l1, l3))
}

func main() {
	gzipFile("data/empty_report.json")
	gzipFile("data/large_report.json")
}
