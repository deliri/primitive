package process

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// ServiceRestart names the native service manager's restart agreement.
type ServiceRestart uint8

const (
	ServiceRestartUnknown ServiceRestart = iota
	ServiceRestartOnFailure
	ServiceRestartAlways
)

func (v ServiceRestart) IsValid() bool {
	return v == ServiceRestartOnFailure || v == ServiceRestartAlways
}
func (v ServiceRestart) Validate() error {
	if !v.IsValid() {
		return core.ErrProcessContract
	}
	return nil
}
func (ServiceRestart) OffWireEnum() {}
func (v ServiceRestart) String() string {
	switch v {
	case ServiceRestartOnFailure:
		return "on-failure"
	case ServiceRestartAlways:
		return "always"
	default:
		return core.UnknownEnumDiagnostic
	}
}

// ServiceDocumentRequest is a caller-owned user-service definition. This
// operation only encodes a document: it does not install, enable or start it.
// Darwin requires native stdout/stderr paths; Linux uses its journal.
type ServiceDocumentRequest struct {
	OperatingSystem             core.OperatingSystem
	Name                        core.PathComponent
	Command                     core.AbsolutePath
	Arguments                   []Argument
	Environment                 []EnvironmentVariable
	Stdout, Stderr              core.AbsolutePath
	Restart                     ServiceRestart
	RestartSeconds, StopSeconds uint32
}

func (r ServiceDocumentRequest) Validate() error {
	if err := errors.Join(r.Name.Validate(), r.Command.Validate(), r.Restart.Validate(), validateArguments(r.Arguments), (Environment{Mode: EnvironmentModeExact, Variables: r.Environment}).Validate()); err != nil {
		return errors.Join(core.ErrProcessContract, err)
	}
	if r.RestartSeconds == 0 || r.StopSeconds == 0 {
		return core.ErrProcessContract
	}
	switch r.OperatingSystem {
	case core.OperatingSystemLinux:
		if r.Stdout != (core.AbsolutePath{}) || r.Stderr != (core.AbsolutePath{}) {
			return core.ErrProcessContract
		}
	case core.OperatingSystemDarwin:
		if err := errors.Join(r.Stdout.Validate(), r.Stderr.Validate()); err != nil {
			return errors.Join(core.ErrProcessContract, err)
		}
	default:
		return core.ErrProcessUnsupported
	}
	for _, text := range []string{r.Name.String(), r.Command.String(), r.Stdout.String(), r.Stderr.String()} {
		if !serviceTextValid(text, r.OperatingSystem) {
			return core.ErrProcessContract
		}
	}
	for _, arg := range r.Arguments {
		if !serviceTextValid(arg.value, r.OperatingSystem) {
			return core.ErrProcessContract
		}
	}
	for _, variable := range r.Environment {
		if r.OperatingSystem == core.OperatingSystemLinux && !systemdEnvironmentNameValid(variable.Name.text()) {
			return core.ErrProcessContract
		}
		if !serviceTextValid(variable.Name.text(), r.OperatingSystem) || !serviceTextValid(variable.Value.text(), r.OperatingSystem) {
			return core.ErrProcessContract
		}
	}
	return nil
}

func systemdEnvironmentNameValid(name string) bool {
	for index, character := range name {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character == '_' || index > 0 && character >= '0' && character <= '9' {
			continue
		}
		return false
	}
	return name != ""
}

func serviceTextValid(text string, platform core.OperatingSystem) bool {
	if !utf8.ValidString(text) {
		return false
	}
	if platform != core.OperatingSystemDarwin {
		return true
	}
	for _, r := range text {
		if r != '\t' && r != '\n' && r != '\r' && (r < 0x20 || r > 0xD7FF && r < 0xE000 || r > 0xFFFD && r < 0x10000) {
			return false
		}
	}
	return true
}

// WriteServiceDocument encodes launchd XML with Go's XML encoder or systemd's
// documented C quoting and specifier escaping. No shell is introduced. Invalid
// requests produce no bytes; writer/native cancellation causes remain visible.
func WriteServiceDocument(ctx context.Context, destination io.Writer, r ServiceDocumentRequest) error {
	if err := errors.Join(contextstate.Validate(ctx), r.Validate()); err != nil {
		return err
	}
	if core.WriterIsNil(destination) {
		return core.ErrProcessContract
	}
	// Go's MultiWriter enforces complete writes, including a short nil write.
	destination = io.MultiWriter(destination)
	var err error
	switch r.OperatingSystem {
	case core.OperatingSystemDarwin:
		err = writeLaunchdDocument(destination, r)
	case core.OperatingSystemLinux:
		err = writeSystemdDocument(destination, r)
	default:
		return core.ErrProcessUnsupported
	}
	return errors.Join(err, contextstate.Validate(ctx))
}

