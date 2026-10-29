import {useState,type ReactNode} from 'react'
import {LayersIcon,MapIcon,PlayIcon} from '@/components/icons'
import {ChoiceBrowser,type NavigationChoice} from './ChoiceBrowser'
import {resourceKey,useWorkspace,type Resource} from '@/lib/workspaceContext'

const humanize=(text:string)=>text.replaceAll('-',' ').replaceAll('_',' ').replace(/^./,v=>v.toUpperCase())
const locationName=(r:Resource)=>`${r.workspaceId==='default'?'Default workspace':humanize(r.workspaceId)} / ${r.environmentId==='local'?'Local installation':humanize(r.environmentId)}`
const started=(value?:string)=>value?new Date(value).toLocaleString(undefined,{year:'numeric',month:'short',day:'numeric',hour:'2-digit',minute:'2-digit',second:'2-digit'}):''

export function WorkspaceChooser({resources,onChoose,openRequest=0,exerciseOnly=false,trigger}:{resources:Resource[];onChoose:(key:string)=>Promise<void>;openRequest?:number;exerciseOnly?:boolean;trigger?:ReactNode}){
 const {session}=useWorkspace(),current=resourceKey(session.resource),storageKey=`emad.navigation.pins.v1.${session.principal.id}`
 const [pins,setPins]=useState<string[]>(()=>{try{const raw=JSON.parse(localStorage.getItem(storageKey)??'[]');return Array.isArray(raw)?raw.filter((v):v is string=>typeof v==='string').slice(0,16):[]}catch{return[]}})
 const details=new Map(session.resourceDetails?.map(item=>[resourceKey(item.resource),item]))
 const options:NavigationChoice[]=[...resources].filter(r=>session.productProfile!=='map'||r.partition==='PUBLIC_EVIDENCE').sort((a,b)=>{
  const x=details.get(resourceKey(a))?.startedAt??'',y=details.get(resourceKey(b))?.startedAt??''
  return y.localeCompare(x)||resourceKey(a).localeCompare(resourceKey(b))
 }).map(r=>{
  const id=resourceKey(r),info=details.get(id),isPublic=r.partition==='PUBLIC_EVIDENCE',studio=r.partition==='EXERCISE_INPUT',legacy=r.runId==='legacy',active=!!info?.status&&['RUNNING','PAUSED','QUEUED','STOPPING'].includes(info.status)
  const label=isPublic?'Public evidence':studio?'Simulation Studio':humanize(info?.label??(legacy?'Legacy exercise':'Exercise'))
  const reference=r.runId?`Ref ${r.runId.slice(-8)}`:''
  const showLocation=isPublic||studio||r.workspaceId!==session.resource.workspaceId||r.environmentId!==session.resource.environmentId
  return {id,label,group:isPublic?'Public Context':studio?'Authoring':legacy?'Legacy':active?'Active exercises':'Exercise history',icon:isPublic?<MapIcon/>:studio?<LayersIcon/>:<PlayIcon/>,status:info?.status,description:isPublic?'Real-world observations, imagery and reviewed evidence':studio?'Protected scenes, models and exercise inputs':started(info?.startedAt)||'Start time unavailable',meta:[showLocation?locationName(r):'',reference].filter(Boolean).join(' · '),keywords:[r.workspaceId,r.environmentId,r.runId,info?.startedAt,info?.status].join(' ')}
 }).sort((a,b)=>['Public Context','Authoring','Active exercises','Exercise history','Legacy'].indexOf(a.group)-['Public Context','Authoring','Active exercises','Exercise history','Legacy'].indexOf(b.group))
 const selected=options.find(item=>item.id===current)
 const pin=(id:string)=>setPins(previous=>{const next=previous.includes(id)?previous.filter(value=>value!==id):[id,...previous].slice(0,16);try{localStorage.setItem(storageKey,JSON.stringify(next))}catch{/* Device preferences are optional. */}return next})
 const currentDetail=details.get(current),caption=currentDetail?.startedAt?started(currentDetail.startedAt):locationName(session.resource)
 return <ChoiceBrowser label={exerciseOnly?'Choose exercise':'Workspace environment and run'} title={exerciseOnly?'Choose an exercise':'Choose a workspace'} description={exerciseOnly?'Choose the authorized exercise you want to open. Dates and states distinguish retained runs; no scenario truth is loaded.':session.productProfile==='map'?'Choose an authorized Public Context environment.':'Public evidence, exercise perception and protected authoring use separate workspaces. Choose where you want to work.'} items={options} value={current} onChoose={id=>id===current&&!exerciseOnly?Promise.resolve():onChoose(id)} rows pins={pins} onPin={pin} openRequest={openRequest} queryOnRequest="exercise" className={exerciseOnly?'exercise-choice-trigger':'workspace-choice-trigger'} trigger={trigger??<>{selected?.icon}<span className="workspace-choice-summary" title={`${selected?.label} · ${caption} · ${session.resource.runId??''}`}><strong>{selected?.label??'Choose workspace'}</strong><small>{caption}</small></span><small>Context</small></>}/>
}
