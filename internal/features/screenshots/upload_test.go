package screenshots

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/tergeoo/asc-mcp/internal/features/screenshots/mocks"
	"github.com/tergeoo/asc-mcp/internal/shared/asc"
	"github.com/tergeoo/asc-mcp/internal/shared/store"
	"github.com/tergeoo/asc-mcp/internal/shared/validate"
)

func writeTempImage(t *testing.T, name string, data []byte) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write temp image: %v", err)
	}
	return path
}

func reservation(id string, ops []uploadOperation) asc.Resource {
	attrs, _ := json.Marshal(map[string]any{"fileName": "shot.png", "uploadOperations": ops})
	return asc.Resource{ID: id, Attributes: attrs}
}

func TestUploadReserveUploadCommit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mocks.NewMockascClient(ctrl)
	svc := &Service{asc: m, store: store.Noop{}}
	ctx := context.Background()

	data := []byte("PNGDATA-bytes-here")
	path := writeTempImage(t, "shot.png", data)
	sum := md5.Sum(data)
	checksum := hex.EncodeToString(sum[:])

	// No existing set for the display type -> one is created.
	m.EXPECT().GetVersionLocScreenshotSets(ctx, "vloc-1").Return(nil, nil)
	m.EXPECT().CreateScreenshotSet(ctx, gomock.Any()).Return(asc.Resource{ID: "set-1"}, nil)

	// Reserve returns a single upload operation covering the whole file.
	ops := []uploadOperation{{Method: "PUT", URL: "https://upload.example/part1", Offset: 0, Length: len(data)}}
	m.EXPECT().ReserveScreenshot(ctx, gomock.Any()).Return(reservation("shot-1", ops), nil)
	m.EXPECT().UploadChunk(ctx, "PUT", "https://upload.example/part1", gomock.Any(), data).Return(nil)

	// Commit must carry the MD5 checksum.
	m.EXPECT().CommitScreenshot(ctx, "shot-1", gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, payload any) (asc.Resource, error) {
			doc := payload.(map[string]any)["data"].(map[string]any)
			attrs := doc["attributes"].(map[string]any)
			if attrs["sourceFileChecksum"] != checksum {
				t.Fatalf("commit checksum = %v, want %s", attrs["sourceFileChecksum"], checksum)
			}
			if attrs["uploaded"] != true {
				t.Fatal("commit must set uploaded=true")
			}
			return reservation("shot-1", nil), nil
		})

	out, err := svc.Upload(ctx, UploadInput{VersionLocalizationID: "vloc-1", DisplayType: "APP_IPHONE_67", FilePath: path})
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	result := out.Result.(map[string]any)
	if result["id"] != "shot-1" || result["setId"] != "set-1" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestUploadReusesExistingSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mocks.NewMockascClient(ctrl)
	svc := &Service{asc: m, store: store.Noop{}}
	ctx := context.Background()

	data := []byte("xx")
	path := writeTempImage(t, "a.jpg", data)

	setAttrsJSON, _ := json.Marshal(map[string]string{"screenshotDisplayType": "APP_IPHONE_67"})
	m.EXPECT().GetVersionLocScreenshotSets(ctx, "vloc-1").
		Return([]asc.Resource{{ID: "set-existing", Attributes: setAttrsJSON}}, nil)
	// No CreateScreenshotSet expected.
	m.EXPECT().ReserveScreenshot(ctx, gomock.Any()).Return(reservation("shot-2", nil), nil)
	m.EXPECT().CommitScreenshot(ctx, "shot-2", gomock.Any()).Return(reservation("shot-2", nil), nil)

	if _, err := svc.Upload(ctx, UploadInput{VersionLocalizationID: "vloc-1", DisplayType: "APP_IPHONE_67", FilePath: path}); err != nil {
		t.Fatalf("upload: %v", err)
	}
}

func TestUploadRejectsBadInput(t *testing.T) {
	svc := &Service{store: store.Noop{}}
	ctx := context.Background()

	// Unsupported extension.
	path := writeTempImage(t, "x.gif", []byte("x"))
	if _, err := svc.Upload(ctx, UploadInput{VersionLocalizationID: "v", DisplayType: "APP_IPHONE_67", FilePath: path}); err == nil {
		t.Fatal("expected error for unsupported format")
	}

	// Bad display type.
	png := writeTempImage(t, "x.png", []byte("x"))
	_, err := svc.Upload(ctx, UploadInput{VersionLocalizationID: "v", DisplayType: "IPHONE", FilePath: png})
	var ve *validate.Error
	if err == nil {
		t.Fatal("expected error for bad display type")
	}
	_ = ve
}
