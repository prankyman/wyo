package httpapi

import (
	"net/http"

	"github.com/prankyman/wyo/internal/backup"
	"github.com/prankyman/wyo/internal/httpapi/gen"
	"github.com/prankyman/wyo/internal/notices"
	"github.com/prankyman/wyo/internal/updatestate"
)

// text is a pointer to a text, nil when it is empty: the description leaves such a field out.
func text(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}

func backupBody(e backup.Entry) gen.Backup {
	return gen.Backup{Name: e.Name, Size: e.Size, CreatedAt: e.CreatedAt}
}

// GetInstallation answers GET /api/v1/installation.
func (a *api) GetInstallation(w http.ResponseWriter, r *http.Request) {
	auto, err := a.deps.Settings.AutoUpdates(r.Context())
	if err != nil {
		a.fail(w, r, err)

		return
	}

	state, err := updatestate.Read(a.deps.DataDir)
	if err != nil {
		a.fail(w, r, err)

		return
	}

	var body gen.Installation

	body.Version = a.deps.Info.Version
	body.Updates.Auto = auto
	body.Updates.Requested = updatestate.Pending(a.deps.DataDir)
	body.Updates.LastCheckAt = state.LastCheckAt
	body.Updates.LatestVersion = text(state.LatestVersion)
	body.Updates.WaitingVersion = text(state.WaitingVersion)
	body.Updates.FailedVersion = text(state.FailedVersion)

	cert := a.deps.Certificates.Status()
	body.Certificate.Mode = gen.InstallationCertificateMode(cert.Mode)
	body.Certificate.State = gen.InstallationCertificateState(cert.State)
	body.Certificate.Domain = text(cert.Domain)
	body.Certificate.NotAfter = cert.NotAfter
	body.Certificate.LastError = text(cert.LastError)

	copies := a.deps.Backups.State(r.Context())
	body.Backups.State = gen.InstallationBackupsState(copies.Name)

	if copies.Last != nil {
		last := backupBody(*copies.Last)
		body.Backups.Last = &last
	}

	writeJSON(w, http.StatusOK, body)
}

func (a *api) requestUpdate(w http.ResponseWriter, r *http.Request, kind updatestate.RequestKind) {
	if err := updatestate.Request(a.deps.DataDir, kind); err != nil {
		a.failWith(w, r, err, "update_request_failed")

		return
	}

	writeJSON(w, http.StatusAccepted, gen.UpdateRequest{Requested: gen.UpdateRequestRequested(kind)})
}

// RequestUpdateCheck answers POST /api/v1/updates/check.
func (a *api) RequestUpdateCheck(w http.ResponseWriter, r *http.Request) {
	a.requestUpdate(w, r, updatestate.RequestCheck)
}

// RequestUpdateInstall answers POST /api/v1/updates/install.
func (a *api) RequestUpdateInstall(w http.ResponseWriter, r *http.Request) {
	a.requestUpdate(w, r, updatestate.RequestInstall)
}

// ListBackups answers GET /api/v1/backups.
func (a *api) ListBackups(w http.ResponseWriter, r *http.Request) {
	entries, err := a.deps.Backups.List(r.Context())
	if err != nil {
		a.fail(w, r, err)

		return
	}

	body := gen.BackupList{Backups: make([]gen.Backup, 0, len(entries))}
	for _, e := range entries {
		body.Backups = append(body.Backups, backupBody(e))
	}

	writeJSON(w, http.StatusOK, body)
}

// CreateBackup answers POST /api/v1/backups.
func (a *api) CreateBackup(w http.ResponseWriter, r *http.Request) {
	entry, err := a.deps.Backups.Write(r.Context())
	if err != nil {
		a.failWith(w, r, err, "backup_failed")

		return
	}

	writeJSON(w, http.StatusCreated, backupBody(entry))
}

func noticeBody(n notices.Notice) gen.Notice {
	body := gen.Notice{Id: n.ID, Source: gen.NoticeSource(n.Source), Code: n.Code, Tone: gen.NoticeTone(n.Tone), At: n.At}

	if len(n.Params) > 0 {
		body.Params = &n.Params
	}

	return body
}

// ListNotices answers GET /api/v1/notices.
func (a *api) ListNotices(w http.ResponseWriter, r *http.Request) {
	list := notices.Collect(r.Context(), a.deps.Notices)

	body := gen.NoticeList{Notices: make([]gen.Notice, 0, len(list))}
	for _, n := range list {
		body.Notices = append(body.Notices, noticeBody(n))
	}

	writeJSON(w, http.StatusOK, body)
}
