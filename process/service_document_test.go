package process

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func serviceDocumentFixture(t *testing.T, platform core.OperatingSystem) ServiceDocumentRequest {
	t.Helper()
	name, err := core.ParsePathComponent("com.example.worker")
	if err != nil {
		t.Fatal(err)
	}
	command, err := core.ParseAbsolutePath("/private/path with % and $/worker")
	if err != nil {
		t.Fatal(err)
	}
	args, err := ParseArguments([]string{"--config", "/private/config ' \" & <> % $\\name", "", ";", "literal\nnewline"})
	if err != nil {
		t.Fatal(err)
	}
	env, err := ParseExactEnvironment([]string{"PATH=/usr/bin:/bin", "GOMAXPROCS=1"})
	if err != nil {
		t.Fatal(err)
	}
	r := ServiceDocumentRequest{OperatingSystem: platform, Name: name, Command: command, Arguments: args, Environment: env.Variables, Restart: ServiceRestartAlways, RestartSeconds: 3, StopSeconds: 20}
	if platform == core.OperatingSystemDarwin {
		r.Stdout, err = core.ParseAbsolutePath("/private/log path/out&.log")
		if err != nil {
			t.Fatal(err)
		}
		r.Stderr, err = core.ParseAbsolutePath("/private/log path/error.log")
		if err != nil {
			t.Fatal(err)
		}
	}
	if err = r.Validate(); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestServiceDocumentLayerTriad(t *testing.T) {
	t.Parallel()
	for _, platform := range []core.OperatingSystem{core.OperatingSystemDarwin, core.OperatingSystemLinux} {
		t.Run(platform.String(), func(t *testing.T) {
			t.Parallel()
			r := serviceDocumentFixture(t, platform)
			var output bytes.Buffer
			if err := WriteServiceDocument(t.Context(), &output, r); err != nil {
				t.Fatal(err)
			}
			if platform == core.OperatingSystemDarwin {
				decoder := xml.NewDecoder(bytes.NewReader(output.Bytes()))
				var stringsSeen []string
				for {
					token, err := decoder.Token()
					if err == io.EOF {
						break
					}
					if err != nil {
						t.Fatal(err)
					}
					start, ok := token.(xml.StartElement)
					if !ok || start.Name.Local != "string" {
						continue
					}
					var value string
					if err = decoder.DecodeElement(&value, &start); err != nil {
						t.Fatal(err)
					}
					stringsSeen = append(stringsSeen, value)
				}
				want := []string{r.Name.String(), r.Command.String()}
				for _, arg := range r.Arguments {
					want = append(want, arg.value)
				}
				for _, variable := range r.Environment {
					want = append(want, variable.Value.text())
				}
				want = append(want, r.Stdout.String(), r.Stderr.String())
				if !slices.Equal(stringsSeen, want) {
					t.Fatalf("XML round trip=%q, want %q", stringsSeen, want)
				}
			} else {
				for _, fragment := range []string{`ExecStart=:"/private/path with %% and $/worker"`, `"/private/config ' \" & <> %% $\\name"`, ` "" \; "literal\nnewline"`, "KillMode=control-group\n", "Type=exec\n"} {
					if !strings.Contains(output.String(), fragment) {
						t.Fatalf("systemd projection=%q, want exact native fragment %q", output.String(), fragment)
					}
				}
			}
			for _, mutate := range []func(*ServiceDocumentRequest){func(v *ServiceDocumentRequest) { v.Command = core.AbsolutePath{} }, func(v *ServiceDocumentRequest) { v.Restart = ServiceRestartUnknown }, func(v *ServiceDocumentRequest) { v.StopSeconds = 0 }, func(v *ServiceDocumentRequest) { v.Arguments = []Argument{{}} }} {
				changed := r
				mutate(&changed)
				output.Reset()
				err := WriteServiceDocument(t.Context(), &output, changed)
				if !errors.Is(err, core.ErrProcessContract) || output.Len() != 0 {
					t.Fatalf("invalid document=(%q,%v), want no output and contract refusal", output.String(), err)
				}
			}
			r.Arguments = nil
			r.Environment = nil
			output.Reset()
			if err := WriteServiceDocument(t.Context(), &output, r); err != nil || output.Len() == 0 {
				t.Fatalf("empty argv/environment=(%q,%v), want command-only native service", output.String(), err)
			}
		})
	}
}

// Native Apple parsing proves that generated XML is an actual plist, without
// registering a launch agent or starting a service.
func TestLaunchdDocumentNativeParser(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "darwin" {
		t.Skip("Apple plist parser requires Darwin")
	}
	directory := t.TempDir()
	r := serviceDocumentFixture(t, core.OperatingSystemDarwin)
	var output bytes.Buffer
	if err := WriteServiceDocument(t.Context(), &output, r); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "worker.plist")
	if err := os.WriteFile(path, output.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	json, err := exec.CommandContext(t.Context(), "/usr/bin/plutil", "-convert", "json", "-o", "-", "--", path).Output()
	if err != nil {
		t.Fatalf("native plist parser=%v, source=%q", err, output.String())
	}
	if !bytes.Contains(json, []byte(`"ProgramArguments"`)) || !bytes.Contains(json, []byte(`"GOMAXPROCS":"1"`)) {
		t.Fatalf("native plist JSON=%s, want argv and exact worker budget", json)
	}
}

func TestPOSIXArgumentNativeRoundTrip(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"", "plain", "space separated", "apostrophe's", `double " quotes`, `literal $HOME`, `backticks ` + "`" + `printf surprise` + "`", `$(printf surprise)`, "; printf surprise", "literal\nnewline", "雪", "\\"} {
		name := text
		if name == "" {
			name = "empty argument remains empty"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			arg, err := NewArgument(text)
			if err != nil {
				t.Fatal(err)
			}
			quoted, err := QuotePOSIXArgument(arg)
			if err != nil {
				t.Fatal(err)
			}
			command := exec.CommandContext(t.Context(), "/bin/sh", "-c", "printf '%s' "+quoted.value)
			command.Dir = directory
			command.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + directory}
			got, err := command.Output()
			if err != nil || string(got) != text {
				t.Fatalf("native quoted argument=(%q,%v), want %q", got, err, text)
			}
		})
	}
}

