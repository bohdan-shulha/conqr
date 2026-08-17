# Changelog

## [2.5.0] - 2026-08-17

- feat(startup): accept a group name in dependsOn (#24)


## [2.4.3] - 2026-08-17

- fix(process-manager): stop a process that you stop while it starts, instead of leaving it to run under the STOP label

## [2.4.2] - 2026-08-17

- fix(process-manager): remove the data race on the process state that `go test -race` reports
- fix(process-manager): let `r` start a process that you stop while it waits for a dependency
- fix(process-manager): report `› Stop initiated` when you stop a waiting process
- chore: run the tests with the race detector in `make test`

## [2.4.1] - 2026-08-17

- fix(npm): resolve bundled binary on x64 hosts (#20)- Maintenance release.


## [2.4.0] - 2026-08-17

- feat(startup): add dependency-ordered startup with readiness detection (#19)
- feat: add the `dependsOn`, `ready`, `busy`, and `readyTimeout` config keys
- feat(tui): add the `WAIT` and `BUILD` status labels
- fix: keep the group and the command configuration after a restart


## [2.3.0] - 2026-06-07

- chore: release v2.1.0
- feat(tui): add process groups with merged log views
- feat(tui): add manual process stop with s key
- fix(tui): preserve scroll when toggling log-only
- fix(tui): preserve log viewport while scrolled

## [2.1.0] - 2026-06-07

- feat(tui): add process groups with merged log views
- feat(tui): add manual process stop with s key
- fix(tui): preserve scroll when toggling log-only
- fix(tui): preserve log viewport while scrolled

## [v2.0.0] - 2026-05-30

- ci: harden npm release publishing- Maintenance release.


## [2.0.0] - 2026-05-30

- refactor: rewrite conqr runtime in Go
- Rewrote the runtime in Go while preserving the npm `conqr` command.
- Switched the terminal UI to Bubble Tea, Bubbles viewport, and Lip Gloss.
- Added npm packaging for bundled Go binaries.
