from pathlib import Path

path = Path('.aurora-interaction-a11y.mjs')
source = path.read_text()

old_layout = r'''async function layoutAudit(page,label){
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
}'''
new_layout = r'''async function layoutAudit(page,label){
  const data=await page.evaluate(()=>{
    const viewport=document.documentElement.clientWidth
    const offenders=[...document.querySelectorAll('body *')].flatMap(el=>{
      const s=getComputedStyle(el)
      if(s.position==='fixed'&&s.visibility==='hidden')return []
      if(el.getAttribute('aria-hidden')==='true')return []
      const r=el.getBoundingClientRect()
      if(!r.width||!r.height||s.display==='none'||s.visibility==='hidden')return []
      if(r.right<=viewport+1&&r.left>=-1)return []
      return [{
        tag:el.tagName,
        left:Math.round(r.left*10)/10,
        right:Math.round(r.right*10)/10,
        width:Math.round(r.width*10)/10,
        className:String(el.className||'').slice(0,240),
        text:(el.textContent||'').trim().replace(/\s+/g,' ').slice(0,120),
        html:el.outerHTML?.slice(0,420)||'',
      }]
    }).slice(0,20)
    return {
      scrollWidth:document.documentElement.scrollWidth,
      clientWidth:viewport,
      bodyScrollWidth:document.body.scrollWidth,
      bodyClientWidth:document.body.clientWidth,
      offenders,
    }
  })
  observations.push({type:'layout',label,...data})
  if(data.scrollWidth>data.clientWidth+1||data.bodyScrollWidth>data.bodyClientWidth+1){
    issues.push({severity:'P1',type:'horizontal-overflow',label,data})
  }
}'''
if old_layout not in source:
    raise SystemExit('layoutAudit block not found')
source = source.replace(old_layout, new_layout, 1)

old_visible = r'''    const visible=(el)=>{
      const r=el.getBoundingClientRect()
      const s=getComputedStyle(el)
      return r.width>0&&r.height>0&&s.visibility!=='hidden'&&s.display!=='none'
    }
    const nameOf=(el)=>{
      const labelledBy=el.getAttribute('aria-labelledby')
      const labelled=labelledBy?labelledBy.split(/\s+/).map(id=>document.getElementById(id)?.textContent||'').join(' ').trim():''
      return (el.getAttribute('aria-label')||labelled||el.getAttribute('title')||el.getAttribute('alt')||el.getAttribute('placeholder')||el.textContent||'').trim().replace(/\s+/g,' ').slice(0,160)
    }'''
new_visible = r'''    const visible=(el)=>{
      if(el.getAttribute('aria-hidden')==='true')return false
      const r=el.getBoundingClientRect()
      const s=getComputedStyle(el)
      if(s.clipPath==='inset(50%)'||s.clip==='rect(0px, 0px, 0px, 0px)')return false
      return r.width>1&&r.height>1&&s.visibility!=='hidden'&&s.display!=='none'
    }
    const nameOf=(el)=>{
      const labelledBy=el.getAttribute('aria-labelledby')
      const labelled=labelledBy?labelledBy.split(/\s+/).map(id=>document.getElementById(id)?.textContent||'').join(' ').trim():''
      const labels='labels' in el&&el.labels?[...el.labels].map(label=>label.textContent||'').join(' ').trim():''
      return (el.getAttribute('aria-label')||labelled||labels||el.getAttribute('title')||el.getAttribute('alt')||el.getAttribute('placeholder')||el.textContent||'').trim().replace(/\s+/g,' ').slice(0,160)
    }'''
if old_visible not in source:
    raise SystemExit('interactive visible/name block not found')
source = source.replace(old_visible, new_visible, 1)

old_keyboard = r'''      const labelledBy=el.getAttribute('aria-labelledby')
      const labelled=labelledBy?labelledBy.split(/\s+/).map(id=>document.getElementById(id)?.textContent||'').join(' ').trim():''
      const name=(el.getAttribute('aria-label')||labelled||el.getAttribute('title')||el.getAttribute('placeholder')||el.textContent||'').trim().replace(/\s+/g,' ').slice(0,120)
      const indicator=(s.outlineStyle!=='none'&&parseFloat(s.outlineWidth||'0')>0)||s.boxShadow!=='none'
      return {tag:el.tagName,name,visible:r.width>0&&r.height>0,focusVisible:el.matches(':focus-visible'),indicator,width:Math.round(r.width),height:Math.round(r.height)}'''
new_keyboard = r'''      const labelledBy=el.getAttribute('aria-labelledby')
      const labelled=labelledBy?labelledBy.split(/\s+/).map(id=>document.getElementById(id)?.textContent||'').join(' ').trim():''
      const labels='labels' in el&&el.labels?[...el.labels].map(label=>label.textContent||'').join(' ').trim():''
      const name=(el.getAttribute('aria-label')||labelled||labels||el.getAttribute('title')||el.getAttribute('placeholder')||el.textContent||'').trim().replace(/\s+/g,' ').slice(0,120)
      const indicator=(s.outlineStyle!=='none'&&parseFloat(s.outlineWidth||'0')>0)||s.boxShadow!=='none'
      return {tag:el.tagName,name,visible:r.width>1&&r.height>1&&el.getAttribute('aria-hidden')!=='true',focusVisible:el.matches(':focus-visible'),indicator,width:Math.round(r.width),height:Math.round(r.height),tabIndex:el.tabIndex,html:el.outerHTML?.slice(0,420)||'',parent:el.parentElement?.outerHTML?.slice(0,420)||''}'''
if old_keyboard not in source:
    raise SystemExit('keyboard name block not found')
source = source.replace(old_keyboard, new_keyboard, 1)

source = source.replace(
    "  await sidebarTrigger.click()\n  const mobileSidebar=page.locator('[data-mobile=\"true\"]').first()",
    "  await sidebarTrigger.click()\n  await page.waitForTimeout(500)\n  const mobileSidebar=page.locator('[data-mobile=\"true\"]').first()",
    1,
)
source = source.replace(
    "    await search.click()\n    const dialog=page.locator('[role=\"dialog\"]:visible').first()",
    "    await search.click()\n    await page.waitForTimeout(350)\n    const dialog=page.locator('[role=\"dialog\"]:visible').first()",
    1,
)
source = source.replace(
    "    await candidate.click()\n    const popover=page.locator('[data-slot=\"popover-content\"]:visible').first()",
    "    await candidate.click()\n    await page.waitForTimeout(300)\n    const popover=page.locator('[data-slot=\"popover-content\"]:visible').first()",
    1,
)
source = source.replace(
    "    await quickTools.click()\n    const toolsPopover=page.locator('[data-slot=\"popover-content\"]:visible').first()",
    "    await quickTools.click()\n    await page.waitForTimeout(300)\n    const toolsPopover=page.locator('[data-slot=\"popover-content\"]:visible').first()",
    1,
)
source = source.replace(
    "      await config.click()\n      const dialog=page.locator('[role=\"dialog\"]:visible').first()",
    "      await config.click()\n      await page.waitForTimeout(350)\n      const dialog=page.locator('[role=\"dialog\"]:visible').first()",
    1,
)

path.write_text(source)
