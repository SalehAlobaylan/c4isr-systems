export type AtlasDockSize='collapsed'|'preview'|'expanded'|'full'
export type AtlasDockPlacement='auto'|'horizontal'|'vertical'

/** Browsing is a shallow tray. Detailed work uses width on larger displays;
 * substantial height is reserved for an explicit Expand or Maximize action. */
export function atlasDockLayout(size:AtlasDockSize,placement:AtlasDockPlacement,width:number,height:number,hasPanel:boolean){
 const narrow=width<=760
 const canUseSide=width>=700
 const automaticSide=hasPanel&&width>=1100||width>=700&&height<=620
 const side=size!=='collapsed'&&size!=='full'&&canUseSide&&(placement==='vertical'||placement==='auto'&&automaticSide)
 const previewHeight=hasPanel?Math.min(248,Math.max(176,Math.round(height*.28))):width<1100?152:124
 const bottomHeight=size==='expanded'?Math.max(previewHeight,Math.min(360,Math.round(height*.4))):previewHeight
 const previewWidth=Math.min(520,Math.max(400,Math.round(width*.36)))
 const sideWidth=size==='expanded'?Math.max(previewWidth,Math.min(500,Math.round(width*.42))):previewWidth
 return {orientation:side?'vertical' as const:'horizontal' as const,compactControls:width<1100||side,extent:(side?sideWidth:bottomHeight)+'px',narrow}
}
