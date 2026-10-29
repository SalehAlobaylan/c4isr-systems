import {useWorkspace} from '@/lib/workspaceContext'
import {ChoiceBrowser,type NavigationChoice} from '@/components/navigation/ChoiceBrowser'
import {Select} from '@base-ui-components/react/select'
import {ActivityIcon,CheckIcon,ChevronDownIcon,LayersIcon,MapIcon,RadarIcon,SearchIcon,ShieldAlertIcon} from '@/components/icons'
import {toolNames,toolPermissions,type MapTool} from './WorkbenchContext'

const descriptions:Record<MapTool,[string,string,string]>={
 investigations:['Start here','Resume a saved question, exact scope and evidence revisions.','cases notes questions saved work'],
 coverage:['Explore','Check whether the selected area and period can be served and who can recover it.','availability gaps recovery readiness'],
 systems:['Manage','See the six-domain grid, installed content and system health.','dashboard capabilities catalogue'],
 geography:['Explore','Inspect geographic references, source records and provenance.','terrain reference countries regions evidence'],
 weather:['Explore','Inspect station weather and cited official hazard reports.','bulletins forecasts hazards stations'],
 tracks:['Explore','Follow retained aircraft and vessel observations with uncertainty.','aircraft aviation ADS-B AIS ships vessels'],
 imagery:['Analyze','Compare retained satellite imagery and reproducible change results.','sentinel satellite NDVI captures raster'],
 review:['Analyze','Review proposals, confidence and independent publication history.','assessments intelligence analyst'],
 context:['Analyze','Preview and export a validated, versioned context package.','exports bundles simulation inputs'],
 sources:['Manage','Check provider access, source freshness and retrieval failures.','credentials FIRMS USGS OpenSky activation'],
 jobs:['Manage','Follow durable preparation and analysis jobs and retry history.','queue processing workers errors'],
 simulation:['Simulation','Enter Studio, exercise operations, recording and replay.','3D models scenes simulation exercises evaluation']
}
const icons={'Start here':<LayersIcon/>,Explore:<MapIcon/>,Analyze:<SearchIcon/>,Manage:<ActivityIcon/>,Simulation:<RadarIcon/>} as Record<string,NavigationChoice['icon']>

export function MapWorkbenchChooser({value,permissions,onChoose}:{value?:MapTool;permissions:string[];onChoose:(value:MapTool|'previews')=>void}){
 const {session}=useWorkspace()
 const items:NavigationChoice[]=[{id:'previews',label:'Map previews',group:'Start here',description:'Browse records in a shallow tray while keeping the map visible.',icon:<MapIcon/>,keywords:'records preview feed'}]
 const priority:MapTool[]=permissions.includes('investigations.review')?['investigations','coverage','review']:permissions.includes('assessments.review')?['review','investigations','context']:permissions.includes('investigations.edit')?['investigations','imagery','coverage']:permissions.includes('sources.manage')?['sources','coverage','jobs']:['geography','coverage']
 for(const tool of [...new Set([...priority,...Object.keys(toolNames)])] as MapTool[]){if(tool==='simulation'&&session.productProfile==='map')continue;const [group,description,keywords]=descriptions[tool];items.push({id:tool,label:toolNames[tool],group,description,keywords,icon:icons[group]??<ShieldAlertIcon/>,disabled:!!toolPermissions[tool]&&!permissions.includes(toolPermissions[tool]!),reason:'Your account needs access to this capability.'})}
 return <ChoiceBrowser label="Map workspace" title="Open a workbench" description="Choose a capability for the current map. Workbenches share the dock and preserve your investigation and drafts." value={value??'previews'} items={items} onChoose={id=>onChoose(id as MapTool|'previews')} className="atlas-workspace-picker" trigger={<><LayersIcon/><span>{value?toolNames[value]:'Workbenches'}</span></>}/>
}

export function PreviewFeedChooser({value,items,onChoose}:{value:string;items:NavigationChoice[];onChoose:(value:string)=>void}){
 const selected=items.find(item=>item.id===value)
 return <Select.Root value={value} onValueChange={next=>next&&onChoose(next)} items={items.map(item=>({value:item.id,label:item.label}))}>
  <Select.Trigger aria-label="Preview category" className="navigation-choice-trigger atlas-feed-picker"><span className="navigation-feed-trigger-icon">{selected?.icon}</span><span>{selected?.label??'Preview feed'}</span><Select.Icon><ChevronDownIcon/></Select.Icon></Select.Trigger>
  <Select.Portal><Select.Positioner side="top" align="start" alignItemWithTrigger={false} sideOffset={8} collisionPadding={12} className="navigation-feed-positioner"><Select.Popup className="navigation-feed-popup">
   <header><strong>Preview feed</strong><p>Browse retained records. Map layers stay independent.</p></header>
   {items.map(item=><Select.Item key={item.id} value={item.id} data-feed={item.id} aria-label={item.label} aria-description={item.description} className="navigation-feed-item"><span className="navigation-feed-icon">{item.icon}</span><span><Select.ItemText>{item.label}</Select.ItemText><small>{item.description}</small></span><Select.ItemIndicator><CheckIcon/></Select.ItemIndicator></Select.Item>)}
  </Select.Popup></Select.Positioner></Select.Portal>
 </Select.Root>
}
