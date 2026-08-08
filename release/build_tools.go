package release

import (
	"bytes"
	"context"
	"debug/buildinfo"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/garble"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	buildToolExecutableMaximumBytes = 128 << 20
	buildToolProbeOutputBytes       = 4 << 10
	goCommandModulePath             = "cmd/go"
	goVersionArgument               = "version"
)

// BuildToolVerificationRequest identifies the exact local executables and
// bounded process conditions used to prove them.
type BuildToolVerificationRequest struct {
	GoExecutable     core.AbsolutePath
	GarbleExecutable core.AbsolutePath
	WorkingDirectory core.AbsolutePath
	HostEnvironment  process.Environment
	WaitDelay        temporal.Duration
}

// Validate proves the complete verification boundary before file or process I/O.
func (r BuildToolVerificationRequest) Validate() error {
	for _, err := range []error{
		r.GoExecutable.Validate(), r.GarbleExecutable.Validate(),
		r.WorkingDirectory.Validate(), r.HostEnvironment.Validate(), r.WaitDelay.Validate(),
	} {
		if err != nil {
			return contractError(errors.New("build tool verification request is invalid"), err)
		}
	}
	values, err := r.HostEnvironment.Strings()
	if err != nil {
		return contractError(errors.New("build tool verification environment is invalid"), err)
	}
	if values == nil {
		return contractError(errors.New("build tool verification environment inherits ambient state"))
	}
	if r.WaitDelay.IsZero() {
		return contractError(errors.New("build tool verification wait delay is zero"))
	}
	return nil
}

// VerifiedBuildTools is proof that exact on-disk Go and Garble executables
// matched every compiler-owned identity at one observation.
type VerifiedBuildTools struct {
	goExecutable           core.AbsolutePath
	garbleExecutable       core.AbsolutePath
	goExecutableDigest     core.SHA256Digest
	garbleExecutableDigest core.SHA256Digest
	hostPlatform           core.Platform
	goToolchain            GoToolchainIdentity
	garbleTool             garble.ToolIdentity
	valid                  bool
}

// Validate proves the retained tool identities and observation seal.
func (v VerifiedBuildTools) Validate() error {
	if !v.valid {
		return contractError(errors.New("verified build tools are unset"))
	}
	for _, err := range []error{
		v.goExecutableDigest.Validate(), v.garbleExecutableDigest.Validate(),
		v.goExecutable.Validate(), v.garbleExecutable.Validate(), v.hostPlatform.Validate(),
		v.goToolchain.Validate(), v.garbleTool.Validate(),
	} {
		if err != nil {
			return contractError(errors.New("verified build tools are invalid"), err)
		}
	}
	if v.goToolchain != CurrentGoToolchain() || v.garbleTool != garble.CurrentTool() {
		return contractError(errors.New("verified build tools differ from the current release tools"))
	}
	return nil
}

// Accessors return the exact verified tool facts.
func (v VerifiedBuildTools) GoExecutable() core.AbsolutePath       { return v.goExecutable }
func (v VerifiedBuildTools) GarbleExecutable() core.AbsolutePath   { return v.garbleExecutable }
func (v VerifiedBuildTools) HostPlatform() core.Platform           { return v.hostPlatform }
func (v VerifiedBuildTools) GoToolchain() GoToolchainIdentity      { return v.goToolchain }
func (v VerifiedBuildTools) GarbleTool() garble.ToolIdentity       { return v.garbleTool }
func (v VerifiedBuildTools) GoExecutableDigest() core.SHA256Digest { return v.goExecutableDigest }
func (v VerifiedBuildTools) GarbleExecutableDigest() core.SHA256Digest {
	return v.garbleExecutableDigest
}

// VerifyBuildTools inspects both executable files and executes the selected Go
// command with bounded output. Operator-supplied version strings are never
// accepted as evidence.
func VerifyBuildTools(
	ctx context.Context,
	request BuildToolVerificationRequest,
) (VerifiedBuildTools, error) {
	if ctx == nil {
		return VerifiedBuildTools{}, contractError(core.ErrNilContext)
	}
	if err := ctx.Err(); err != nil {
		return VerifiedBuildTools{}, contractError(err)
	}
	if err := request.Validate(); err != nil {
		return VerifiedBuildTools{}, err
	}
	if err := validatePackageCapability(); err != nil {
		return VerifiedBuildTools{}, err
	}
	goToolchain := CurrentGoToolchain()
	goDigest, hostPlatform, err := verifyGoTool(ctx, request, goToolchain)
	if err != nil {
		return VerifiedBuildTools{}, err
	}
	garbleTool := garble.CurrentTool()
	garbleDigest, err := verifyGarbleTool(request.GarbleExecutable, garbleTool, goToolchain)
	if err != nil {
		return VerifiedBuildTools{}, err
	}
	verified := VerifiedBuildTools{
		goExecutableDigest: goDigest, garbleExecutableDigest: garbleDigest,
		goExecutable: request.GoExecutable, garbleExecutable: request.GarbleExecutable,
		hostPlatform: hostPlatform, goToolchain: goToolchain,
		garbleTool: garbleTool, valid: true,
	}
	if err := verified.Validate(); err != nil {
		return VerifiedBuildTools{}, err
	}
	return verified, nil
}