func TestServiceRestartDomainExhaustive(t *testing.T) {
	t.Parallel()
	for value := 0; value <= 255; value++ {
		restart := ServiceRestart(value)
		err := restart.Validate()
		valid := restart == ServiceRestartOnFailure || restart == ServiceRestartAlways
		if valid {
			if err != nil || !restart.IsValid() || restart.String() == core.UnknownEnumDiagnostic {
				t.Fatalf("restart %d=(%q,%v), want admitted", value, restart.String(), err)
			}
		} else if !errors.Is(err, core.ErrProcessContract) || restart.IsValid() || restart.String() != core.UnknownEnumDiagnostic {
			t.Fatalf("restart %d=(%q,%v), want refusal", value, restart.String(), err)
		}
	}
}

type serviceBrokenWriter struct{}

func (serviceBrokenWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

type serviceShortWriter struct{}

func (serviceShortWriter) Write(data []byte) (int, error) { return len(data) - 1, nil }

func TestServiceEncodingRefusesUnrepresentableTextBeforeOutput(t *testing.T) {
	t.Parallel()
	r := serviceDocumentFixture(t, core.OperatingSystemDarwin)
	arg, err := NewArgument("\x01")
	if err != nil {
		t.Fatal(err)
	}
	r.Arguments = []Argument{arg}
	var output bytes.Buffer
	if err = WriteServiceDocument(t.Context(), &output, r); !errors.Is(err, core.ErrProcessContract) || output.Len() != 0 {
		t.Fatalf("unrepresentable plist=(%q,%v), want no output", output.String(), err)
	}
	r = serviceDocumentFixture(t, core.OperatingSystemDarwin)
	if err = WriteServiceDocument(t.Context(), serviceBrokenWriter{}, r); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("broken service sink=%v, want original cause", err)
	}
	if err = WriteServiceDocument(t.Context(), serviceShortWriter{}, r); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short nil write=%v, want native short-write cause", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err = WriteServiceDocument(ctx, &output, r); !errors.Is(err, context.Canceled) || output.Len() != 0 {
		t.Fatalf("cancelled service=(%q,%v), want no output", output.String(), err)
	}
}

func TestSystemdEnvironmentNameBoundary(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		wantErr error
	}{
		{"PATH", nil}, {"_private2", nil}, {"2FIRST", core.ErrProcessContract}, {"has space", core.ErrProcessContract}, {"has-percent%", core.ErrProcessContract}, {"雪", core.ErrProcessContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := serviceDocumentFixture(t, core.OperatingSystemLinux)
			variable, err := ParseExactEnvironment([]string{tc.name + "=literal"})
			if err != nil {
				t.Fatal(err)
			}
			r.Environment = variable.Variables
			var output bytes.Buffer
			err = WriteServiceDocument(t.Context(), &output, r)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || output.Len() != 0 {
					t.Fatalf("environment %q=(%q,%v), want no output and refusal", tc.name, output.String(), err)
				}
				return
			}
			if err != nil || output.Len() == 0 {
				t.Fatalf("environment %q=(%q,%v), want native service", tc.name, output.String(), err)
			}
		})
	}
}

