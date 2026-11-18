package cli

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/prankyman/wyo/internal/config"
	"github.com/prankyman/wyo/internal/updatestate"
)

func TestTheDoctorSaysWhatIsRightAndWhatIsWrong(t *testing.T) {
	t.Parallel()

	texts := defaultTexts()
	healthy := func() *fakeSystem {
		return &fakeSystem{
			service: &fakeService{active: true}, hosts: []string{"203.0.113.10"}, own: []string{"203.0.113.10"},
			free: 5 << 30, notAfter: time.Date(2027, 2, 15, 12, 0, 0, 0, time.UTC),
		}
	}

	cases := []struct {
		name     string
		sys      func() *fakeSystem
		env      map[string]string
		state    *updatestate.State
		backup   bool
		wantCode int
		want     []string
	}{
		{
			name: "all is well", sys: healthy, env: map[string]string{"WYO_DOMAIN": "wyo.example.ru"}, backup: true,
			want: []string{
				texts.T("cli.doctor.service.ok", nil),
				texts.T("cli.doctor.domain.ok", map[string]any{"domain": "wyo.example.ru", "address": "203.0.113.10"}),
				texts.T("cli.doctor.port.ok", map[string]any{"port": "443"}),
				texts.T("cli.doctor.certificate.ok", map[string]any{"date": "15.02.2027"}),
				texts.T("cli.doctor.summary.ok", nil),
			},
		},
		{
			name: "everything is wrong",
			sys: func() *fakeSystem {
				return &fakeSystem{service: &fakeService{err: errors.New("no systemd")}, dialErr: errors.New("refused"), certErr: errors.New("none")}
			},
			env: map[string]string{"WYO_DOMAIN": "wyo.example.ru"}, state: &updatestate.State{FailedVersion: "0.0.4"}, wantCode: exitFailure,
			want: []string{
				texts.T("cli.doctor.service.bad", nil),
				texts.T("cli.doctor.domain.none", map[string]any{"domain": "wyo.example.ru"}),
				texts.T("cli.doctor.port.bad", map[string]any{"port": "80"}),
				texts.T("cli.doctor.certificate.bad", map[string]any{"domain": "wyo.example.ru"}),
				texts.T("cli.doctor.disk.bad", map[string]any{"free": 0}),
				texts.T("cli.doctor.backup.bad", nil),
				texts.T("cli.doctor.summary.bad", map[string]any{"count": 8}),
				texts.T("cli.doctor.update.hint", nil),
			},
		},
		{
			name: "the domain points elsewhere and a version waits",
			sys: func() *fakeSystem {
				s := healthy()
				s.hosts = []string{"198.51.100.7"}

				return s
			},
			env: map[string]string{"WYO_DOMAIN": "wyo.example.ru"}, state: &updatestate.State{WaitingVersion: "0.0.5"}, backup: true,
			wantCode: exitFailure,
			want: []string{
				texts.T("cli.doctor.domain.bad", map[string]any{"domain": "wyo.example.ru", "address": "198.51.100.7", "own": "203.0.113.10"}),
				"0.0.5",
			},
		},
		{
			name: "behind a web server", sys: healthy, env: map[string]string{"WYO_DOMAIN": "wyo.example.ru", "WYO_BEHIND_PROXY": "1"}, backup: true,
			want: []string{texts.T("cli.doctor.domain.proxy", nil), texts.T("cli.doctor.port.ok", map[string]any{"port": "8484"})},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := initialisedFolder(t)
			if tc.backup {
				if got := runWith(t, &fakeSystem{}, nil, "backup", "--data", dir); got.code != exitOK {
					t.Fatalf("backup: %+v", got)
				}
			}

			if tc.state != nil {
				if err := updatestate.Write(dir, *tc.state); err != nil {
					t.Fatal(err)
				}
			}

			got := runWith(t, tc.sys(), tc.env, "doctor", "--data", dir)
			if got.code != tc.wantCode {
				t.Fatalf("exit %d, want %d: %s", got.code, tc.wantCode, got.stdout)
			}

			for _, line := range tc.want {
				if !strings.Contains(got.stdout, line) {
					t.Errorf("missing %q in:\n%s", line, got.stdout)
				}
			}
		})
	}
}

func TestAnUnreadableUpdateStateIsNoProblemOfTheDoctor(t *testing.T) {
	t.Parallel()

	dir := initialisedFolder(t)
	writeFile(t, dir+"/"+updatestate.StateFile, "{")

	if f := (&Env{texts: defaultTexts()}).updateFinding(configOf(dir)); !f.ok {
		t.Fatalf("got %+v", f)
	}
}

func configOf(dir string) config.Config { return config.Config{DataDir: dir} }

func TestTheDoctorKnowsTheInstallationFromTheInstallersFile(t *testing.T) {
	t.Parallel()

	dir := initialisedFolder(t)
	file := dir + "/wyo.env"
	writeFile(t, file, "WYO_DOMAIN=wyo.example.ru\nWYO_BEHIND_PROXY=1\nWYO_DATA_DIR="+dir+"\n")

	var stdout bytes.Buffer

	Run(t.Context(), &Env{
		Args: []string{"doctor"}, Lookup: func(string) (string, bool) { return "", false }, Stdin: strings.NewReader(""),
		Stdout: &stdout, Stderr: io.Discard, System: &fakeSystem{service: &fakeService{active: true}}, EnvFile: file,
	})

	if !strings.Contains(stdout.String(), defaultTexts().T("cli.doctor.domain.proxy", nil)) {
		t.Fatalf("the doctor did not read the installer's file:\n%s", stdout.String())
	}
}
