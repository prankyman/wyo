package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/prankyman/wyo/internal/version"
)

func writeFile(t *testing.T, path, text string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestTheCommandLineIsReadAsTheTableSays(t *testing.T) {
	t.Parallel()

	texts := defaultTexts()
	cases := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{name: "help lists every command", args: []string{"--help"}, wantStdout: "wyo owner reset-password"},
		{name: "help as a word", args: []string{"help"}, wantStdout: "wyo doctor"},
		{name: "the version as a command", args: []string{"version"}, wantStdout: version.Version},
		{name: "the version as a flag", args: []string{"--version"}, wantStdout: version.Version},
		{
			name: "an unknown command", args: []string{"frobnicate", "now"}, wantCode: exitBadInput,
			wantStderr: texts.T("cli.unknownCommand", map[string]any{"name": "frobnicate now"}),
		},
		{name: "an unknown flag of a command", args: []string{"backup", "--nope"}, wantCode: exitBadInput, wantStderr: "-nope"},
		{name: "an argument too many", args: []string{"doctor", "extra"}, wantCode: exitBadInput, wantStderr: "extra"},
		{name: "a wrong setting of the environment", args: []string{"backup"}, wantCode: exitBadInput, wantStderr: "WYO_DOMAIN"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			env := map[string]string{}
			if strings.Contains(tc.name, "environment") {
				env["WYO_DOMAIN"] = "https://not a domain"
			}

			got := runWith(t, &fakeSystem{}, env, tc.args...)
			if got.code != tc.wantCode || !strings.Contains(got.stdout, tc.wantStdout) || !strings.Contains(got.stderr, tc.wantStderr) {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

func TestInitMakesTheFolderOnceAndChecksItsChoice(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "data")
	sys := &fakeSystem{}

	if got := runWith(t, sys, nil, "init", "--data", dir, "--updates=manual"); got.code != exitOK || !strings.Contains(got.stdout, dir) {
		t.Fatalf("first init: %+v", got)
	}

	if got := runWith(t, sys, nil, "init", "--data", dir); got.code != exitOK {
		t.Fatalf("second init changes nothing and succeeds: %+v", got)
	}

	if _, err := os.Stat(filepath.Join(dir, "secret.key")); err != nil {
		t.Fatalf("the key file: %v", err)
	}

	if got := runWith(t, sys, nil, "init", "--data", dir, "--updates=maybe"); got.code != exitBadInput || !strings.Contains(got.stderr, "--updates") {
		t.Fatalf("a wrong choice: %+v", got)
	}
}

func TestACommandRootRunsWorksAsTheOwnerOfTheDataFolder(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		euid  int
		owner [2]int
		want  *[2]int
	}{
		{name: "root on the folder of the wyo user", euid: 0, owner: [2]int{998, 997}, want: &[2]int{998, 997}},
		{name: "root on a folder root owns", euid: 0, owner: [2]int{0, 0}},
		{name: "the wyo user itself", euid: 998, owner: [2]int{998, 997}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sys := &fakeSystem{euid: tc.euid, owner: tc.owner}
			if err := becomeDataOwner(sys, t.TempDir()); err != nil {
				t.Fatal(err)
			}

			if (tc.want == nil) != (sys.became == nil) || (tc.want != nil && *tc.want != *sys.became) {
				t.Fatalf("became %v, want %v", sys.became, tc.want)
			}
		})
	}
}
