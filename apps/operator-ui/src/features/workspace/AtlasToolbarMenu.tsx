import {useEffect,useRef,type ReactNode} from 'react'

/** Transient toolbar menus never allocate another persistent map panel. */
export function AtlasToolbarMenu({label,className='',summary,children}:{label:string;className?:string;summary:ReactNode;children:ReactNode}){
 const ref=useRef<HTMLDetailsElement>(null)
 useEffect(()=>{const dismiss=(event:PointerEvent)=>{if(ref.current?.open&&!ref.current.contains(event.target as Node))ref.current.open=false};document.addEventListener('pointerdown',dismiss);return()=>document.removeEventListener('pointerdown',dismiss)},[])
 return <details ref={ref} className={`atlas-toolbar-menu ${className}`} onToggle={event=>{
   if(event.currentTarget.open)document.querySelectorAll<HTMLDetailsElement>('.atlas-toolbar-menu[open]').forEach(other=>{if(other!==ref.current)other.open=false})
 }} onClick={event=>{if((event.target as HTMLElement).closest('button')&&ref.current)ref.current.open=false}} onKeyDown={event=>{
   if(event.key==='Escape'&&ref.current?.open){event.stopPropagation();ref.current.open=false;ref.current.querySelector('summary')?.focus()}
 }}><summary aria-label={label}>{summary}</summary><div className="atlas-toolbar-menu-body">{children}</div></details>
}
