package api

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"

	"github.com/schollz/progressbar/v3"
)

const uploadEndpoint = "/hub/data/document/upload"

var vulerabilityMapping = map[string]string{
	"vulnerability": "vulnerabilities.zip",
	"sbom":          "sbom.zip",
}

func (c *Client) UploadVulnerabilities(filePath, fileType string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			log.Printf("error closing file: %v", cerr)
		}
	}()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", filePath)
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close multipart writer: %w", err)
	}

	bar := progressbar.DefaultBytes(int64(buf.Len()), "uploading")
	proxyReader := progressbar.NewReader(bytes.NewReader(buf.Bytes()), bar)

	req, err := http.NewRequest("POST", c.HubHost+uploadEndpoint, &proxyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	q := req.URL.Query()
	q.Add("name", vulerabilityMapping[fileType])
	q.Add("category", "SECURITY_METADATA")
	q.Add("type", "ZIP")
	req.URL.RawQuery = q.Encode()

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			log.Printf("error closing response body: %v", cerr)
		}
	}()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
