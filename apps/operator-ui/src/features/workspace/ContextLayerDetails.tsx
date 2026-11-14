import {projectContextStatus} from './contextLayerStatus'

export function ContextLayerDetails({status}: {status: ReturnType<typeof projectContextStatus>}) {
  return <div className="space-y-1">
    <p>{status.state}</p>
    <p>Admitted footprint · {status.coverage}. {status.completeness}.</p>
    {status.retrievalFailed && <p>Latest provider attempt failed. Retained records keep their original dates.</p>}
    <p>{status.total === undefined ? 'Matching count unknown' : `${status.total.toLocaleString()} matching records`} · {status.displayed.toLocaleString()} display marks{status.limited ? ' · capped or aggregated' : ''}.</p>
    {status.latest && <p>Latest observation shown or reported by coverage <time dateTime={status.latest}>{status.latest}</time></p>}
    {status.age.map((row, index) => <p key={`${row.source}:${index}`}>{row.source} · observation age {row.freshness}{row.evidenceAt && ` · observed ${row.evidenceAt}`}{row.retrievalAt && ` · retrieved ${row.retrievalAt}`}</p>)}
    <p>Provider query coverage leaves observation completeness unknown.</p>
  </div>
}