func launchdText(destination io.Writer, key, value string) error {
	if _, err := io.WriteString(destination, "<key>"+key+"</key><string>"); err != nil {
		return err
	}
	if err := xml.EscapeText(destination, []byte(value)); err != nil {
		return err
	}
	_, err := io.WriteString(destination, "</string>\n")
	return err
}

func writeLaunchdDocument(destination io.Writer, r ServiceDocumentRequest) error {
	if _, err := io.WriteString(destination, xml.Header+"<plist version=\"1.0\"><dict>\n"); err != nil {
		return err
	}
	if err := launchdText(destination, "Label", r.Name.String()); err != nil {
		return err
	}
	if _, err := io.WriteString(destination, "<key>ProgramArguments</key><array>\n"); err != nil {
		return err
	}
	for index := 0; index <= len(r.Arguments); index++ {
		value := r.Command.String()
		if index > 0 {
			value = r.Arguments[index-1].value
		}
		if _, err := io.WriteString(destination, "<string>"); err != nil {
			return err
		}
		if err := xml.EscapeText(destination, []byte(value)); err != nil {
			return err
		}
		if _, err := io.WriteString(destination, "</string>\n"); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(destination, "</array><key>RunAtLoad</key><true/><key>KeepAlive</key>"); err != nil {
		return err
	}
	keepAlive := "<true/>"
	if r.Restart == ServiceRestartOnFailure {
		keepAlive = "<dict><key>SuccessfulExit</key><false/></dict>"
	}
	if _, err := fmt.Fprintf(destination, "%s<key>ThrottleInterval</key><integer>%d</integer><key>ExitTimeOut</key><integer>%d</integer>\n<key>EnvironmentVariables</key><dict>\n", keepAlive, r.RestartSeconds, r.StopSeconds); err != nil {
		return err
	}
	for _, variable := range r.Environment {
		if _, err := io.WriteString(destination, "<key>"); err != nil {
			return err
		}
		if err := xml.EscapeText(destination, []byte(variable.Name.text())); err != nil {
			return err
		}
		if _, err := io.WriteString(destination, "</key><string>"); err != nil {
			return err
		}
		if err := xml.EscapeText(destination, []byte(variable.Value.text())); err != nil {
			return err
		}
		if _, err := io.WriteString(destination, "</string>\n"); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(destination, "</dict>\n"); err != nil {
		return err
	}
	if err := launchdText(destination, "StandardOutPath", r.Stdout.String()); err != nil {
		return err
	}
	if err := launchdText(destination, "StandardErrorPath", r.Stderr.String()); err != nil {
		return err
	}
	_, err := io.WriteString(destination, "</dict></plist>\n")
	return err
}

// systemd.syntax defines these C escapes. %% protects literal specifiers.
// ExecStart's ':' prefix disables environment substitution; arguments remain
// exact even when they contain '$'. A standalone semicolon has its own escape.
// https://github.com/systemd/systemd/blob/main/man/systemd.service.xml
func systemdServiceWord(text string) string {
	if text == ";" {
		return `\;`
	}
	return strconv.Quote(strings.ReplaceAll(text, "%", "%%"))
}

func writeSystemdDocument(destination io.Writer, r ServiceDocumentRequest) error {
	if _, err := io.WriteString(destination, "[Unit]\nAfter=network-online.target\n\n[Service]\nType=exec\nExecStart=:"); err != nil {
		return err
	}
	if _, err := io.WriteString(destination, systemdServiceWord(r.Command.String())); err != nil {
		return err
	}
	for _, arg := range r.Arguments {
		if _, err := io.WriteString(destination, " "+systemdServiceWord(arg.value)); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(destination, "\n"); err != nil {
		return err
	}
	for _, variable := range r.Environment {
		if _, err := fmt.Fprintf(destination, "Environment=%s\n", systemdServiceWord(variable.Name.text()+"="+variable.Value.text())); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(destination, "Restart=%s\nRestartSec=%d\nTimeoutStopSec=%d\nKillMode=control-group\n\n[Install]\nWantedBy=default.target\n", r.Restart.String(), r.RestartSeconds, r.StopSeconds)
	return err
}

// QuotePOSIXArgument projects one exact argument into OpenSSH's remote-shell
// command grammar. It does not interpret or execute a shell program.
func QuotePOSIXArgument(value Argument) (Argument, error) {
	if err := value.Validate(); err != nil {
		return Argument{}, err
	}
	return NewArgument("'" + strings.ReplaceAll(value.value, "'", "'\"'\"'") + "'")
}
