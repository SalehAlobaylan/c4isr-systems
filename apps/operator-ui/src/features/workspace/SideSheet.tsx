import { useEffect, useRef, type ReactNode } from 'react'

export function SideSheet({title,children,onClose}:{title:string;children:ReactNode;onClose:()=>void}) {
  const ref=useRef<HTMLDialogElement>(null)
  useEffect(()=>{const dialog=ref.current;dialog?.showModal();return()=>dialog?.close()},[])
  return <dialog ref={ref} className="ux-dialog" aria-labelledby="sheet-title" onCancel={onClose} onClick={e=>{if(e.target===e.currentTarget)onClose()}}><section className="ux-dialog-panel"><header><h2 id="sheet-title">{title}</h2><button className="ux-button" onClick={onClose} autoFocus>Close</button></header><div className="ux-dialog-body">{children}</div></section></dialog>
}
