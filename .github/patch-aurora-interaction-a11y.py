from pathlib import Path

path = Path('.aurora-interaction-a11y.mjs')
source = path.read_text()
old = r'''async function auditDesktopOverlays(theme){
  const context=await makeContext({width:1440,height:1000,theme})
  const page=await context.newPage()
  const label=`desktop-overlays-${theme}`
  watchPage(page,label)
  await page.goto(`${APP}/dashboard/overview`,{waitUntil:'domcontentloaded'})
  await settle(page)
  await layoutAudit(page,label)
  await keyboardAudit(page,label,16)
  const config=page.locator('header button[aria-describedby="config-drawer-description"]').first()
  if(await config.count()){
    await config.click()
    const dialog=page.locator('[role="dialog"]:visible').first()
    await bboxFitsViewport(dialog,page,label,'config-drawer')
    await page.screenshot({path:path.join(outDir,'screenshots',`${label}-config.png`),fullPage:false})
    await page.keyboard.press('Escape')
  }else{
    issues.push({severity:'P2',type:'desktop-config-trigger-missing',label})
  }
  await page.close()
  await context.close()
}'''
new = r'''async function auditDesktopOverlays(theme){
  const context=await makeContext({width:1440,height:1000,theme})
  const page=await context.newPage()
  const label=`desktop-overlays-${theme}`
  watchPage(page,label)
  await page.goto(`${APP}/dashboard/overview`,{waitUntil:'domcontentloaded'})
  await settle(page)
  await layoutAudit(page,label)
  await keyboardAudit(page,label,16)

  const quickTools=page.getByRole('button',{name:/快捷工具|Quick tools/i}).first()
  if(!(await quickTools.count())||!(await quickTools.isVisible())){
    issues.push({severity:'P1',type:'desktop-quick-tools-trigger-missing',label})
  }else{
    await quickTools.click()
    const toolsPopover=page.locator('[data-slot="popover-content"]:visible').first()
    await bboxFitsViewport(toolsPopover,page,label,'quick-tools-popover')
    await page.screenshot({path:path.join(outDir,'screenshots',`${label}-quick-tools.png`),fullPage:false})

    const config=page.locator('button[aria-describedby="config-drawer-description"]:visible').first()
    if(await config.count()){
      await config.click()
      const dialog=page.locator('[role="dialog"]:visible').first()
      await bboxFitsViewport(dialog,page,label,'config-drawer')
      await page.screenshot({path:path.join(outDir,'screenshots',`${label}-config.png`),fullPage:false})
      await page.keyboard.press('Escape')
    }else{
      issues.push({severity:'P1',type:'desktop-config-trigger-missing-in-quick-tools',label})
    }
  }
  await page.close()
  await context.close()
}'''
if old not in source:
    raise SystemExit('Expected desktop overlay QA block not found')
path.write_text(source.replace(old, new, 1))
