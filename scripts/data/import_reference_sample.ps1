param(
    [Parameter(Mandatory = $true)][string]$TokenFile,
    [Parameter(Mandatory = $true)][string]$WorkspaceId,
    [Parameter(Mandatory = $true)][string]$EnvironmentId,
    [Parameter(Mandatory = $true)][string]$PlacesSourceId,
    [Parameter(Mandatory = $true)][string]$ElevationSourceId,
    [uri]$BaseUrl = 'http://localhost:8080'
)
$ErrorActionPreference = 'Stop'
if (-not $BaseUrl.IsAbsoluteUri -or ($BaseUrl.Scheme -ne 'https' -and -not ($BaseUrl.Scheme -eq 'http' -and $BaseUrl.IsLoopback)) -or $BaseUrl.UserInfo -or $BaseUrl.Query -or $BaseUrl.Fragment -or $BaseUrl.AbsolutePath -ne '/') {
    throw 'BaseUrl must be an HTTPS origin, or a loopback HTTP origin for local development.'
}
$referenceToken = (Get-Content -LiteralPath $TokenFile -Raw).Trim()
if (-not $referenceToken -or $referenceToken.Contains("`n") -or $referenceToken.Contains("`r")) { throw 'Invalid credential file.' }
$referenceHeaders = @{
    Authorization = "Bearer $referenceToken"
    'X-EMAD-Workspace-ID' = $WorkspaceId
    'X-EMAD-Environment-ID' = $EnvironmentId
    'X-EMAD-Partition' = 'PUBLIC_EVIDENCE'
}
$referenceRoot = Join-Path $PSScriptRoot '../../testdata/reference/jeddah-makkah'
foreach ($referenceItem in @(@{ Name = 'places'; Source = $PlacesSourceId }, @{ Name = 'elevation'; Source = $ElevationSourceId })) {
    $referenceManifest = Get-Content -LiteralPath (Join-Path $referenceRoot ($referenceItem.Name + '.json')) -Raw -Encoding UTF8 | ConvertFrom-Json
    $referenceManifest.sourceId = $referenceItem.Source
    $referenceBody = $referenceManifest | ConvertTo-Json -Depth 40 -Compress
    $referenceResult = Invoke-RestMethod -Method Post -Uri ($BaseUrl.AbsoluteUri.TrimEnd('/') + '/api/v1/reference-datasets') -Headers $referenceHeaders -ContentType 'application/json; charset=utf-8' -Body ([Text.Encoding]::UTF8.GetBytes($referenceBody)) -TimeoutSec 45
    Write-Output ("{0}: id={1} version={2} sha256={3}" -f $referenceItem.Name, $referenceResult.id, $referenceResult.version, $referenceResult.sha256)
}
