// Runtime confirmation for finding VB-4: the nbdkit plugin logs the S3
// backend config struct, which includes SecretKey, so the object-store
// secret can appear verbatim in structured logs. This reproduces the
// rendering with the same slog call shape used at nbd/viperblock.go:268.
package main

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"

	"github.com/mulgadc/viperblock/viperblock/backends/s3"
)

func main() {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	cfg := s3.S3Config{
		VolumeName: "vol0",
		Bucket:     "b",
		Region:     "ap-southeast-2",
		AccessKey:  "AKIAEXAMPLE",
		SecretKey:  "TOP-secret-object-store-key-value",
		Host:       "https://127.0.0.1:8443",
	}
	// Same shape as nbd/viperblock.go:268 -- struct passed as a dangling arg.
	logger.Info("Creating Viperblock backend with btype, config", cfg)

	out := buf.String()
	fmt.Println("log line:", strings.TrimSpace(out))
	if strings.Contains(out, "TOP-secret-object-store-key-value") {
		fmt.Println("CONFIRMED VB-4: SecretKey value is rendered into the log output.")
	} else {
		fmt.Println("NOT CONFIRMED: secret not present in log output.")
	}
}
