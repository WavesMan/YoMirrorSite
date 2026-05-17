$c = Get-Content 'internal/core/s3/client.go' -Raw
$c = $c -replace 'client := s3\.NewFromConfig\(awsCfg\)', 'client := s3.NewFromConfig(awsCfg, func(o *s3.Options) { o.UsePathStyle = true })'
Set-Content 'internal/core/s3/client.go' -Value $c -NoNewline
Write-Host "DONE"
