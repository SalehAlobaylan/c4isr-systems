import {expect,type Page} from '@playwright/test'

export async function chooseNavigation(page:Page,label:string,id:string){
 await page.getByRole('button',{name:label,exact:true}).click()
 const dialog=page.getByRole('dialog')
 await expect(dialog).toBeVisible()
 if(label==='Workspace environment and run'){
  const resource=JSON.parse(id) as string[]
  await dialog.getByRole('searchbox').fill(resource[3]||resource[0])
 }
 await dialog.locator(`[data-choice=${JSON.stringify(id)}]`).click()
 await expect(dialog).toBeHidden()
}
export const chooseWorkbench=(page:Page,id:string)=>chooseNavigation(page,'Map workspace',id)
export async function chooseFeed(page:Page,id:string){
 await page.getByRole('combobox',{name:'Preview category',exact:true}).click()
 await page.getByRole('option').and(page.locator(`[data-feed=${JSON.stringify(id)}]`)).click()
 await expect(page.getByRole('listbox')).toBeHidden()
}
