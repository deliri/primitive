package gotoolchain

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestGoStringLiteralAgreesWithNativeGoValues(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ name, lexeme, want string }{
		{name: "empty interpreted string", lexeme: `""`},
		{name: "empty raw string", lexeme: "``"},
		{name: "ordinary SQL string", lexeme: `"SELECT "`, want: "SELECT "},
		{name: "quoted byte remains quote", lexeme: `"\""`, want: "\""},
		{name: "escaped slash remains slash", lexeme: `"\\"`, want: "\\"},
		{name: "newline byte", lexeme: `"\n"`, want: "\n"},
		{name: "tab byte", lexeme: `"\t"`, want: "\t"},
		{name: "backspace byte", lexeme: `"\b"`, want: "\b"},
		{name: "form feed byte", lexeme: `"\f"`, want: "\f"},
		{name: "vertical tab byte", lexeme: `"\v"`, want: "\v"},
		{name: "alert byte", lexeme: `"\a"`, want: "\a"},
		{name: "carriage return byte", lexeme: `"\r"`, want: "\r"},
		{name: "octal encoded SQL byte", lexeme: `"S\105LECT "`, want: "SELECT "},
		{name: "hexadecimal encoded SQL byte", lexeme: `"SE\x4cECT "`, want: "SELECT "},
		{name: "unicode encoded SQL byte", lexeme: `"SE\u004cECT "`, want: "SELECT "},
		{name: "long unicode encoded SQL byte", lexeme: `"SE\U0000004cECT "`, want: "SELECT "},
		{name: "escaped SQL separator", lexeme: `"SELECT\x20"`, want: "SELECT "},
		{name: "quoted escape spelling remains data", lexeme: `"SELECT\\x20"`, want: `SELECT\x20`},
		{name: "raw escape spelling remains data", lexeme: "`SELECT\\x20`", want: `SELECT\x20`},
		{name: "raw multiline value", lexeme: "`first\nsecond`", want: "first\nsecond"},
		{name: "raw carriage returns are omitted by Go", lexeme: "`first\r\nsecond`", want: "first\nsecond"},
		{name: "literal non ASCII text", lexeme: `"é界"`, want: "é界"},
		{name: "non ASCII BMP escape", lexeme: `"\u00e9"`, want: "é"},
		{name: "non BMP escape", lexeme: `"\U0001f642"`, want: "🙂"},
		{name: "nul byte is retained", lexeme: `"\x00"`, want: string([]byte{0})},
		{name: "largest ASCII byte", lexeme: `"\x7f"`, want: string([]byte{0x7f})},
		{name: "non UTF8 byte is retained", lexeme: `"\x80"`, want: string([]byte{0x80})},
		{name: "largest byte is retained", lexeme: `"\xff"`, want: string([]byte{0xff})},
		{name: "largest octal byte", lexeme: `"\377"`, want: string([]byte{0xff})},
		{name: "unicode before surrogate interval", lexeme: `"\ud7ff"`, want: string(rune(0xd7ff))},
		{name: "unicode after surrogate interval", lexeme: `"\ue000"`, want: string(rune(0xe000))},
		{name: "largest unicode value", lexeme: `"\U0010ffff"`, want: string(rune(0x10ffff))},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			file, err := parser.ParseFile(token.NewFileSet(), "subject.go", "package subject\nvar _ = "+row.lexeme+"\n", 0)
			if err != nil {
				t.Fatal(err)
			}
			literal := file.Decls[0].(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Values[0].(*ast.BasicLit)
			observed, err := ObserveGoStringLiteral(t.Context(), GoStringLiteralRequest{Literal: literal})
			native, nativeErr := types.Eval(token.NewFileSet(), nil, token.NoPos, row.lexeme)
			if nativeErr != nil || native.Value == nil || native.Value.Kind() != constant.String {
				t.Fatalf("native string value%+v/%v", native, nativeErr)
			}
			if err != nil || observed.Value != row.want || observed.Value != constant.StringVal(native.Value) {
				t.Fatalf("native string observation%q/%v want%q", observed.Value, err, row.want)
			}
		})
	}
}

func TestGoStringLiteralPreservesNativeSyntaxRefusal(t *testing.T) {
	t.Parallel()
	for _, lexeme := range []string{`"`, `"open`, `"\z"`, `"\x0"`, `"\xgg"`, `"\u000"`, `"\uD800"`, `"\uDFFF"`, `"\U0000000"`, `"\U00110000"`, `"\400"`, `"\777"`, `"x"tail`, "\"first\nsecond\"", "`open"} {
		t.Run(lexeme, func(t *testing.T) {
			t.Parallel()
			observed, err := ObserveGoStringLiteral(t.Context(), GoStringLiteralRequest{Literal: &ast.BasicLit{Kind: token.STRING, Value: lexeme}})
			if observed != (GoStringLiteralObservation{}) || !errors.Is(err, core.ErrGoToolchainOutput) || !errors.Is(err, strconv.ErrSyntax) {
				t.Fatalf("native string refusal%+v/%v", observed, err)
			}
		})
	}
}

