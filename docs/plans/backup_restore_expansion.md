# Add Backup and Restore Support for Remaining Modules

## Goal Description
Extend the `Backuper` and `Restorer` interface implementation to the following modules: `gitea`, `postgres`, `hobbypod`, `webdav`, and `workpod`. This will standardize backup and restore operations across the personal-server application.
Modules will only backup their data volumes, consistent with the `bitwarden` implementation.

## Proposed Changes

### Application Logic
No changes needed in `app.go` as it already handles the interface.

### Gitea Module
- **File**: `internal/modules/gitea/gitea.go`
- **Action**: Implement `Backup` and `Restore`.
- **Data Path**: `/data` (to be verified).
- **Pod Label**: `app=gitea`.

### Postgres Module
- **File**: `internal/modules/postgres/postgres.go`
- **Action**: Implement `Backup` and `Restore`.
- **Note**: Will check `old_scripts/backup.sh` to decide between `pg_dump` vs raw volume backup.

### HobbyPod Module
- **File**: `internal/modules/hobbypod/hobbypod.go`
- **Action**: Implement `Backup` and `Restore`.

### WebDAV Module
- **File**: `internal/modules/webdav/webdav.go`
- **Action**: Implement `Backup` and `Restore`.

### WorkPod Module
- **File**: `internal/modules/workpod/workpod.go`
- **Action**: Implement `Backup` and `Restore`.

## Verification Plan
1.  **Build**
    - `go build ./cmd/main.go`
2.  **Manual Verification**
    - Verify command availability: `personal-server <module> backup`
    - Verify code compilation.