func verifyGoTool(
	ctx context.Context,
	request BuildToolVerificationRequest,
	toolchain GoToolchainIdentity,
) (core.SHA256Digest, core.Platform, error) {
	info, digest, err := inspectBuildTool(request.GoExecutable)
	if err != nil {
		return core.SHA256Digest{}, core.Platform{}, err
	}
	version, err := toolchain.Version()
	if err != nil || info.Path != goCommandModulePath || info.GoVersion != version {
		return core.SHA256Digest{}, core.Platform{}, contractError(
			errors.New("go executable identity differs from the admitted toolchain"), err,
		)
	}
	platform, err := probeGoVersion(ctx, request, version)
	if err != nil {
		return core.SHA256Digest{}, core.Platform{}, err
	}
	return digest, platform, nil
}

func verifyGarbleTool(
	path core.AbsolutePath,
	tool garble.ToolIdentity,
	goToolchain GoToolchainIdentity,
) (core.SHA256Digest, error) {
	info, digest, err := inspectBuildTool(path)
	if err != nil {
		return core.SHA256Digest{}, err
	}
	goVersion, err := goToolchain.Version()
	if err != nil {
		return core.SHA256Digest{}, err
	}
	if err := validateGarbleBuildInfo(info, tool, goVersion); err != nil {
		return core.SHA256Digest{}, err
	}
	return digest, nil
}

func inspectBuildTool(path core.AbsolutePath) (_ *buildinfo.BuildInfo, digest core.SHA256Digest, resultErr error) {
	file, err := os.Open(path.String()) // #nosec G304 -- validated absolute tool capability is the inspected object.
	if err != nil {
		return nil, core.SHA256Digest{}, contractError(errors.New("open build tool executable"), err)
	}
	defer func() {
		resultErr = errors.Join(resultErr, file.Close())
	}()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > buildToolExecutableMaximumBytes {
		return nil, core.SHA256Digest{}, contractError(errors.New("build tool executable is not a bounded regular file"), err)
	}
	build, err := buildinfo.Read(file)
	if err != nil {
		return nil, core.SHA256Digest{}, contractError(errors.New("read build tool identity"), err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, core.SHA256Digest{}, contractError(errors.New("rewind build tool executable"), err)
	}
	writer := core.NewDigestWriter()
	buffer := make([]byte, 64<<10)
	if _, err := io.CopyBuffer(writer, file, buffer); err != nil {
		return nil, core.SHA256Digest{}, contractError(errors.New("digest build tool executable"), err)
	}
	toolDigest, _, err := writer.Seal()
	if err != nil {
		return nil, core.SHA256Digest{}, contractError(errors.New("digest build tool executable"), err)
	}
	return build, toolDigest, nil
}

func probeGoVersion(
	ctx context.Context,
	request BuildToolVerificationRequest,
	wantVersion string,
) (core.Platform, error) {
	arguments, err := process.ParseArguments([]string{goVersionArgument})
	if err != nil {
		return core.Platform{}, contractError(err)
	}
	limit, err := core.NewByteCount(buildToolProbeOutputBytes)
	if err != nil {
		return core.Platform{}, contractError(err)
	}
	var stdout, stderr bytes.Buffer
	result, err := process.Run(ctx, process.Request{
		Streams: process.Streams{Stdin: bytes.NewReader(nil), Stdout: &stdout, Stderr: &stderr},
		Command: request.GoExecutable, WorkingDirectory: request.WorkingDirectory,
		Arguments: arguments, Environment: request.HostEnvironment,
		OutputLimit: limit, WaitDelay: request.WaitDelay,
		Containment: process.Containment{
			Isolation:    process.IsolationDirect,
			CancelSignal: process.CancelSignalKill,
		},
	})
	if err != nil {
		return core.Platform{}, contractError(errors.New("execute go version probe"), err)
	}
	exit, err := result.ExitCode()
	if err != nil {
		return core.Platform{}, contractError(err)
	}
	success, err := exit.Success()
	if err != nil || !success || stderr.Len() != 0 {
		return core.Platform{}, contractError(errors.New("go version probe did not exit cleanly"), err)
	}
	return parseGoVersionOutput(stdout.String(), wantVersion)
}

func parseGoVersionOutput(output, wantVersion string) (core.Platform, error) {
	platformText, err := parseGoVersionLine(output, wantVersion)
	if err != nil {
		return core.Platform{}, err
	}
	var platform core.Platform
	if err := platform.UnmarshalText([]byte(strings.Replace(platformText, "/", "-", 1))); err != nil {
		return core.Platform{}, contractError(errors.New("go version host platform is invalid"), err)
	}
	return platform, nil
}

func parseGoVersionLine(output, wantVersion string) (string, error) {
	line, ok := strings.CutSuffix(output, "\n")
	if !ok {
		return "", contractError(errors.New("go version output is not canonical"))
	}
	fields := strings.Split(line, " ")
	if !goVersionTokensMatch(fields, wantVersion) {
		return "", contractError(errors.New("go version output differs from the admitted toolchain"))
	}
	if strings.Count(fields[3], "/") != 1 {
		return "", contractError(errors.New("go version host platform is malformed"))
	}
	return fields[3], nil
}

func goVersionTokensMatch(fields []string, wantVersion string) bool {
	return len(fields) == 4 && fields[0] == "go" && fields[1] == "version" &&
		fields[2] == wantVersion
}

func validateGarbleBuildInfo(info *buildinfo.BuildInfo, tool garble.ToolIdentity, goVersion string) error {
	module, moduleErr := tool.ModulePath()
	version, versionErr := tool.Version()
	sum, sumErr := tool.ModuleSum()
	if err := errors.Join(moduleErr, versionErr, sumErr); err != nil {
		return err
	}
	if info.Path != module || info.Main.Path != module || info.Main.Version != version ||
		info.Main.Sum != sum || info.GoVersion != goVersion {
		return contractError(errors.New("garble executable identity differs from the admitted tool"))
	}
	return nil
}
