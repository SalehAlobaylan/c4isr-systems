import {useMemo,useState} from 'react'
import {SimulationAssetsPage} from '@/routes/simulation-assets'
import {WorkbenchContext,type WorkbenchSearch} from './WorkbenchContext'

/** Deliberately opened within the already-authorized input workspace. */
export function StudioAssetDock({open,onClose}:{open:boolean;onClose:()=>void}){
 const [search,setSearch]=useState<WorkbenchSearch>({}),[maximized,setMaximized]=useState(false)
 const host=useMemo(()=>({search,regionName:'Protected authoring library',open:()=>{},update:setSearch,close:onClose,authoring:true}),[search,onClose])
 return <section className="studio-assets-dock" aria-label="Studio asset workbench" hidden={!open} data-maximized={maximized}><header><h2>Models, templates & environments</h2><span className="ux-muted">Protected authoring · workspace resources</span><div className="ux-actions"><button className="ux-button" onClick={()=>setMaximized(v=>!v)}>{maximized?'Return to scene':'Maximize library'}</button><button className="ux-button" onClick={onClose}>Close library</button></div></header><div><WorkbenchContext.Provider value={host}><SimulationAssetsPage/></WorkbenchContext.Provider></div></section>
}
