package cli

import "context"

// command is one command of the wyo server. Name is what a person types after `wyo`; a name of two words is a group and
// its command (`owner create`). The README names only commands of this table (src/lib/readme-policy.test.ts reads it).
type command struct {
	Name string
	run  func(ctx context.Context, e *Env, args []string) int
}

// commands is the one table of the wyo server's commands.
func commands() []command {
	return []command{
		{Name: "serve", run: runServe},
		{Name: "version", run: runVersion},
		{Name: "init", run: runInit},
		{Name: "owner create", run: runOwnerCreate},
		{Name: "owner reset-password", run: runOwnerResetPassword},
		{Name: "update", run: runUpdate},
		{Name: "backup", run: runBackup},
		{Name: "restore", run: runRestore},
		{Name: "doctor", run: runDoctor},
	}
}
