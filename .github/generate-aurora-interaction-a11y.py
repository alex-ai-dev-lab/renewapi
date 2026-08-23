from pathlib import Path

source = Path('.aurora-interaction-base.mjs').read_text()
marker = 'await fs.rm(outDir'
if marker not in source:
    raise SystemExit('Unable to locate deterministic harness execution marker')
prefix = source.split(marker, 1)[0]
custom = r'''
await fs.rm(outDir,{recursive:true,force:true})
await fs.mkdir(path.join(outDir,'screenshots'),{recursive:true})
const browser=await chromium.launch({headless:true})
const issues=[]
const observations=[]
const runtimeErrors=[]
const requests=[]
const unhandled=[]

async function makeContext({width,height,theme='light',locale='zh-CN',reducedMotion='no-preference'}){
  const context=await browser.newContext({
    viewport:{width,height},
    deviceScaleFactor:1,
    locale,
    timezoneId:'Asia/Taipei',
    colorScheme:theme,
    reducedMotion,
  })
  await context.addInitScript(({user,fixedNow,theme,locale})=>{
    localStorage.setItem('user',JSON.stringify(user))
    localStorage.setItem('uid',String(user.id))
    localStorage.setItem('i18nextLng',locale)
    localStorage.setItem('theme',theme)
    const NativeDate=Date
    class FixedDate extends NativeDate{
      constructor(...args){super(...(args.length?args:[fixedNow]))}
      static now(){return fixedNow}
    }
    Date=FixedDate
  },{user,fixedNow,theme,locale})
  await context.route('**/api/**',async route=>{
    const url=new URL(route.request().url())
    const payload=responseFor(url)
    requests.push(`${route.request().method()} ${url.pathname}${url.search}`)
    if(payload==null){
      unhandled.push(`${route.request().method()} ${url.pathname}${url.search}`)
      return route.fulfill({status:404,contentType:'application/json',body:JSON.stringify({success:false,message:'QA mock missing'})})
    }
    return route.fulfill({status:200,contentType:'application/json',body:JSON.stringify(payload)})
  })
  return context
}

function watchPage(page,label){
  page.on('console',m=>{if(m.type()==='error')runtimeErrors.push(`[${label}] ${m.text()}`)})
  page.on('pageerror',e=>runtimeErrors.push(`[${label}] PAGEERROR ${e.message}`))
}

async function settle(page){
  await page.waitForTimeout(1200)
  await page.evaluate(()=>document.fonts?.ready)
}

async function layoutAudit(page,label){
  const data=await page.evaluate(()=>({
    scrollWidth:document.documentElement.scrollWidth,
    clientWidth:document.documentElement.clientWidth,
    bodyScrollWidth:document.body.scrollWidth,
    bodyClientWidth:document.body.clientWidth,
  }))
  observations.push({type:'layout',label,...data})
  if(data.scrollWidth>data.clientWidth+1||data.bodyScrollWidth>data.bodyClientWidth+1){
    issues.push({severity:'P1',type:'horizontal-overflow',label,data})
  }
}

async function interactiveAudit(page,label,{mobile=false}={}){
  const result=await page.evaluate(({mobile})=>{
    const visible=(el)=>{
      const r=el.getBoundingClientRect()
      const s=getComputedStyle(el)
      return r.width>0&&r.height>0&&s.visibility!=='hidden'&&s.display!=='none'
    }
    const nameOf=(el)=>{
      const labelledBy=el.getAttribute('aria-labelledby')
      const labelled=labelledBy?labelledBy.split(/\s+/).map(id=>document.getElementById(id)?.textContent||'').join(' ').trim():''
      return (el.getAttribute('aria-label')||labelled||el.getAttribute('title')||el.getAttribute('alt')||el.getAttribute('placeholder')||el.textContent||'').trim().replace(/\s+/g,' ').slice(0,160)
    }
    const controls=[...document.querySelectorAll('button,a[href],input,select,textarea,[role="button"],[role="switch"],[role="checkbox"],[role="radio"]')].filter(visible)
    const unnamed=controls.filter(el=>!nameOf(el)).map(el=>({tag:el.tagName,role:el.getAttribute('role'),html:el.outerHTML.slice(0,240)}))
    const headerTargets=[...document.querySelectorAll('header button,header a[href]')].filter(visible).map(el=>{
      const r=el.getBoundingClientRect()
      return {name:nameOf(el),width:Math.round(r.width),height:Math.round(r.height)}
    })
    const undersized=mobile?headerTargets.filter(x=>x.width<24||x.height<24):[]
    const tables=[...document.querySelectorAll('table')].map(table=>{
      const tr=table.getBoundingClientRect()
      let p=table.parentElement
      let scrollContainer=null
      while(p&&p!==document.body){
        const s=getComputedStyle(p)
        if(['auto','scroll'].includes(s.overflowX)){scrollContainer=p;break}
        p=p.parentElement
      }
      return {
        width:Math.round(tr.width),
        viewport:document.documentElement.clientWidth,
        needsScroll:tr.width>document.documentElement.clientWidth+1,
        hasScrollContainer:Boolean(scrollContainer),
        containerClientWidth:scrollContainer?Math.round(scrollContainer.clientWidth):null,
        containerScrollWidth:scrollContainer?Math.round(scrollContainer.scrollWidth):null,
      }
    })
    return {controls:controls.length,unnamed,headerTargets,undersized,tables}
  },{mobile})
  observations.push({type:'interactive-dom',label,...result})
  if(result.unnamed.length){issues.push({severity:'P2',type:'unnamed-controls',label,items:result.unnamed})}
  if(result.undersized.length){issues.push({severity:'P2',type:'mobile-target-under-24px',label,items:result.undersized})}
  for(const table of result.tables){
    if(table.needsScroll&&!table.hasScrollContainer){issues.push({severity:'P1',type:'table-without-horizontal-scroll-container',label,table})}
  }
}

async function keyboardAudit(page,label,count=14){
  const stops=[]
  for(let i=0;i<count;i++){
    await page.keyboard.press('Tab')
    const stop=await page.evaluate(()=>{
      const el=document.activeElement
      if(!el||el===document.body)return null
      const r=el.getBoundingClientRect()
      const s=getComputedStyle(el)
      const labelledBy=el.getAttribute('aria-labelledby')
      const labelled=labelledBy?labelledBy.split(/\s+/).map(id=>document.getElementById(id)?.textContent||'').join(' ').trim():''
      const name=(el.getAttribute('aria-label')||labelled||el.getAttribute('title')||el.getAttribute('placeholder')||el.textContent||'').trim().replace(/\s+/g,' ').slice(0,120)
      const indicator=(s.outlineStyle!=='none'&&parseFloat(s.outlineWidth||'0')>0)||s.boxShadow!=='none'
      return {tag:el.tagName,name,visible:r.width>0&&r.height>0,focusVisible:el.matches(':focus-visible'),indicator,width:Math.round(r.width),height:Math.round(r.height)}
    })
    if(stop)stops.push(stop)
  }
  observations.push({type:'keyboard',label,stops})
  const visibleStops=stops.filter(x=>x.visible)
  if(!visibleStops.length){issues.push({severity:'P1',type:'no-visible-keyboard-focus',label})}
  const unnamed=visibleStops.filter(x=>!x.name)
  if(unnamed.length){issues.push({severity:'P2',type:'keyboard-focus-without-name',label,items:unnamed})}
  const noIndicator=visibleStops.filter(x=>x.focusVisible&&!x.indicator)
  if(noIndicator.length>Math.ceil(visibleStops.length/3)){
    issues.push({severity:'P2',type:'focus-visible-without-strong-indicator',label,items:noIndicator})
  }
}

async function bboxFitsViewport(locator,page,label,type){
  await locator.waitFor({state:'visible',timeout:5000})
  const box=await locator.boundingBox()
  const viewport=page.viewportSize()
  observations.push({type,label,box,viewport})
  if(!box||!viewport||box.x<-1||box.y<-1||box.x+box.width>viewport.width+1||box.y+box.height>viewport.height+1){
    issues.push({severity:'P1',type:`${type}-outside-viewport`,label,box,viewport})
  }
}

async function auditMobileOverlays(theme){
  const context=await makeContext({width:375,height:812,theme})
  const page=await context.newPage()
  const label=`mobile-overlays-${theme}`
  watchPage(page,label)
  await page.goto(`${APP}/dashboard/overview`,{waitUntil:'domcontentloaded'})
  await settle(page)
  await layoutAudit(page,label)
  await interactiveAudit(page,label,{mobile:true})
  await keyboardAudit(page,label,12)

  const sidebarTrigger=page.locator('[data-slot="sidebar-trigger"]').first()
  await sidebarTrigger.click()
  const mobileSidebar=page.locator('[data-mobile="true"]').first()
  await bboxFitsViewport(mobileSidebar,page,label,'mobile-sidebar')
  await page.screenshot({path:path.join(outDir,'screenshots',`${label}-sidebar.png`),fullPage:false})
  await page.keyboard.press('Escape')

  const search=page.locator('header').getByRole('button',{name:/搜索|Search/i}).first()
  if(await search.count()){
    await search.click()
    const dialog=page.locator('[role="dialog"]:visible').first()
    await bboxFitsViewport(dialog,page,label,'command-dialog')
    await page.screenshot({path:path.join(outDir,'screenshots',`${label}-command.png`),fullPage:false})
    await page.keyboard.press('Escape')
  }else{
    issues.push({severity:'P1',type:'mobile-search-trigger-missing',label})
  }

  const ariaLabels=await page.locator('header button[aria-label]').evaluateAll(nodes=>nodes.map(n=>n.getAttribute('aria-label')))
  observations.push({type:'mobile-header-labels',label,labels:ariaLabels})
  const candidate=page.locator('header button[aria-label*="通知"],header button[aria-label*="Notification"],header button[aria-label*="notification"]').first()
  if(await candidate.count()){
    await candidate.click()
    const popover=page.locator('[data-slot="popover-content"]:visible').first()
    if(await popover.count()){
      await bboxFitsViewport(popover,page,label,'notification-popover')
      await page.screenshot({path:path.join(outDir,'screenshots',`${label}-notifications.png`),fullPage:false})
    }
    await page.keyboard.press('Escape')
  }
  await page.close()
  await context.close()
}

async function auditDesktopOverlays(theme){
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
}

async function auditResponsiveMatrix(theme,locale='zh-CN'){
  for(const [width,height,labelSuffix] of [[1024,1000,'1024'],[768,1000,'768'],[375,812,'375'],[720,500,'zoom-200-proxy']]){
    const context=await makeContext({width,height,theme,locale})
    for(const [name,route] of pages){
      const label=`${theme}-${locale}-${labelSuffix}-${name}`
      const page=await context.newPage()
      watchPage(page,label)
      await page.goto(`${APP}${route}`,{waitUntil:'domcontentloaded'})
      await settle(page)
      await layoutAudit(page,label)
      await interactiveAudit(page,label,{mobile:width<768})
      if(width===375&&(name==='dashboard'||name==='logs'||name==='settings')){
        await page.screenshot({path:path.join(outDir,'screenshots',`${label}.png`),fullPage:false})
      }
      await page.close()
    }
    await context.close()
  }
}

async function auditReducedMotion(theme){
  const context=await makeContext({width:1440,height:1000,theme,reducedMotion:'reduce'})
  const page=await context.newPage()
  const label=`reduced-motion-${theme}`
  watchPage(page,label)
  await page.goto(`${APP}/dashboard/overview`,{waitUntil:'domcontentloaded'})
  await settle(page)
  const matches=await page.evaluate(()=>matchMedia('(prefers-reduced-motion: reduce)').matches)
  observations.push({type:'reduced-motion',label,matches})
  if(!matches){issues.push({severity:'P1',type:'reduced-motion-emulation-failed',label})}
  await layoutAudit(page,label)
  await page.close()
  await context.close()
}

await auditResponsiveMatrix('light','zh-CN')
await auditResponsiveMatrix('dark','zh-CN')
await auditResponsiveMatrix('light','en-US')
await auditMobileOverlays('light')
await auditMobileOverlays('dark')
await auditDesktopOverlays('light')
await auditDesktopOverlays('dark')
await auditReducedMotion('light')
await auditReducedMotion('dark')

await fs.writeFile(path.join(outDir,'issues.json'),JSON.stringify(issues,null,2))
await fs.writeFile(path.join(outDir,'observations.json'),JSON.stringify(observations,null,2))
await fs.writeFile(path.join(outDir,'requests.txt'),requests.join('\n'))
await fs.writeFile(path.join(outDir,'unhandled.txt'),unhandled.join('\n'))
await fs.writeFile(path.join(outDir,'console-errors.txt'),runtimeErrors.join('\n'))
await fs.writeFile(path.join(outDir,'qa-summary.txt'),[
  'responsive=1024/768/375 + 720px 200%-zoom proxy',
  'themes=light/dark',
  'long-i18n=en-US mobile/responsive smoke',
  'mobile-overlays=sidebar/command/notifications when available',
  'desktop-overlay=config drawer',
  'keyboard=sampled tab order and focus indicators',
  'a11y=accessible names + WCAG 2.2 AA 24px persistent header target floor',
  'reduced-motion=browser preference emulated',
  `issues=${issues.length}`,
  `consoleErrors=${runtimeErrors.length}`,
  `unhandledApiRequests=${unhandled.length}`,
].join('\n'))
await browser.close()

if(runtimeErrors.length||unhandled.length||issues.some(x=>x.severity==='P0'||x.severity==='P1')){
  console.error(JSON.stringify({issues,runtimeErrors,unhandled},null,2))
  process.exit(1)
}
'''
Path('.aurora-interaction-a11y.mjs').write_text(prefix + custom)
