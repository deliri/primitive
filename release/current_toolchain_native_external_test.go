package release_test

import (
	"runtime"
	"testing"

	"github.com/deliri/primitive/v2026/release"
)

// This follows the actual worker's Go executable, not a requested platform or
// a version-printing fixture. Parsing, standing, digest and the version probe
// all run through VerifyBuildTools before the receipt is admitted.
func TestCurrentGoToolchainAdmitsTheInstalledNativeExecutable(t *testing.T) {
	t.Parallel()
	request := buildToolVerificationRequestForLiveTest(t)
	verified, err := release.VerifyBuildTools(t.Context(), request)
	if err != nil {
		t.Fatalf("native Go admission=%v, want the installed reviewed compiler", err)
	}
	if err := verified.Validate(); err != nil {
		t.Fatal(err)
	}
	version, err := verified.GoToolchain().Version()
	if err != nil || version != "go1.27.3" {
		t.Fatalf("verified compiler=%q/%v, want go1.27.3", version, err)
	}
	if verified.HostPlatform().String() != runtime.GOOS+"-"+runtime.GOARCH {
		t.Fatalf("verified platform=%v, want actual native %s-%s", verified.HostPlatform(), runtime.GOOS, runtime.GOARCH)
	}
	if err := verified.GoExecutableDigest().Validate(); err != nil {
		t.Fatalf("native executable digest=%v, want verified bytes", err)
	}
}
