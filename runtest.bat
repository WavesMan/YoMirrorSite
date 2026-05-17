@echo off
set GOCACHE=C:\Users\diwei\go-build-cache
mkdir C:\Users\diwei\go-build-cache 2>nul
cd /d D:\GitHub\Source_Codes2\YoMirrorSite
go test -short -count=1 ./internal/config/... ./internal/model/... ./internal/syncer/... ./internal/core/s3/... ./api/handler/...