func FuzzServiceDocumentProjection(f *testing.F) {
	name, err := core.ParsePathComponent("com.example.worker")
	if err != nil {
		f.Fatal(err)
	}
	command, err := core.ParseAbsolutePath("/usr/bin/worker")
	if err != nil {
		f.Fatal(err)
	}
	stdout, err := core.ParseAbsolutePath("/private/out.log")
	if err != nil {
		f.Fatal(err)
	}
	stderr, err := core.ParseAbsolutePath("/private/err.log")
	if err != nil {
		f.Fatal(err)
	}
	arg, err := NewArgument("literal $ % ' \" \\ snow 雪")
	if err != nil {
		f.Fatal(err)
	}
	seed := ServiceDocumentRequest{OperatingSystem: core.OperatingSystemDarwin, Name: name, Command: command, Arguments: []Argument{arg}, Stdout: stdout, Stderr: stderr, Restart: ServiceRestartAlways, RestartSeconds: 3, StopSeconds: 20}
	if err = seed.Validate(); err != nil {
		f.Fatal(err)
	}
	f.Add(arg.value, uint8(seed.OperatingSystem), uint8(seed.Restart))
	f.Add("", uint8(core.OperatingSystemLinux), uint8(ServiceRestartOnFailure))
	f.Add("", uint8(255), uint8(255))
	f.Fuzz(func(t *testing.T, text string, platform, restart uint8) {
		arg, err := NewArgument(text)
		if err != nil {
			return
		}
		r := seed
		r.OperatingSystem = core.OperatingSystem(platform)
		r.Restart = ServiceRestart(restart)
		r.Arguments = []Argument{arg}
		if r.OperatingSystem == core.OperatingSystemLinux {
			r.Stdout = core.AbsolutePath{}
			r.Stderr = core.AbsolutePath{}
		}
		var output bytes.Buffer
		err = WriteServiceDocument(t.Context(), &output, r)
		if err != nil {
			if !errors.Is(err, core.ErrProcessContract) && !errors.Is(err, core.ErrProcessUnsupported) || output.Len() != 0 {
				t.Fatalf("refused service=(%q,%v), want no output", output.String(), err)
			}
			return
		}
		if err = r.Validate(); err != nil {
			t.Fatalf("accepted invalid service: %v", err)
		}
		var second bytes.Buffer
		if err = WriteServiceDocument(t.Context(), &second, r); err != nil || !bytes.Equal(output.Bytes(), second.Bytes()) {
			t.Fatalf("service projection is not canonical: %v", err)
		}
		if r.OperatingSystem == core.OperatingSystemDarwin {
			decoder := xml.NewDecoder(bytes.NewReader(output.Bytes()))
			var values []string
			for {
				token, err := decoder.Token()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				start, ok := token.(xml.StartElement)
				if !ok || start.Name.Local != "string" {
					continue
				}
				var value string
				if err = decoder.DecodeElement(&value, &start); err != nil {
					t.Fatal(err)
				}
				values = append(values, value)
			}
			want := []string{name.String(), command.String(), text, stdout.String(), stderr.String()}
			if !slices.Equal(values, want) {
				t.Fatalf("XML projection=%q, want exact %q", values, want)
			}
		} else {
			_, tail, found := strings.Cut(output.String(), "ExecStart=:\"/usr/bin/worker\" ")
			if !found {
				t.Fatal("systemd executable or literal-mode prefix changed")
			}
			encoded, _, found := strings.Cut(tail, "\n")
			if !found {
				t.Fatal("systemd command lacks termination")
			}
			decoded := ";"
			if encoded != `\;` {
				decoded, err = strconv.Unquote(encoded)
				if err != nil {
					t.Fatal(err)
				}
			}
			var restored strings.Builder
			for index := 0; index < len(decoded); index++ {
				if decoded[index] == '%' {
					index++
					if index >= len(decoded) || decoded[index] != '%' {
						t.Fatal("unescaped systemd specifier")
					}
				}
				restored.WriteByte(decoded[index])
			}
			if restored.String() != text {
				t.Fatalf("systemd argument round trip=%q, want %q", restored.String(), text)
			}
		}
	})
}

func FuzzPOSIXArgumentSemanticClosure(f *testing.F) {
	for _, text := range []string{"", "plain", "apostrophe's", `$HOME;"\\`, "雪"} {
		arg, err := NewArgument(text)
		if err != nil {
			f.Fatal(err)
		}
		if _, err = QuotePOSIXArgument(arg); err != nil {
			f.Fatal(err)
		}
		f.Add(text)
	}
	f.Fuzz(func(t *testing.T, text string) {
		arg, err := NewArgument(text)
		if err != nil {
			return
		}
		quoted, err := QuotePOSIXArgument(arg)
		if err != nil {
			if !errors.Is(err, core.ErrProcessContract) || quoted != (Argument{}) {
				t.Fatalf("refused quoted argument=(%v,%v)", quoted, err)
			}
			return
		}
		// Independent single-word shell lexer for the encoder's quoted subset.
		var decoded strings.Builder
		var state byte
		for index := 0; index < len(quoted.value); index++ {
			character := quoted.value[index]
			switch state {
			case 0:
				switch character {
				case '\'', '"':
					state = character
				default:
					t.Fatalf("unquoted shell byte %q", character)
				}
			case '\'':
				if character == '\'' {
					state = 0
				} else {
					decoded.WriteByte(character)
				}
			case '"':
				if character == '"' {
					state = 0
				} else {
					if character != '\'' {
						t.Fatalf("unexpected double-quoted expansion byte %q", character)
					}
					decoded.WriteByte(character)
				}
			}
		}
		if state != 0 || decoded.String() != text {
			t.Fatalf("quote round trip=(%q,%d), want %q and closed word", decoded.String(), state, text)
		}
	})
}
