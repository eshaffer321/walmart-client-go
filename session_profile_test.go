package walmart

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eshaffer321/walmart-client-go/v2/internal/cookies"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testCapturedPlatform  = "usweb-1.284.0-test"
	testCapturedUserAgent = "Mozilla/5.0 Chrome/149.0.0.0"
)

func TestParseCurlCapturesRequestProfileWithoutCookieHeader(t *testing.T) {
	curlCmd := "curl 'https://www.walmart.com/orchestra/orders/graphql/getOrder/" +
		defaultGetOrderHash + "?variables=%7B%7D' \\\n" +
		"  -H 'user-agent: Mozilla/5.0 Chrome/149.0.0.0' \\\n" +
		"  -H 'sec-ch-ua: \"Chromium\";v=\"149\"' \\\n" +
		"  -H 'x-o-platform-version: usweb-1.284.0-test' \\\n" +
		"  -H 'cookie: CID=abc; SPID=xyz; auth=token'"

	capture := cookies.ParseCurl(curlCmd)

	assert.Contains(t, capture.URL, "/getOrder/"+defaultGetOrderHash)
	assert.Equal(t, testCapturedUserAgent, capture.Headers[headerUserAgent])
	assert.Equal(t, `"Chromium";v="149"`, capture.Headers["sec-ch-ua"])
	assert.Equal(t, testCapturedPlatform, capture.Headers[headerPlatformVersion])
	assert.Equal(t, map[string]string{cookieNameCID: "abc", cookieNameSPID: "xyz", cookieNameAuth: "token"}, capture.Cookies)
	assert.NotContains(t, capture.Headers, "cookie", "cookie credentials must not be retained as profile headers")
}

func TestCookieStoreReplacePersistsProfileAndResecuresFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cookies.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"cookies":{}}`), 0644))
	require.NoError(t, os.Chmod(path, 0644))

	store := cookies.NewStore(path, nil)
	store.Set("stale", &cookies.Cookie{Value: "old"})
	profile := cookies.RequestProfile{
		GetOrderHash: defaultGetOrderHash,
		Headers: map[string]string{
			headerUserAgent:       testCapturedUserAgent,
			headerPlatformVersion: testCapturedPlatform,
		},
	}
	store.Replace(map[string]*cookies.Cookie{
		cookieNameCID: {Value: "new", Essential: true},
	}, profile)
	require.NoError(t, store.Save())

	loaded := cookies.NewStore(path, nil)
	require.NoError(t, loaded.Load())
	assert.Nil(t, loaded.Get("stale"))
	assert.Equal(t, "new", loaded.Get(cookieNameCID).Value)
	assert.Equal(t, profile, loaded.GetRequestProfile())

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}
