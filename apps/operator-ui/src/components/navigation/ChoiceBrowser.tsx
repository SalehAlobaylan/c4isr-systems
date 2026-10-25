import {Dialog} from '@base-ui-components/react/dialog'
import {useEffect,useLayoutEffect,useRef,useState,type ReactNode} from 'react'
import {CheckIcon,ChevronDownIcon,SearchIcon,XIcon} from '@/components/icons'

export type NavigationChoice={id:string;label:string;description:string;group:string;icon:ReactNode;meta?:string;status?:string;keywords?:string;disabled?:boolean;reason?:string}
const normalize=(text:string)=>text.normalize('NFKD').replace(/[\u0300-\u036f]/g,'').toLocaleLowerCase()

/** A deliberate, searchable action browser. Native buttons retain ordinary
 * tab/Enter behavior; the dialog owns focus, Escape and return to its trigger. */
export function ChoiceBrowser({label,title,description,items,value,onChoose,trigger,className='',rows=false,pins,onPin,limit=6,openRequest=0,queryOnRequest=''}:{
 label:string;title:string;description:string;items:NavigationChoice[];value?:string;onChoose:(id:string)=>void|Promise<void>;trigger:ReactNode;className?:string;rows?:boolean;pins?:string[];onPin?:(id:string)=>void;limit?:number;openRequest?:number;queryOnRequest?:string
}){
 const [open,setOpen]=useState(false),[search,setSearch]=useState(''),[group,setGroup]=useState(''),[expanded,setExpanded]=useState<string[]>([]),[busy,setBusy]=useState(''),[error,setError]=useState('')
 const input=useRef<HTMLInputElement>(null),results=useRef<HTMLDivElement>(null)
 const pendingPin=useRef('')
 useLayoutEffect(()=>{const id=pendingPin.current;if(!id)return;pendingPin.current='';[...results.current?.querySelectorAll<HTMLButtonElement>('[data-pin]')??[]].find(button=>button.dataset.pin===id)?.focus()},[pins])
 useEffect(()=>{if(openRequest){setSearch(queryOnRequest);setGroup('');setExpanded([]);setError('');setOpen(true)}},[openRequest,queryOnRequest])
 const query=normalize(search.trim()),groups=[...new Set(items.map(item=>item.group))]
 const visible=items.filter(item=>(!group||item.group===group)&&normalize([item.label,item.description,item.meta,item.status,item.keywords,item.group].join(' ')).includes(query))
 const sections=[...(onPin&&visible.some(item=>pins?.includes(item.id))?['Pinned']:[]),...groups]
 const choose=async(id:string)=>{if(busy)return;setBusy(id);setError('');try{await onChoose(id);setOpen(false)}catch(e){setError(e instanceof Error?e.message:'This workspace could not be opened. Try again.')}finally{setBusy('')}}
 return <Dialog.Root open={open} onOpenChange={next=>{setOpen(next);if(next){setSearch('');setGroup('');setExpanded([]);setError('')}}}>
  <Dialog.Trigger className={`navigation-choice-trigger ${className}`} aria-label={label} title={typeof trigger==='string'?trigger:undefined}>{trigger}<ChevronDownIcon/></Dialog.Trigger>
  <Dialog.Portal>
   <Dialog.Backdrop className="navigation-choice-backdrop"/>
   <Dialog.Popup className={`navigation-choice-popup ${rows?'navigation-choice-rows':''}`} initialFocus={input}>
    <header><div><p className="navigation-choice-eyebrow">EMAD · navigation</p><Dialog.Title>{title}</Dialog.Title><Dialog.Description>{description}</Dialog.Description></div><Dialog.Close aria-label="Close chooser"><XIcon/></Dialog.Close></header>
    <div className="navigation-choice-search"><SearchIcon/><input ref={input} type="search" aria-label={`Search ${title.toLowerCase()}`} placeholder={rows?'Search name, date, state or reference…':'Search tools and capabilities…'} value={search} onChange={e=>setSearch(e.target.value)} onKeyDown={e=>{if(e.key==='ArrowDown'){e.preventDefault();results.current?.querySelector<HTMLButtonElement>('[data-choice]:not(:disabled)')?.focus()}}}/>{search&&<button aria-label="Clear chooser search" onClick={()=>{setSearch('');input.current?.focus()}}><XIcon/></button>}</div>
    <div className="navigation-choice-filters" role="group" aria-label="Choice categories"><button aria-pressed={!group} onClick={()=>setGroup('')}>All <span>{items.length}</span></button>{groups.map(name=><button key={name} aria-pressed={group===name} onClick={()=>setGroup(name)}>{name} <span>{items.filter(item=>item.group===name).length}</span></button>)}</div>
    {error&&<p role="alert" className="navigation-choice-error">{error}</p>}
    <div ref={results} className="navigation-choice-results" aria-label="Available choices" aria-busy={!!busy} onKeyDown={event=>{
     if(!['ArrowDown','ArrowUp','Home','End'].includes(event.key)||(event.target as HTMLElement).closest('[data-choice]')===null)return
     const buttons=[...results.current!.querySelectorAll<HTMLButtonElement>('[data-choice]:not(:disabled)')],index=buttons.indexOf(event.target as HTMLButtonElement)
     event.preventDefault();if(event.key==='ArrowUp'&&index===0){input.current?.focus();return}buttons[event.key==='Home'?0:event.key==='End'?buttons.length-1:Math.max(0,Math.min(buttons.length-1,index+(event.key==='ArrowDown'?1:-1)))]?.focus()
    }}>
     {!visible.length&&<div className="navigation-choice-empty"><SearchIcon/><h3>No matching choices</h3><p>Try another name, reference or category.</p><button onClick={()=>{setSearch('');setGroup('');input.current?.focus()}}>Clear filters</button></div>}
     {sections.map(section=>{
      const choices=visible.filter(item=>section==='Pinned'?pins?.includes(item.id):item.group===section&&!pins?.includes(item.id)).sort((a,b)=>a.id===value?-1:b.id===value?1:0),shown=query||expanded.includes(section)?choices:choices.slice(0,limit)
      return choices.length>0&&<section key={section} aria-label={section}><h3>{section}<span>{choices.length}</span></h3><div className="navigation-choice-grid">{shown.map(item=><div className="navigation-choice-card" key={item.id} data-current={item.id===value}>
       <button data-choice={item.id} disabled={item.disabled||!!busy} aria-label={`Open ${item.label}${item.meta?`, ${item.meta}`:''}`} aria-description={`${item.disabled?item.reason:item.description}${item.status?`. ${item.status.toLowerCase()}`:''}`} aria-current={item.id===value?'true':undefined} onClick={()=>void choose(item.id)}>
        <span className="navigation-choice-icon">{item.icon}</span><span className="navigation-choice-copy"><strong>{item.label}</strong><span>{item.disabled?item.reason??'Access required':item.description}</span>{item.meta&&<small>{item.meta}</small>}</span>
        <span className="navigation-choice-state">{item.id===value?<span className="navigation-choice-current"><CheckIcon/>Current</span>:busy===item.id?'Opening…':item.status&&<span data-state={item.status}>{item.status.replaceAll('_',' ').toLowerCase()}</span>}</span>
       </button>{onPin&&<button className="navigation-choice-pin" data-pin={item.id} aria-label={`${pins?.includes(item.id)?'Unpin':'Pin'} ${item.label}, ${item.meta??item.id}`} aria-pressed={!!pins?.includes(item.id)} onClick={()=>{pendingPin.current=item.id;onPin(item.id)}} title="Pin this workspace">{pins?.includes(item.id)?'★':'☆'}</button>}
      </div>)}</div>{shown.length<choices.length&&<button className="navigation-choice-more" onClick={()=>setExpanded(v=>[...v,section])}>Show all {choices.length} in {section.toLowerCase()}</button>}</section>
     })}
    </div>
    <footer><span>{visible.length} {visible.length===1?'choice':'choices'}{search?' match your search':''}</span><span>↑ ↓ browse · Enter open · Esc return</span></footer>
   </Dialog.Popup>
  </Dialog.Portal>
 </Dialog.Root>
}
