package downloads

import (
	"bytes"
	"context"
	"io"
	"net/http/httptest"
	"testing"

	"beeba.org/internal/config"
	downloaddomain "beeba.org/internal/domain/downloads"
	"beeba.org/internal/security/tokens"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5"
)

const testContentID = "11111111-1111-1111-1111-111111111111"

func TestPublicDownloadHonorsSingleByteRange(t *testing.T) {
	store := &fakeStore{
		target: downloadTarget(int64(len("abcdefghijklmnopqrstuvwxyz"))),
	}
	objects := &fakeObjectStore{
		data: []byte("abcdefghijklmnopqrstuvwxyz"),
	}
	app := fiber.New()
	app.Get("/content/:contentID/download", New(config.Config{}, store, objects).Public)

	req := httptest.NewRequest("GET", "/content/"+testContentID+"/download", nil)
	req.Header.Set("Range", "bytes=2-5")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusPartialContent {
		t.Fatalf("expected status %d, got %d", fiber.StatusPartialContent, resp.StatusCode)
	}
	if got := resp.Header.Get("Accept-Ranges"); got != "bytes" {
		t.Fatalf("expected Accept-Ranges bytes, got %q", got)
	}
	if got := resp.Header.Get("Content-Range"); got != "bytes 2-5/26" {
		t.Fatalf("expected Content-Range bytes 2-5/26, got %q", got)
	}
	if got := resp.Header.Get("Content-Length"); got != "4" {
		t.Fatalf("expected Content-Length 4, got %q", got)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != "cdef" {
		t.Fatalf("expected ranged body %q, got %q", "cdef", string(body))
	}
	if objects.fullReads != 0 {
		t.Fatalf("expected no full object reads, got %d", objects.fullReads)
	}
	if objects.rangeStart != 2 || objects.rangeEnd != 5 {
		t.Fatalf("expected range 2-5, got %d-%d", objects.rangeStart, objects.rangeEnd)
	}
	if store.events != 0 {
		t.Fatalf("expected follow-up range request not to record a download event, got %d", store.events)
	}
}

func TestPublicDownloadRecordsInitialByteRange(t *testing.T) {
	store := &fakeStore{
		target: downloadTarget(int64(len("abcdefghijklmnopqrstuvwxyz"))),
	}
	objects := &fakeObjectStore{
		data: []byte("abcdefghijklmnopqrstuvwxyz"),
	}
	app := fiber.New()
	app.Get("/content/:contentID/download", New(config.Config{}, store, objects).Public)

	req := httptest.NewRequest("GET", "/content/"+testContentID+"/download", nil)
	req.Header.Set("Range", "bytes=0-7")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusPartialContent {
		t.Fatalf("expected status %d, got %d", fiber.StatusPartialContent, resp.StatusCode)
	}
	if store.events != 1 {
		t.Fatalf("expected initial range request to record one download event, got %d", store.events)
	}
}

func TestPublicDownloadRejectsUnsatisfiableRangeWithoutOpeningObject(t *testing.T) {
	store := &fakeStore{
		target: downloadTarget(int64(len("abcdefghijklmnopqrstuvwxyz"))),
	}
	objects := &fakeObjectStore{
		data: []byte("abcdefghijklmnopqrstuvwxyz"),
	}
	app := fiber.New()
	app.Get("/content/:contentID/download", New(config.Config{}, store, objects).Public)

	req := httptest.NewRequest("GET", "/content/"+testContentID+"/download", nil)
	req.Header.Set("Range", "bytes=99-120")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("expected status %d, got %d", fiber.StatusRequestedRangeNotSatisfiable, resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Range"); got != "bytes */26" {
		t.Fatalf("expected Content-Range bytes */26, got %q", got)
	}
	if objects.fullReads != 0 || objects.rangeReads != 0 {
		t.Fatalf("expected no object reads, got full=%d range=%d", objects.fullReads, objects.rangeReads)
	}
	if store.events != 0 {
		t.Fatalf("expected no download event for invalid range, got %d", store.events)
	}
}

func TestPublicDownloadHeadAdvertisesRangeWithoutOpeningObject(t *testing.T) {
	store := &fakeStore{
		target: downloadTarget(int64(len("abcdefghijklmnopqrstuvwxyz"))),
	}
	objects := &fakeObjectStore{
		data: []byte("abcdefghijklmnopqrstuvwxyz"),
	}
	app := fiber.New()
	app.Head("/content/:contentID/download", New(config.Config{}, store, objects).Public)

	resp, err := app.Test(httptest.NewRequest("HEAD", "/content/"+testContentID+"/download", nil))
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status %d, got %d", fiber.StatusOK, resp.StatusCode)
	}
	if got := resp.Header.Get("Accept-Ranges"); got != "bytes" {
		t.Fatalf("expected Accept-Ranges bytes, got %q", got)
	}
	if got := resp.Header.Get("Content-Length"); got != "26" {
		t.Fatalf("expected Content-Length 26, got %q", got)
	}
	if objects.fullReads != 0 || objects.rangeReads != 0 {
		t.Fatalf("expected no object reads, got full=%d range=%d", objects.fullReads, objects.rangeReads)
	}
	if store.events != 0 {
		t.Fatalf("expected no download event for HEAD, got %d", store.events)
	}
}

