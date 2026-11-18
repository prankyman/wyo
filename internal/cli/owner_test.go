package cli

import (
	"strings"
	"testing"
)

func TestTheOwnerIsCreatedOnceAndItsPasswordSetAgain(t *testing.T) {
	t.Parallel()

	dir := initialisedFolder(t)
	sys := &fakeSystem{}
	texts := defaultTexts()

	if got := runWith(t, sys, nil, "owner", "reset-password", "--data", dir, "--password-file", passwordFile(t, "a-long-password")); got.code != exitFailure ||
		!strings.Contains(got.stderr, texts.T("cli.owner.noOwner", nil)) {
		t.Fatalf("reset before create: %+v", got)
	}

	if got := runWith(t, sys, nil, "owner", "create", "--data", dir, "--login", "owner", "--password-file", passwordFile(t, "a-long-password")); got.code != exitOK ||
		!strings.Contains(got.stdout, "«owner»") {
		t.Fatalf("create: %+v", got)
	}

	if got := runWith(t, sys, nil, "owner", "create", "--data", dir, "--login", "other", "--password-file", passwordFile(t, "a-long-password")); got.code != exitFailure ||
		!strings.Contains(got.stderr, texts.T("cli.owner.exists", nil)) {
		t.Fatalf("a second owner: %+v", got)
	}

	sys.terminal, sys.secrets = true, []string{"another-long-one", "another-long-one"}
	if got := runWith(t, sys, nil, "owner", "reset-password", "--data", dir); got.code != exitOK ||
		!strings.Contains(got.stdout, texts.T("cli.owner.passwordReset", nil)) || !strings.Contains(got.stderr, "10") {
		t.Fatalf("reset from the keyboard: %+v", got)
	}
}

func TestWhatTheOwnerCommandsRefuse(t *testing.T) {
	t.Parallel()

	texts := defaultTexts()
	cases := []struct {
		name     string
		args     func(dir string) []string
		sys      *fakeSystem
		wantCode int
		want     string
	}{
		{
			name: "no login", args: func(dir string) []string { return []string{"owner", "create", "--data", dir} },
			sys: &fakeSystem{}, wantCode: exitBadInput, want: texts.T("cli.owner.needLogin", nil),
		},
		{
			name: "a login too short",
			args: func(dir string) []string {
				return []string{"owner", "create", "--data", dir, "--login", "ab", "--password-file", passwordFile(t, "a-long-password")}
			},
			sys: &fakeSystem{}, wantCode: exitBadInput, want: texts.T("cli.owner.badLogin", map[string]any{"min": 3, "max": 64}),
		},
		{
			name: "a password too short",
			args: func(dir string) []string {
				return []string{"owner", "create", "--data", dir, "--login", "owner", "--password-file", passwordFile(t, "short")}
			},
			sys: &fakeSystem{}, wantCode: exitBadInput, want: texts.T("cli.owner.badPassword", map[string]any{"min": 10}),
		},
		{
			name: "a password file that cannot be read",
			args: func(dir string) []string {
				return []string{"owner", "create", "--data", dir, "--login", "owner", "--password-file", dir + "/absent"}
			},
			sys: &fakeSystem{}, wantCode: exitBadInput, want: textBefore(texts, "cli.owner.passwordFile", "{file}"),
		},
		{
			name: "no keyboard and no file", args: func(dir string) []string { return []string{"owner", "create", "--data", dir, "--login", "owner"} },
			sys: &fakeSystem{}, wantCode: exitBadInput, want: texts.T("cli.owner.noTerminal", nil),
		},
		{
			name: "two passwords that differ", args: func(dir string) []string { return []string{"owner", "create", "--data", dir, "--login", "owner"} },
			sys: &fakeSystem{terminal: true, secrets: []string{"a-long-password", "a-long-passworD"}}, wantCode: exitBadInput,
			want: texts.T("cli.owner.passwordsDiffer", nil),
		},
		{
			name: "the keyboard gives nothing", args: func(dir string) []string { return []string{"owner", "create", "--data", dir, "--login", "owner"} },
			sys: &fakeSystem{terminal: true}, wantCode: exitFailure, want: "no more input",
		},
		{
			name: "the keyboard gives one line only", args: func(dir string) []string { return []string{"owner", "create", "--data", dir, "--login", "owner"} },
			sys: &fakeSystem{terminal: true, secrets: []string{"a-long-password"}}, wantCode: exitFailure, want: "no more input",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := runWith(t, tc.sys, nil, tc.args(initialisedFolder(t))...)
			if got.code != tc.wantCode || !strings.Contains(got.stderr, tc.want) {
				t.Fatalf("got %+v, want exit %d and %q", got, tc.wantCode, tc.want)
			}
		})
	}
}

// textBefore is the part of a text before one of its placeholders, for a text whose parameter is a temporary path.
func textBefore(t texts, key, placeholder string) string {
	before, _, _ := strings.Cut(t.messages[key], placeholder)

	return strings.TrimSpace(before)
}