func TestGoStringLiteralRefusesContradictoryNativeNodeAndContext(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name    string
		literal *ast.BasicLit
	}{
		{name: "missing native node"},
		{name: "rune token cannot borrow string identity", literal: &ast.BasicLit{Kind: token.CHAR, Value: `'x'`}},
		{name: "native string spelling cannot borrow rune token", literal: &ast.BasicLit{Kind: token.CHAR, Value: `"x"`}},
		{name: "integer token cannot borrow string identity", literal: &ast.BasicLit{Kind: token.INT, Value: `1`}},
		{name: "missing lexeme cannot be an empty string", literal: &ast.BasicLit{Kind: token.STRING}},
		{name: "rune lexeme cannot borrow string token", literal: &ast.BasicLit{Kind: token.STRING, Value: `'x'`}},
		{name: "unquoted identifier cannot borrow string token", literal: &ast.BasicLit{Kind: token.STRING, Value: `name`}},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			observed, err := ObserveGoStringLiteral(t.Context(), GoStringLiteralRequest{Literal: row.literal})
			if observed != (GoStringLiteralObservation{}) || !errors.Is(err, core.ErrGoToolchainContract) {
				t.Fatalf("contradictory native string%+v/%v", observed, err)
			}
		})
	}
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(io.ErrClosedPipe)
	for _, owner := range []struct {
		name string
		ctx  context.Context
		want error
	}{{name: "missing native context", want: core.ErrGoToolchainContract}, {name: "native custom cause", ctx: ctx, want: io.ErrClosedPipe}} {
		t.Run(owner.name, func(t *testing.T) {
			t.Parallel()
			observed, err := ObserveGoStringLiteral(owner.ctx, GoStringLiteralRequest{Literal: &ast.BasicLit{Kind: token.STRING, Value: `"data"`}})
			if observed != (GoStringLiteralObservation{}) || !errors.Is(err, owner.want) {
				t.Fatalf("native string context%+v/%v want%v", observed, err, owner.want)
			}
		})
	}
}

func TestGoStringLiteralHasNoDecodedExtentQuota(t *testing.T) {
	t.Parallel()
	for _, extent := range []int{(32 << 10) - 1, 32 << 10, (32 << 10) + 1, (64 << 10) - 1, 64 << 10, (64 << 10) + 1, 8 << 20} {
		t.Run(fmt.Sprint(extent), func(t *testing.T) {
			t.Parallel()
			want := strings.Repeat("x", extent)
			observed, err := ObserveGoStringLiteral(t.Context(), GoStringLiteralRequest{Literal: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(want)}})
			if err != nil || observed.Value != want {
				t.Fatalf("native string extent%d got%d/%v", extent, len(observed.Value), err)
			}
		})
	}
}

func TestGoStringLiteralWithholdsDecodedValueOnCancellation(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, lexeme  string
		syntaxRefusal bool
	}{{name: "decoded value is withheld", lexeme: `"private source bytes"`}, {name: "native syntax and cancellation are retained", lexeme: `"\z"`, syntaxRefusal: true}} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			parent, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			ctx := &goBuildConstraintCancellationContext{Context: parent, cancel: cancel}
			observed, err := ObserveGoStringLiteral(ctx, GoStringLiteralRequest{Literal: &ast.BasicLit{Kind: token.STRING, Value: row.lexeme}})
			if observed != (GoStringLiteralObservation{}) || !errors.Is(err, io.ErrClosedPipe) || !errors.Is(err, context.Canceled) {
				t.Fatalf("terminal native string%+v/%v", observed, err)
			}
			if row.syntaxRefusal && !errors.Is(err, strconv.ErrSyntax) {
				t.Fatalf("terminal native string lost syntax refusal%v", err)
			}
		})
	}
}

func FuzzGoStringLiteralConservesNativeDecodedBytes(f *testing.F) {
	for _, value := range []string{"", "SELECT ", "é界", string([]byte{0, 0xff}), "quoted \\n text"} {
		native := constant.MakeString(value)
		f.Add(native.ExactString())
	}
	f.Add(`"\uD800"`)
	f.Add(`"\z"`)
	f.Fuzz(func(t *testing.T, lexeme string) {
		request := GoStringLiteralRequest{Literal: &ast.BasicLit{Kind: token.STRING, Value: lexeme}}
		observed, err := ObserveGoStringLiteral(t.Context(), request)
		native := constant.MakeFromLiteral(lexeme, token.STRING, 0)
		if err != nil {
			if native.Kind() == constant.String && len(lexeme) > 0 && (lexeme[0] == '"' || lexeme[0] == '`') {
				t.Fatalf("native Go string was incorrectly refused%q/%v", lexeme, err)
			}
			want := core.ErrGoToolchainContract
			if len(lexeme) > 0 && (lexeme[0] == '"' || lexeme[0] == '`') {
				want = core.ErrGoToolchainOutput
			}
			if observed != (GoStringLiteralObservation{}) || !errors.Is(err, want) {
				t.Fatalf("native string rejected value%+v/%v want%v", observed, err, want)
			}
			if want == core.ErrGoToolchainOutput && !errors.Is(err, strconv.ErrSyntax) {
				t.Fatalf("native string lost syntax cause%v", err)
			}
			return
		}
		if native.Kind() != constant.String || observed.Value != constant.StringVal(native) {
			t.Fatalf("native string accepted%q want native%s", observed.Value, native)
		}
		canonical := constant.MakeString(observed.Value).ExactString()
		second, secondErr := ObserveGoStringLiteral(t.Context(), GoStringLiteralRequest{Literal: &ast.BasicLit{Kind: token.STRING, Value: canonical}})
		if secondErr != nil || second != observed {
			t.Fatalf("native string byte roundtrip%+v/%v want%+v", second, secondErr, observed)
		}
	})
}
