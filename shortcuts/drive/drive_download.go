// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package drive

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	// validate import used below

	"github.com/larksuite/cli/internal/output"
	"github.com/larksuite/cli/internal/validate"
	"github.com/larksuite/cli/shortcuts/common"
)

var DriveDownload = common.Shortcut{
	Service:     "drive",
	Command:     "+download",
	Description: "Download a file from Drive to local",
	Risk:        "read",
	Scopes:      []string{"drive:file:download"},
	AuthTypes:   []string{"user", "bot"},
	Flags: []common.Flag{
		{Name: "file-token", Desc: "file token", Required: true},
		{Name: "output", Desc: "local save path"},
		{Name: "overwrite", Type: "bool", Desc: "overwrite existing output file"},
	},
	DryRun: func(ctx context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
		fileToken := runtime.Str("file-token")
		outputPath := runtime.Str("output")
		if outputPath == "" {
			outputPath = fileToken
		}
		return common.NewDryRunAPI().
			GET("/open-apis/drive/v1/files/:file_token/download").
			Set("file_token", fileToken).Set("output", outputPath)
	},
	Execute: func(ctx context.Context, runtime *common.RuntimeContext) error {
		fileToken := runtime.Str("file-token")
		outputPath := runtime.Str("output")
		overwrite := runtime.Bool("overwrite")

		if err := validate.ResourceName(fileToken, "--file-token"); err != nil {
			return output.ErrValidation("%s", err)
		}

		if outputPath == "" {
			outputPath = fileToken
		}
		safePath, err := validate.SafeOutputPath(outputPath)
		if err != nil {
			return output.ErrValidation("unsafe output path: %s", err)
		}
		if err := common.EnsureWritableFile(safePath, overwrite); err != nil {
			return err
		}

		fmt.Fprintf(runtime.IO().ErrOut, "Downloading: %s\n", common.MaskToken(fileToken))

		apiResp, err := runtime.DoAPI(&larkcore.ApiReq{
			HttpMethod: http.MethodGet,
			ApiPath:    fmt.Sprintf("/open-apis/drive/v1/files/%s/download", validate.EncodePathSegment(fileToken)),
		}, larkcore.WithFileDownload())
		if err != nil {
			return output.ErrNetwork("download failed: %s", err)
		}

		if apiResp.StatusCode >= 400 {
			return output.ErrNetwork("download failed: HTTP %d: %s", apiResp.StatusCode, string(apiResp.RawBody))
		}

		os.MkdirAll(filepath.Dir(safePath), 0755)

		if err := validate.AtomicWrite(safePath, apiResp.RawBody, 0644); err != nil {
			return output.Errorf(output.ExitInternal, "api_error", "cannot create file: %s", err)
		}

		runtime.Out(map[string]interface{}{
			"saved_path": safePath,
			"size_bytes": len(apiResp.RawBody),
		}, nil)
		return nil
	},
}