func TestUnlistedPrivateDownloadUsesAccessTokenAndRangeSupport(t *testing.T) {
	accessToken := "bb_dl_secret"
	store := &fakeStore{
		publicErr: pgx.ErrNoRows,
		target:    downloadTarget(int64(len("abcdefghijklmnopqrstuvwxyz"))),
	}
	store.target.Visibility = "private"
	objects := &fakeObjectStore{
		data: []byte("abcdefghijklmnopqrstuvwxyz"),
	}
	app := fiber.New()
	app.Get("/content/:contentID/download", New(config.Config{}, store, objects).Public)

	req := httptest.NewRequest("GET", "/content/"+testContentID+"/download?access="+accessToken, nil)
	req.Header.Set("Range", "bytes=0-7")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusPartialContent {
		t.Fatalf("expected status %d, got %d", fiber.StatusPartialContent, resp.StatusCode)
	}
	if store.unlistedCalls != 1 {
		t.Fatalf("expected one unlisted target lookup, got %d", store.unlistedCalls)
	}
	if store.unlistedTokenHash != tokens.Hash(accessToken) {
		t.Fatalf("expected hashed access token lookup, got %q", store.unlistedTokenHash)
	}
	if got := resp.Header.Get("Content-Range"); got != "bytes 0-7/26" {
		t.Fatalf("expected Content-Range bytes 0-7/26, got %q", got)
	}
	if store.events != 1 {
		t.Fatalf("expected initial unlisted range request to record one download event, got %d", store.events)
	}
}

func TestPrivateDownloadWithoutAccessTokenStaysUnavailableThroughPublicEndpoint(t *testing.T) {
	store := &fakeStore{
		publicErr: pgx.ErrNoRows,
		target:    downloadTarget(int64(len("abcdefghijklmnopqrstuvwxyz"))),
	}
	store.target.Visibility = "private"
	objects := &fakeObjectStore{
		data: []byte("abcdefghijklmnopqrstuvwxyz"),
	}
	app := fiber.New()
	app.Get("/content/:contentID/download", New(config.Config{}, store, objects).Public)

	resp, err := app.Test(httptest.NewRequest("GET", "/content/"+testContentID+"/download", nil))
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("expected status %d, got %d", fiber.StatusNotFound, resp.StatusCode)
	}
	if store.unlistedCalls != 0 {
		t.Fatalf("expected no unlisted target lookup without access token, got %d", store.unlistedCalls)
	}
	if objects.fullReads != 0 || objects.rangeReads != 0 {
		t.Fatalf("expected no object reads, got full=%d range=%d", objects.fullReads, objects.rangeReads)
	}
}

func downloadTarget(size int64) downloaddomain.Target {
	return downloaddomain.Target{
		ContentID:  testContentID,
		FileID:     "22222222-2222-2222-2222-222222222222",
		AuthorID:   "33333333-3333-3333-3333-333333333333",
		Visibility: "public",
		Bucket:     "packages",
		StorageKey: "objects/test.bee",
		Filename:   "test.BEE",
		FileSize:   size,
	}
}

type fakeStore struct {
	target            downloaddomain.Target
	publicErr         error
	unlistedCalls     int
	unlistedTokenHash string
	events            int
}

func (s *fakeStore) GetPublicTarget(context.Context, string, string) (downloaddomain.Target, error) {
	if s.publicErr != nil {
		return downloaddomain.Target{}, s.publicErr
	}
	return s.target, nil
}

func (s *fakeStore) GetUnlistedTarget(_ context.Context, _ string, tokenHash string, _ string) (downloaddomain.Target, error) {
	s.unlistedCalls++
	s.unlistedTokenHash = tokenHash
	return s.target, nil
}

func (s *fakeStore) GetOwnerTarget(context.Context, string, string, string) (downloaddomain.Target, error) {
	return s.target, nil
}

func (s *fakeStore) RecordEvent(context.Context, downloaddomain.EventInput) error {
	s.events++
	return nil
}

type fakeObjectStore struct {
	data       []byte
	fullReads  int
	rangeReads int
	rangeStart int64
	rangeEnd   int64
}

func (s *fakeObjectStore) GetObject(context.Context, string, string) (io.ReadCloser, error) {
	s.fullReads++
	return io.NopCloser(bytes.NewReader(s.data)), nil
}

func (s *fakeObjectStore) GetObjectRange(_ context.Context, _ string, _ string, start int64, end int64) (io.ReadCloser, error) {
	s.rangeReads++
	s.rangeStart = start
	s.rangeEnd = end
	return io.NopCloser(bytes.NewReader(s.data[start : end+1])), nil
}
