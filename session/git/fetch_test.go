package git

import (
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"
	"time"
)

func newRepoWithRemote(t *testing.T, url string) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"remote", "add", "origin", url},
		{"config", "credential.helper", ""}, // keep the OS keychain out of the test
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

// A remote that asks for credentials must fail fast, not wait on a password prompt.
func TestFetchBranches_AuthRequiredRemoteDoesNotPrompt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", `Basic realm="test"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	t.Setenv("GIT_ASKPASS", "")
	t.Setenv("SSH_ASKPASS", "")

	dir := newRepoWithRemote(t, srv.URL+"/repo.git")
	start := time.Now()
	FetchBranches(dir)
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("FetchBranches took %v; it should fail fast instead of prompting", d)
	}
}

// A remote that never answers must not hang the fetch past fetchTimeout.
func TestFetchBranches_HungRemoteTimesOut(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	defer srv.Close()
	defer close(release)

	old := fetchTimeout
	fetchTimeout = 500 * time.Millisecond
	defer func() { fetchTimeout = old }()

	dir := newRepoWithRemote(t, srv.URL+"/repo.git")
	start := time.Now()
	FetchBranches(dir)
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("FetchBranches took %v; want it bounded by fetchTimeout", d)
	}
}
