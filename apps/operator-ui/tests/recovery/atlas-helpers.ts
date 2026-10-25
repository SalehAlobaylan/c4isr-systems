import {expect,type Page} from '@playwright/test'

/** Adaptive side/phone toolbars expose the same actions through one menu. */
export async function mapControl(page:Page,name:string){
 const action=page.getByRole('button',{name,exact:true})
 await expect.poll(async()=>await action.isVisible()||await page.getByLabel('Map controls',{exact:true}).isVisible()).toBeTruthy()
 if(!await action.isVisible())await page.getByLabel('Map controls',{exact:true}).click()
 await action.click()
}

export async function expectMapControlFocus(page:Page,name:string){
 const menu=page.getByLabel('Map controls',{exact:true})
 await expect(await menu.isVisible()?menu:page.getByRole('button',{name,exact:true})).toBeFocused()
}
