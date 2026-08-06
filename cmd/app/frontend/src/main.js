import './style.css'
import './reader.css'
import './library.css'
import 'katex/dist/katex.min.css'
import renderMathInElement from 'katex/contrib/auto-render'

const backend = () => window.go?.ui?.App
const app = document.querySelector('#app')

app.innerHTML = `
  <header><div><span class="eyebrow">LOCAL LEARNING WORKBENCH</span><h1>tutorio</h1></div></header>
  <main>
    <section class="create">
      <h2>Compile a tutorial</h2>
      <p>Turn a long-form tutorial into a structured, reusable guide.</p>
      <form id="compile-form"><label for="url">YouTube URL</label><div class="row"><input id="url" type="url" placeholder="https://youtube.com/watch?v=…" required><button id="compile-youtube" disabled>Compile guide</button></div></form><div class="import-row"><span>Or compile a local source.</span><div class="import-actions"><button class="quiet" id="import-pdf" disabled>Import PDF</button><button class="quiet" id="import-transcript">Import transcript</button></div></div>
      <div id="message-row" class="message-row" hidden><div id="message" role="status"></div><button id="dismiss-message" class="message-dismiss" type="button" aria-label="Dismiss message" title="Dismiss">×</button></div><div id="progress" class="progress" hidden><div></div></div>
    </section>
    <section id="library"><aside id="setup-status" class="setup-status" hidden aria-live="polite"></aside><div id="job-area" hidden><div class="section-head"><h2>Compilation queue</h2></div><div id="jobs" class="jobs"></div></div><div class="section-head"><h2>Library</h2><button class="quiet" id="refresh">Refresh</button></div><div id="guides" class="guides"><p class="muted">No guides loaded.</p></div></section>
    <section id="reader" hidden><div class="reader-actions"><button class="quiet back" id="back">← Library</button><details class="export-menu"><summary class="quiet">Export</summary><div><button data-export-format="html">HTML</button><button class="quiet" data-export-format="markdown">Markdown</button></div></details></div><div id="guide-content"></div></section>
    <button id="back-to-top" class="back-to-top" hidden aria-label="Back to top">↑ <span>Top</span></button>
    <aside id="evidence-panel" class="evidence-panel" hidden aria-label="Source evidence"><div class="evidence-card"><button class="quiet evidence-close" id="close-evidence" aria-label="Close evidence">Close</button><div id="evidence-content"></div></div></aside>
    <div id="image-lightbox" class="image-lightbox" hidden role="dialog" aria-modal="true" aria-label="Zoomed source page"><div class="lightbox-toolbar"><button class="quiet" data-zoom-out aria-label="Zoom out">−</button><button class="quiet" data-zoom-reset>100%</button><button class="quiet" data-zoom-in aria-label="Zoom in">+</button><button class="quiet" data-zoom-close>Close</button></div><div class="lightbox-stage"><img alt=""></div></div>
  </main>`

const message = document.querySelector('#message')
const messageRow = document.querySelector('#message-row')
const dismissMessage = document.querySelector('#dismiss-message')
const progress = document.querySelector('#progress')
const guides = document.querySelector('#guides')
const create = document.querySelector('.create')
const library = document.querySelector('#library')
const reader = document.querySelector('#reader')
const backToTop = document.querySelector('#back-to-top')
const guideContent = document.querySelector('#guide-content')
const evidencePanel = document.querySelector('#evidence-panel')
const evidenceContent = document.querySelector('#evidence-content')
const imageLightbox = document.querySelector('#image-lightbox')
const lightboxStage = imageLightbox.querySelector('.lightbox-stage')
const lightboxImage = imageLightbox.querySelector('img')
const jobs = document.querySelector('#jobs')
const jobArea = document.querySelector('#job-area')
const setupStatus = document.querySelector('#setup-status')
const compileYouTube = document.querySelector('#compile-youtube')
const importPDF = document.querySelector('#import-pdf')
let currentGuide = null
let currentSections = []
let activeRegeneration = null
let libraryScrollY = 0
const visualEvidenceCache = new Map()
const pendingToolPaths = new Map()
const dirtyToolPaths = new Set()
const guidePositionKey=id=>`tutorio:guide-position:${id}`
const readGuidePosition=id=>{try{return Math.max(0,Number(localStorage.getItem(guidePositionKey(id)))||0)}catch{return 0}}
const saveGuidePosition=()=>{if(reader.hidden||!currentGuide?.id)return;try{localStorage.setItem(guidePositionKey(currentGuide.id),String(Math.round(window.scrollY)))}catch{}}
const forgetGuidePosition=id=>{try{localStorage.removeItem(guidePositionKey(id))}catch{}}
const expandedSectionsKey=id=>`tutorio:expanded-sections:${id}`
const readExpandedSections=id=>{try{return new Set(JSON.parse(localStorage.getItem(expandedSectionsKey(id))||'[]').map(String))}catch{return new Set()}}
const saveExpandedSections=()=>{if(!currentGuide?.id)return;const expanded=[...guideContent.querySelectorAll('.guide-section:not(.collapsed)')].map(section=>section.dataset.sectionIndex);try{localStorage.setItem(expandedSectionsKey(currentGuide.id),JSON.stringify(expanded))}catch{}}
const forgetExpandedSections=id=>{try{localStorage.removeItem(expandedSectionsKey(id))}catch{}}
const openReferenceBlocksKey=id=>`tutorio:open-reference-blocks:${id}`
const readOpenReferenceBlocks=id=>{try{const value=localStorage.getItem(openReferenceBlocksKey(id));return value===null?null:new Set(JSON.parse(value).map(String))}catch{return null}}
const saveOpenReferenceBlocks=()=>{if(!currentGuide?.id)return;const open=[...guideContent.querySelectorAll('.reference-block[open]')].map(block=>block.dataset.referenceKey);try{localStorage.setItem(openReferenceBlocksKey(currentGuide.id),JSON.stringify(open))}catch{}}
const forgetOpenReferenceBlocks=id=>{try{localStorage.removeItem(openReferenceBlocksKey(id))}catch{}}
const escapeHTML = value => String(value ?? '').replace(/[&<>'"]/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[c]))
let messageRevision = 0
function showMessage(value,{dismissible=false,autoClear=0}={}){
  const text=String(value??'').trim()
  const revision=++messageRevision
  message.textContent=text
  messageRow.hidden=!text
  dismissMessage.hidden=!dismissible||!text
  if(autoClear>0&&text)setTimeout(()=>{if(messageRevision===revision)clearMessage()},autoClear)
}
function clearMessage(){messageRevision++;message.textContent='';messageRow.hidden=true;dismissMessage.hidden=true}
dismissMessage.addEventListener('click',clearMessage)

async function loadGuides() {
  if (!backend()) { guides.innerHTML = '<p class="muted">Run with <code>wails dev</code> to connect the library.</p>'; return }
  try {
    const items = await backend().ListGuides()
    guides.innerHTML = items.length ? items.map(g => `<article class="guide-card"><button class="card-open-hit" data-guide-id="${escapeHTML(g.id)}" aria-label="Open ${escapeHTML(g.title)}"></button><button class="quiet danger card-delete" data-delete-guide="${escapeHTML(g.id)}" title="Delete guide" aria-label="Delete ${escapeHTML(g.title)}"><span aria-hidden="true">×</span></button><span>${escapeHTML(g.source_type)}</span><h3>${escapeHTML(g.title)}</h3><p>${escapeHTML(g.overview)}</p><small>${new Date(g.created_at).toLocaleString()}</small><strong>Open guide →</strong></article>`).join('') : '<p class="muted">Your generated guides will appear here.</p>'
    await Promise.all([loadJobs(),loadSystemStatus()])
  } catch (err) { guides.innerHTML = `<p class="error">${escapeHTML(err)}</p>` }
}

async function loadSystemStatus(){
  if(!backend()?.GetSystemStatus)return
  try{
    const report=await backend().GetSystemStatus()
    renderSystemStatus(report)
  }catch(err){
    setupStatus.innerHTML=`<div class="setup-heading"><div><span class="eyebrow">LOCAL SETUP</span><h2>Could not check local dependencies</h2><p>${escapeHTML(err)}</p></div><button class="quiet" data-check-system>Retry</button></div>`
    setupStatus.hidden=false
  }
}
function renderSystemStatus(report){
  const checks=asArray(report.checks)
  const status=id=>checks.find(check=>check.id===id)?.status
  const youtubeReady=status('yt-dlp')==='ready'
  const pdfReady=status('pdftotext')==='ready'
  compileYouTube.disabled=!youtubeReady
  compileYouTube.title=youtubeReady?'':'Install or configure yt-dlp to compile YouTube guides.'
  importPDF.disabled=!pdfReady
  importPDF.title=pdfReady?'':'Install or configure pdftotext to compile PDFs.'
  for(const tool of asArray(report.tools))if(!dirtyToolPaths.has(tool.id))pendingToolPaths.set(tool.id,tool.path||'')
  const issues=checks.filter(check=>check.status!=='ready')
  if(!issues.length){setupStatus.hidden=true;setupStatus.innerHTML='';return}
  const missingToolIDs=new Set(issues.map(check=>check.id).filter(id=>['yt-dlp','pdftotext','pdftocairo'].includes(id)))
  const toolRows=asArray(report.tools).filter(tool=>missingToolIDs.has(tool.id)).map(tool=>`<div class="tool-path-row" data-tool-row="${escapeHTML(tool.id)}"><label for="tool-${escapeHTML(tool.id)}">${escapeHTML(tool.label)} path</label><input id="tool-${escapeHTML(tool.id)}" data-tool-input="${escapeHTML(tool.id)}" value="${escapeHTML(pendingToolPaths.get(tool.id)||'')}" required><button type="button" data-save-tool="${escapeHTML(tool.id)}"${dirtyToolPaths.has(tool.id)?'':' disabled'}>Save</button><small data-tool-hint>${dirtyToolPaths.has(tool.id)?'Edited — save to validate and activate this path.':'Enter a valid executable path to enable this feature.'}</small></div>`).join('')
  setupStatus.innerHTML=`<div class="setup-heading"><div><span class="eyebrow">LOCAL SETUP</span><h2>Some features need attention</h2></div></div><ul>${issues.map(check=>`<li class="setup-${escapeHTML(check.status)}"><strong>${escapeHTML(check.name)}</strong><span>${escapeHTML(check.message)}</span>${check.action?`<small>${escapeHTML(check.action)}</small>`:''}</li>`).join('')}</ul><div class="tool-path-form">${toolRows}<div class="setup-actions"><span>Configuration:</span><code>${escapeHTML(report.config_path)}</code><small>You can also edit this file manually.</small></div></div>`
  setupStatus.hidden=false
}

const durationLabel=milliseconds=>{const seconds=Math.max(0,Math.round((Number(milliseconds)||0)/1000)),hours=Math.floor(seconds/3600),minutes=Math.floor((seconds%3600)/60),remainder=seconds%60;return hours?`${hours}h ${minutes}m ${remainder}s`:minutes?`${minutes}m ${remainder}s`:`${remainder}s`}
const tokenRateLabel=(tokens,milliseconds)=>Number(tokens)>0&&Number(milliseconds)>0?`${(Number(tokens)*1000/Number(milliseconds)).toFixed(1)} tok/s`:''
const countLabel=value=>Math.max(0,Number(value)||0).toLocaleString()
const extractionMethodLabel=value=>({'youtube-captions':'YouTube captions','pdf-text':'PDF text','imported-transcript':'Imported transcript'}[value]||String(value||'Extracted text'))
const elapsedLabel=durationLabel
const regenerationButtonLabel=()=>activeRegeneration?`Regenerating… ${elapsedLabel(Date.now()-activeRegeneration.startedAt)}`:'Regenerate'
function updateActiveRegenerationFeedback(){
  guideContent.querySelectorAll('[data-regenerate-section]').forEach(button=>{const selected=activeRegeneration&&currentGuide?.id===activeRegeneration.guideID&&Number(button.dataset.regenerateSection)===activeRegeneration.index;button.disabled=Boolean(activeRegeneration);button.textContent=selected?regenerationButtonLabel():'Regenerate'})
  guideContent.querySelectorAll('[data-delve-section]').forEach(button=>{button.disabled=Boolean(activeRegeneration)})
  if(activeRegeneration&&!library.hidden){showMessage(`Regenerating “${activeRegeneration.sectionTitle}” of “${activeRegeneration.guideTitle}” · ${elapsedLabel(Date.now()-activeRegeneration.startedAt)} elapsed`);progress.hidden=false;progress.classList.add('indeterminate')}
}
async function loadJobs(){const items=(await backend().ListJobs()).filter(job=>job.status!=='completed'&&job.status!=='cancelled');jobArea.hidden=items.length===0;jobs.innerHTML=items.map(job=>{const active=job.status==='running'&&job.stage==='generating'&&job.total;const position=active?`section ${Math.min(job.current+1,job.total)} of ${job.total}`:(job.total?`${job.current}/${job.total}`:'');const timer=active?` · <span data-job-elapsed data-started="${escapeHTML(job.updated_at)}"></span>`:'';let action;if(job.status==='failed')action=`<div class="job-actions"><button class="quiet" data-show-job="${escapeHTML(job.id)}">Diagnostics</button><button class="quiet" data-retry-job="${escapeHTML(job.id)}">Retry failed section</button></div>`;else if(job.status==='pending')action=`<div class="job-actions"><button class="quiet" data-run-first-job="${escapeHTML(job.id)}" title="Pause the active compilation and run this one first">Run first</button><button class="quiet" data-cancel-job="${escapeHTML(job.id)}">Cancel</button></div>`;else action=`<button class="quiet" data-cancel-job="${escapeHTML(job.id)}">Cancel</button>`;return `<div class="job" data-job="${escapeHTML(job.id)}"><div><strong>${escapeHTML(job.source_title||job.source_uri)}</strong><span>${escapeHTML(job.status)} · ${escapeHTML(job.stage)}${position?` · ${position}`:''}${timer}</span>${job.error?`<small>${escapeHTML(job.error)}</small>`:''}</div>${action}</div>`}).join('');updateJobTimers()}
function updateJobTimers(){document.querySelectorAll('[data-job-elapsed]').forEach(element=>{const elapsed=Date.now()-new Date(element.dataset.started).getTime();element.textContent=`${elapsedLabel(elapsed)} elapsed${elapsed>90000?' · slower than usual':''}`;element.classList.toggle('slow',elapsed>90000)})}

const asArray = values => Array.isArray(values) ? values : values == null ? [] : [values]
const meaningfulValues = values => asArray(values).filter(value => value != null && !['', '{}', '[]', 'null'].includes(String(value).trim()))
const renderList = (title, values = []) => { const filtered = meaningfulValues(values); return filtered.length ? `<section><h2>${title}</h2><ul>${filtered.map(value => `<li>${escapeHTML(value)}</li>`).join('')}</ul></section>` : '' }
const overviewParagraphs=value=>{const blocks=String(value??'').split(/\n\s*\n/).map(block=>block.trim()).filter(Boolean);if(blocks.length!==1||blocks[0].length<360)return blocks;const sentences=blocks[0].match(/[^.!?]+[.!?]+(?:["')\]]+)?|[^.!?]+$/g)||blocks;const paragraphs=[];let current='';for(const sentence of sentences){const next=`${current} ${sentence.trim()}`.trim();if(current&&next.length>280){paragraphs.push(current);current=sentence.trim()}else current=next}if(current)paragraphs.push(current);return paragraphs}
const renderOverview=value=>`<div class="lead">${overviewParagraphs(value).map(paragraph=>`<p>${escapeHTML(paragraph)}</p>`).join('')}</div>`
const renderSectionOverview=value=>{const paragraphs=overviewParagraphs(value);return paragraphs.length?`<div class="section-overview"><span class="eyebrow">SECTION OVERVIEW</span>${paragraphs.map(paragraph=>`<p>${escapeHTML(paragraph)}</p>`).join('')}</div>`:''}
const repairMathText=root=>{const walker=document.createTreeWalker(root,NodeFilter.SHOW_TEXT);for(let node=walker.nextNode();node;node=walker.nextNode()){let value=node.nodeValue.replace(/\b(?:r?ightarrow)\b/g,'→').replace(/\b(?:l?eftarrow)\b/g,'←').replace(/\bullet\b/g,'\\bullet');value=value.replace(/(^|[\s({])ext\{/g,'$1\\text{').replace(/(^|[\s({])rac\{/g,'$1\\frac{');if(!value.includes('$')){const mathStart=value.search(/\\(?:text|frac|sqrt|sum|prod|mathbf|mathrm|operatorname)\b/);if(mathStart>=0)value=`${value.slice(0,mathStart)}$${value.slice(mathStart)}$`}node.nodeValue=value}renderMathInElement(root,{delimiters:[{left:'$$',right:'$$',display:true},{left:'$',right:'$',display:false}],throwOnError:false,strict:false})}
const formatTime = seconds => { const value = Math.max(0, Math.round(Number(seconds) || 0)); return `${Math.floor(value / 60)}:${String(value % 60).padStart(2, '0')}` }
const timestampURL = (uri, seconds) => { try { const url = new URL(uri); url.searchParams.set('t', `${Math.max(0, Math.round(seconds))}s`); return url.toString() } catch { return '' } }
const renderTimestamps = (values = [], sourceURI = '') => asArray(values).length ? `<div class="timestamps">${asArray(values).map(value => { const url = timestampURL(sourceURI, value.start_seconds); const label = `${formatTime(value.start_seconds)}${value.label ? ` · ${escapeHTML(value.label)}` : ''}`; return url ? `<button data-source-url="${escapeHTML(url)}" title="Open source at ${formatTime(value.start_seconds)}">${label} ↗</button>` : `<span>${label}</span>` }).join('')}</div>` : ''
const renderSourceReferences=(references,timestamps,sourceURI)=>{const values=asArray(references);if(!values.length)return renderTimestamps(timestamps,sourceURI);return `<div class="source-links"><h4>Sources</h4><div class="timestamps">${values.map((value,index)=>{if(value.kind==='page'){const start=value.page_start||1;const end=value.page_end||start;const location=start===end?`PDF page ${start}`:`PDF pages ${start}–${end}`;const label=values.length>1?`Source ${index+1} · ${location}`:location;return `<button data-legacy-page="${start}">${label}${value.label?` · ${escapeHTML(value.label)}`:''}</button>`}const url=timestampURL(sourceURI,value.start_seconds),location=`${formatTime(value.start_seconds)}${value.label?` · ${escapeHTML(value.label)}`:''}`,label=values.length>1?`Source ${index+1} · ${location}`:location;return url?`<button data-source-url="${escapeHTML(url)}">${label} ↗</button>`:`<span>${label}</span>`}).join('')}</div></div>`}
const renderCitations=citations=>{const values=asArray(citations);return values.length?`<div class="source-links evidence-links"><h4>Sources</h4><div class="timestamps">${values.map((citation,index)=>{const location=citation.label||'View source evidence';const label=values.length>1?`Source ${index+1} · ${location}`:location;return `<button data-citation-id="${escapeHTML(citation.id)}">${escapeHTML(label)}</button>`}).join('')}</div></div>`:''}

function renderGuide(guide) {
  const cheatSheet = meaningfulValues([...new Set([...(guide.cheat_sheet || []), ...(guide.keyboard_shortcuts || []).map(item => `${item.keys}${item.action ? ` — ${item.action}` : ''}`), ...(guide.commands || []).map(item => `${item.value}${item.description ? ` — ${item.description}` : ''}`), ...(guide.steps || []).flatMap(step => step.commands || [])])])
  const openReferenceBlocks=readOpenReferenceBlocks(guide.id)
  const renderReferenceBlock=(key,title,count,content,openByDefault=false)=>content?`<details class="reference-block" data-reference-key="${key}"${(openReferenceBlocks?.has(key)??openByDefault)?' open':''}><summary><span class="reference-chevron" aria-hidden="true">›</span><span>${title}</span><small>${count}</small></summary><div class="reference-content">${content}</div></details>`:''
  const renderReferenceList=(key,title,values,openByDefault=false)=>{const filtered=meaningfulValues(values);return filtered.length?renderReferenceBlock(key,title,`${filtered.length} ${filtered.length===1?'item':'items'}`,`<ul>${filtered.map(value=>`<li>${escapeHTML(value)}</li>`).join('')}</ul>`,openByDefault):''}
  const renderStep=(step,index,sectionNumber)=>`<article class="step" data-step-index="${index}"><div class="step-number">${sectionNumber}</div><div><div class="step-heading"><h3>${escapeHTML(step.title)}</h3><button class="quiet step-edit" data-edit-step="${index}" title="Edit this step">✎ Edit</button></div><p>${escapeHTML(step.explanation)}</p>${renderCitations(step.citations)||renderSourceReferences(step.references,step.timestamps,guide.source_uri)}${step.source_excerpt&&!step.citations?.length?`<blockquote><span>Supporting transcript</span>${escapeHTML(step.source_excerpt)}</blockquote>`:''}${renderList('Actions',step.actions)}${step.commands?.length?`<h4>Commands</h4>${step.commands.map(command=>`<pre><code>${escapeHTML(command)}</code></pre>`).join('')}`:''}${renderList('Warnings',step.warnings)}</div></article>`
  const indices=currentSections.length?currentSections.map(section=>section.index):[...new Set((guide.steps||[]).map(step=>step.source_segment??0))]
  const expandedSections=readExpandedSections(guide.id)
  const guideSections=indices.map((sectionIndex,position)=>{
    const source=currentSections.find(section=>section.index===sectionIndex)
    const indexedSteps=(guide.steps||[]).map((step,index)=>({step,index})).filter(item=>(item.step.source_segment??0)===sectionIndex)
    const deepDive=(guide.deep_dives||[]).find(item=>item.source_segment===sectionIndex)
    const rawTitle=source?.guide?.title||`Section ${position+1}`
    const title=rawTitle.replace(/^section\s+\d+\s*[:\-–—·]?\s*/i,'').trim()||rawTitle
    const sectionOverview=renderSectionOverview(source?.guide?.overview)
    const timestamps=indexedSteps.flatMap(item=>item.step.timestamps||[])
    const start=timestamps.length?Math.min(...timestamps.map(item=>Number(item.start_seconds)||0)):0
    const end=timestamps.length?Math.max(...timestamps.map(item=>Number(item.end_seconds)||0)):start
    const pageReference=source?.transcript?.reference?.kind==='page'?source.transcript.reference:null
    const range=pageReference?(pageReference.page_start===pageReference.page_end?`PDF page ${pageReference.page_start}`:`PDF pages ${pageReference.page_start}–${pageReference.page_end}`):`${formatTime(start)}–${formatTime(end)}`
    const stepLabel=`${indexedSteps.length} ${indexedSteps.length===1?'step':'steps'}`
    const isExpanded=expandedSections.has(String(sectionIndex))
    const regenerating=activeRegeneration&&activeRegeneration.guideID===guide.id&&activeRegeneration.index===sectionIndex
    const actions=source?`<div class="section-actions"><button class="quiet" data-delve-section="${sectionIndex}"${activeRegeneration?' disabled':''}>${deepDive?'Refresh deep dive':'Delve deeper'}</button><button class="quiet" data-regenerate-section="${sectionIndex}"${activeRegeneration?' disabled':''}>${regenerating?regenerationButtonLabel():'Regenerate'}</button></div>`:''
    const dive=deepDive?`<article class="deep-dive"><span class="eyebrow">DEEP DIVE · ${escapeHTML(deepDive.model||'LOCAL MODEL')}</span><h3>${escapeHTML(deepDive.title)}</h3><p>${escapeHTML(deepDive.explanation)}</p>${renderList('Key points',deepDive.key_points)}${renderList('Examples',deepDive.examples)}${renderList('Caveats',deepDive.caveats)}${renderList('Source evidence',deepDive.evidence)}</article>`:''
    return `<section class="guide-section${isExpanded?'':' collapsed'}" id="source-section-${sectionIndex}" data-section-index="${sectionIndex}"><div class="guide-section-head"><button class="section-toggle" data-toggle-section aria-expanded="${isExpanded}"><span class="section-chevron" aria-hidden="true">›</span><span class="section-summary"><span class="eyebrow">SECTION ${position+1}</span><span class="section-title">${escapeHTML(title)}</span><span class="section-range">${escapeHTML(range)} · ${stepLabel}</span></span></button>${actions}</div><div class="section-body"${isExpanded?'':' hidden'}>${sectionOverview}<div class="steps">${indexedSteps.map((item,stepIndex)=>renderStep(item.step,item.index,stepIndex+1)).join('')}</div>${dive}</div></section>`
  }).join('')
  const sectionControls=indices.length?`<div class="section-index-head"><div><h2>Sections</h2><p class="muted">Select a section to read it.</p></div><div><button class="quiet" data-expand-all>Expand all</button><button class="quiet" data-collapse-all>Collapse all</button></div></div>`:''
  const commands = guide.commands?.length ? renderReferenceBlock('commands','Commands',`${guide.commands.length} ${guide.commands.length===1?'command':'commands'}`,guide.commands.map(command => `<div class="command"><pre><code>${escapeHTML(command.value)}</code></pre><p>${escapeHTML(command.description)}</p></div>`).join('')) : ''
  const shortcuts = guide.keyboard_shortcuts?.length ? renderReferenceBlock('shortcuts','Keyboard shortcuts',`${guide.keyboard_shortcuts.length} ${guide.keyboard_shortcuts.length===1?'shortcut':'shortcuts'}`,`<div class="shortcut-grid">${guide.keyboard_shortcuts.map(item => `<div><kbd>${escapeHTML(item.keys)}</kbd><span>${escapeHTML(item.action)}${item.context ? ` · ${escapeHTML(item.context)}` : ''}</span></div>`).join('')}</div>`) : ''
  const promptRate=tokenRateLabel(guide.generation?.prompt_tokens,guide.generation?.prompt_duration_milliseconds)
  const outputRate=tokenRateLabel(guide.generation?.output_tokens,guide.generation?.output_duration_milliseconds)
  const rateDetails=`${promptRate?`<div><dt>Prompt speed</dt><dd>${promptRate}</dd></div>`:''}${outputRate?`<div><dt>Generation speed</dt><dd>${outputRate}</dd></div>`:''}`
  const sourceMetrics=guide.source_metrics||{}
  const sourceContent=sourceMetrics.characters||sourceMetrics.words?`<div><dt>Source content</dt><dd>${countLabel(sourceMetrics.words)} words · ${countLabel(sourceMetrics.characters)} chars</dd></div>`:''
  const sourceExtent=[sourceMetrics.duration_seconds?durationLabel(sourceMetrics.duration_seconds*1000):'',sourceMetrics.physical_pages?`${countLabel(sourceMetrics.physical_pages)} PDF pages`:'',sourceMetrics.extraction_method?extractionMethodLabel(sourceMetrics.extraction_method):''].filter(Boolean).join(' · ')
  const sourceDetails=`${sourceContent}${sourceExtent?`<div><dt>Source</dt><dd>${escapeHTML(sourceExtent)}</dd></div>`:''}`
  const generation = guide.generation?.model ? `<section class="generation"><h2>Generation details</h2><dl><div><dt>Model</dt><dd>${escapeHTML(guide.generation.model)}</dd></div><div><dt>Sections</dt><dd>${guide.generation.segment_count}</dd></div><div><dt>Duration</dt><dd>${durationLabel(guide.generation.duration_milliseconds)}</dd></div><div><dt>Tokens</dt><dd>${guide.generation.prompt_tokens} in · ${guide.generation.output_tokens} out</dd></div>${rateDetails}${sourceDetails}</dl></section>` : ''
  const overviewStatus=guide.overview_generation?.status||'missing'
  const synthesizedOverview=['ready','stale'].includes(overviewStatus)?renderOverview(guide.overview):''
  const legacyOverview=currentSections.length?'':renderOverview(guide.overview)
  const overviewAction=currentSections.length&&overviewStatus!=='ready'?`<div class="overview-action"><button class="quiet" data-generate-overview>${overviewStatus==='stale'?'Refresh overview':'Generate overview'}</button>${overviewStatus==='failed'?`<span>The previous overview attempt failed. You can retry without recompiling the guide.</span>`:overviewStatus==='stale'?'<span>A section changed after this overview was written.</span>':'<span>Create a short introduction from the saved section summaries.</span>'}</div>`:''
  guideContent.innerHTML = `<article class="guide"><span class="eyebrow">${escapeHTML(String(guide.source_type||'').toUpperCase())} GUIDE</span><div class="guide-title"><h1>${escapeHTML(guide.title)}</h1><button class="quiet title-edit" data-edit-title title="Rename this guide">✎ Rename</button></div>${synthesizedOverview||legacyOverview}${overviewAction}${generation}<section class="outcome"><h2>Final outcome</h2><p>${escapeHTML(guide.final_outcome)}</p></section>${renderReferenceList('prerequisites','Prerequisites',guide.prerequisites)}${sectionControls}<div class="guide-sections">${guideSections}</div><div class="reference-blocks">${renderReferenceList('concepts','Important concepts',guide.important_concepts)}${commands}${shortcuts}${renderReferenceList('warnings','Warnings',guide.warnings)}${renderReferenceList('mistakes','Common mistakes',guide.common_mistakes)}${renderReferenceList('cheat-sheet','Cheat sheet',cheatSheet)}${renderReferenceList('appendix','Appendix',guide.appendix)}</div></article>`
  repairMathText(guideContent)
}

function renderTitleEditor() {
  const container=guideContent.querySelector('.guide-title');if(!container)return
  container.innerHTML=`<form class="title-editor"><label for="guide-title-input">Guide title</label><div><input id="guide-title-input" name="title" value="${escapeHTML(currentGuide.title)}" maxlength="240" required><button type="submit">Save</button><button type="button" class="quiet" data-cancel-title>Cancel</button></div></form>`
  container.querySelector('[data-cancel-title]').addEventListener('click',()=>renderGuide(currentGuide))
  container.querySelector('form').addEventListener('submit',saveGuideTitle)
  const input=container.querySelector('input');input.focus();input.select()
}

async function saveGuideTitle(event) {
  event.preventDefault();const form=event.currentTarget;const title=form.elements.title.value.trim();if(!title){form.elements.title.focus();return}
  const buttons=form.querySelectorAll('button');buttons.forEach(button=>button.disabled=true)
  try{const updated=structuredClone(currentGuide);updated.title=title;currentGuide=await backend().SaveGuide(updated);renderGuide(currentGuide);await loadGuides();showMessage('Guide renamed.',{dismissible:true})}
  catch(err){showMessage(err,{dismissible:true});buttons.forEach(button=>button.disabled=false)}
}

function renderStepEditor(index) {
  const step=currentGuide.steps[index];const article=guideContent.querySelector(`[data-step-index="${index}"]`);if(!step||!article)return
  const sectionNumber=article.querySelector('.step-number')?.textContent||step.number
  article.innerHTML=`<div class="step-number">${escapeHTML(sectionNumber)}</div><form class="inline-editor"><label>Title<input name="title" value="${escapeHTML(step.title)}" required></label><label>Explanation<textarea name="explanation" required>${escapeHTML(step.explanation)}</textarea></label><label>Actions, one per line<textarea name="actions">${escapeHTML((step.actions||[]).join('\n'))}</textarea></label><label>Commands, one per line<textarea name="commands">${escapeHTML((step.commands||[]).join('\n'))}</textarea></label><label>Warnings, one per line<textarea name="warnings">${escapeHTML((step.warnings||[]).join('\n'))}</textarea></label><div class="editor-actions"><button type="submit">Save step</button><button type="button" class="quiet" data-cancel-step>Cancel</button></div></form>`
  article.querySelector('[data-cancel-step]').addEventListener('click',()=>renderGuide(currentGuide))
  article.querySelector('form').addEventListener('submit',event=>saveStep(event,index))
  article.querySelector('input').focus()
}

async function saveStep(event,index) {
  event.preventDefault();const form=event.currentTarget;const updated=structuredClone(currentGuide);const step=updated.steps[index];step.title=form.elements.title.value.trim();step.explanation=form.elements.explanation.value.trim();for(const field of ['actions','commands','warnings'])step[field]=form.elements[field].value.split('\n').map(value=>value.trim()).filter(Boolean);const buttons=form.querySelectorAll('button');buttons.forEach(button=>button.disabled=true);try{currentGuide=await backend().SaveGuide(updated);renderGuide(currentGuide);showMessage('Step saved.',{dismissible:true})}catch(err){showMessage(err,{dismissible:true});buttons.forEach(button=>button.disabled=false)}
}

async function openGuide(id) {
  libraryScrollY=window.scrollY
  try { currentGuide = await backend().GetGuide(id); currentSections = await backend().ListGuideSections(id); renderGuide(currentGuide); create.hidden = true; library.hidden = true; reader.hidden = false; const position=readGuidePosition(id);requestAnimationFrame(()=>{const maximum=Math.max(0,document.documentElement.scrollHeight-window.innerHeight);window.scrollTo({top:Math.min(position,maximum),behavior:'auto'});lastWindowScroll=window.scrollY}) }
  catch (err) { showMessage(err,{dismissible:true}) }
}

document.querySelector('#compile-form').addEventListener('submit', async event => {
  event.preventDefault(); const button = event.currentTarget.querySelector('button'); button.disabled = true; progress.hidden = false; progress.classList.remove('indeterminate'); progress.querySelector('div').style.width = '4%'; showMessage('Adding compilation to the local queue…')
  let queued=false
  try { await backend().QueueYouTube(document.querySelector('#url').value); queued=true;showMessage('Compilation queued. You can keep using the library.',{dismissible:true}); await loadJobs() }
	catch (err) { showMessage(err,{dismissible:true}) }
	finally { button.disabled = false; if (!queued) progress.hidden = true }
})
document.querySelector('#refresh').addEventListener('click', loadGuides)
setupStatus.addEventListener('click',async event=>{const check=event.target.closest('[data-check-system]');if(check){check.disabled=true;await loadSystemStatus();return}const button=event.target.closest('[data-save-tool]');if(!button)return;const id=button.dataset.saveTool,row=button.closest('[data-tool-row]'),input=row.querySelector('[data-tool-input]'),hint=row.querySelector('[data-tool-hint]');button.disabled=true;button.textContent='Validating…';try{const report=await backend().SaveToolPath(id,input.value);dirtyToolPaths.delete(id);pendingToolPaths.set(id,input.value.trim());showMessage(`${row.querySelector('label').textContent} saved and activated.`,{dismissible:true,autoClear:5000});renderSystemStatus(report)}catch(err){hint.textContent=String(err);hint.classList.add('error');button.disabled=false;button.textContent='Save'}})
setupStatus.addEventListener('input',event=>{const input=event.target.closest('[data-tool-input]');if(!input)return;const id=input.dataset.toolInput,row=input.closest('[data-tool-row]'),button=row.querySelector('[data-save-tool]'),hint=row.querySelector('[data-tool-hint]');pendingToolPaths.set(id,input.value);dirtyToolPaths.add(id);button.disabled=false;button.textContent='Save';hint.textContent='Edited — save to validate and activate this path.';hint.classList.remove('error')})
async function selectAndQueueFile(button,select){button.disabled=true;try{const job=await select();if(job?.id){showMessage('File compilation queued.',{dismissible:true});progress.hidden=false;progress.classList.remove('indeterminate');progress.querySelector('div').style.width='4%';await loadJobs()}}catch(err){showMessage(err,{dismissible:true})}finally{button.disabled=false}}
importPDF.addEventListener('click',event=>selectAndQueueFile(event.currentTarget,()=>backend().SelectAndQueuePDF()))
document.querySelector('#import-transcript').addEventListener('click',event=>selectAndQueueFile(event.currentTarget,()=>backend().SelectAndQueueTranscript()))
guides.addEventListener('click',async event=>{const remove=event.target.closest('[data-delete-guide]');if(remove){remove.disabled=true;try{const deleted=await backend().DeleteGuide(remove.dataset.deleteGuide);if(deleted){forgetGuidePosition(remove.dataset.deleteGuide);forgetExpandedSections(remove.dataset.deleteGuide);forgetOpenReferenceBlocks(remove.dataset.deleteGuide);showMessage('Guide deleted.',{dismissible:true});await loadGuides()}else{remove.disabled=false}}catch(err){showMessage(err,{dismissible:true});remove.disabled=false}return}const open=event.target.closest('[data-guide-id]');if(open)openGuide(open.dataset.guideId)})
jobs.addEventListener('click',async event=>{const retry=event.target.closest('[data-retry-job]');const runFirst=event.target.closest('[data-run-first-job]');const cancel=event.target.closest('[data-cancel-job]');const diagnostic=event.target.closest('[data-show-job]');const button=retry||runFirst||cancel||diagnostic;if(!button)return;button.disabled=true;try{if(diagnostic){const sections=await backend().GetJobSections(diagnostic.dataset.showJob);const failed=[...sections].reverse().find(section=>section.error||section.raw_response);const card=diagnostic.closest('[data-job]');card.querySelector('.job-diagnostic')?.remove();card.insertAdjacentHTML('beforeend',`<details class="job-diagnostic" open><summary>Section ${(failed?.index??0)+1} model response</summary><p>${escapeHTML(failed?.error||'No section error was recorded.')}</p>${failed?.raw_response?`<pre><code>${escapeHTML(failed.raw_response)}</code></pre>`:'<p>No raw response was returned by the model.</p>'}</details>`);button.disabled=false;return}if(retry){await backend().RetryJob(retry.dataset.retryJob);showMessage('Failed section requeued; completed sections will be reused.',{dismissible:true})}else if(runFirst){await backend().RunFirstJob(runFirst.dataset.runFirstJob);showMessage('Pausing the active compilation; this one will run first.')}else{await backend().CancelJob(cancel.dataset.cancelJob);showMessage('Compilation cancelled.',{dismissible:true})}await loadJobs()}catch(err){showMessage(err,{dismissible:true});button.disabled=false}})
function returnToLibrary(){saveGuidePosition();closeEvidence();reader.hidden=true;create.hidden=false;library.hidden=false;backToTop.hidden=true;updateActiveRegenerationFeedback();requestAnimationFrame(()=>window.scrollTo({top:libraryScrollY,behavior:'auto'}))}
document.querySelector('#back').addEventListener('click',returnToLibrary)
backToTop.addEventListener('click',()=>window.scrollTo({top:0,behavior:'smooth'}))
document.querySelector('.export-menu').addEventListener('click',async event=>{const button=event.target.closest('[data-export-format]');if(!button||!currentGuide)return;button.disabled=true;try{const path=button.dataset.exportFormat==='html'?await backend().ExportHTML(currentGuide.id):await backend().ExportMarkdown(currentGuide.id);if(path)showMessage(`Exported to ${path}`,{dismissible:true});button.closest('details').open=false}catch(err){showMessage(err,{dismissible:true})}finally{button.disabled=false}})
guideContent.addEventListener('click', event => { const link = event.target.closest('[data-source-url]'); if (link) window.runtime?.BrowserOpenURL?.(link.dataset.sourceUrl) })
function closeEvidence(){closeImageZoom();evidencePanel.hidden=true;evidencePanel.dataset.citationId='';evidenceContent.innerHTML='';document.body.classList.remove('evidence-open')}
function showEvidencePanel(content){evidenceContent.innerHTML=content;evidencePanel.hidden=false;document.body.classList.add('evidence-open')}
document.querySelector('#close-evidence').addEventListener('click',closeEvidence)
evidencePanel.addEventListener('click',event=>{if(event.target===evidencePanel)closeEvidence()})
guideContent.addEventListener('click',async event=>{const link=event.target.closest('[data-citation-id]');if(!link)return;const citationID=link.dataset.citationId;evidencePanel.dataset.citationId=citationID;showEvidencePanel('<p class="muted">Loading source evidence…</p>');try{const value=await backend().GetCitationEvidence(currentGuide.id,citationID);const page=value.chunk?.location?.physical_page;const before=value.previous?.text;const after=value.next?.text;if(evidencePanel.dataset.citationId!==citationID)return;showEvidencePanel(`<span class="eyebrow">SOURCE EVIDENCE</span><h2>${escapeHTML(value.source?.title||currentGuide.title)}</h2><p class="evidence-location">${page?`PDF page ${page}`:'Source location unavailable'}</p><div class="evidence-context">${before?`<p>${escapeHTML(before)}</p>`:''}<mark>${escapeHTML(value.chunk?.text||'No extracted text is available.')}</mark>${after?`<p>${escapeHTML(after)}</p>`:''}</div>${page?'<section class="visual-evidence"><h3>Page preview</h3><div data-visual-preview><p class="muted">Rendering this PDF page locally…</p></div></section>':''}<button data-open-citation="${escapeHTML(citationID)}">Open full PDF</button>${page?`<p class="muted">If your system viewer does not jump automatically, navigate to PDF page ${page}.</p>`:''}`);if(page)loadCitationVisual(citationID,page)}catch(err){showEvidencePanel(`<h2>Evidence unavailable</h2><p>${escapeHTML(err)}</p>`)}})
async function loadCitationVisual(citationID,page){const target=evidenceContent.querySelector('[data-visual-preview]');if(!target)return;const cacheKey=`${currentGuide.source_id}:${page}`;try{let visual=visualEvidenceCache.get(cacheKey);if(!visual){visual=await backend().GetCitationVisual(currentGuide.id,citationID);visualEvidenceCache.set(cacheKey,visual)}if(evidencePanel.dataset.citationId!==citationID)return;target.innerHTML=`<button class="visual-preview-button" data-zoom-image title="Click to enlarge"><img src="${escapeHTML(visual.data_url)}" alt="Rendered PDF page ${Number(visual.physical_page)||''}"><span>Click to zoom</span></button>`}catch(err){if(evidencePanel.dataset.citationId===citationID)target.innerHTML=`<p class="muted">Page preview unavailable: ${escapeHTML(err)}</p>`}}
let lightboxScale=1,lightboxX=0,lightboxY=0,lightboxDrag=null
function updateImageZoom(){lightboxImage.style.transform=`translate(${lightboxX}px,${lightboxY}px) scale(${lightboxScale})`;imageLightbox.querySelector('[data-zoom-reset]').textContent=`${Math.round(lightboxScale*100)}%`;lightboxStage.classList.toggle('can-pan',lightboxScale>1)}
function setImageZoom(scale){lightboxScale=Math.min(5,Math.max(1,scale));if(lightboxScale===1)lightboxX=lightboxY=0;updateImageZoom()}
function openImageZoom(image){lightboxImage.src=image.src;lightboxImage.alt=image.alt;lightboxScale=1;lightboxX=lightboxY=0;imageLightbox.hidden=false;updateImageZoom()}
function closeImageZoom(){imageLightbox.hidden=true;lightboxImage.removeAttribute('src');lightboxDrag=null}
evidenceContent.addEventListener('click',event=>{const button=event.target.closest('[data-zoom-image]');if(button)openImageZoom(button.querySelector('img'))})
imageLightbox.addEventListener('click',event=>{if(event.target===imageLightbox||event.target===lightboxStage)closeImageZoom()})
imageLightbox.querySelector('[data-zoom-close]').addEventListener('click',closeImageZoom)
imageLightbox.querySelector('[data-zoom-in]').addEventListener('click',()=>setImageZoom(lightboxScale+.25))
imageLightbox.querySelector('[data-zoom-out]').addEventListener('click',()=>setImageZoom(lightboxScale-.25))
imageLightbox.querySelector('[data-zoom-reset]').addEventListener('click',()=>setImageZoom(1))
lightboxStage.addEventListener('wheel',event=>{event.preventDefault();setImageZoom(lightboxScale+(event.deltaY<0?.2:-.2))},{passive:false})
lightboxStage.addEventListener('pointerdown',event=>{if(lightboxScale<=1)return;lightboxDrag={x:event.clientX,y:event.clientY,startX:lightboxX,startY:lightboxY};lightboxStage.setPointerCapture(event.pointerId)})
lightboxStage.addEventListener('pointermove',event=>{if(!lightboxDrag)return;lightboxX=lightboxDrag.startX+event.clientX-lightboxDrag.x;lightboxY=lightboxDrag.startY+event.clientY-lightboxDrag.y;updateImageZoom()})
lightboxStage.addEventListener('pointerup',()=>{lightboxDrag=null})
lightboxStage.addEventListener('pointercancel',()=>{lightboxDrag=null})
document.addEventListener('keydown',event=>{if(event.key!=='Escape')return;if(!imageLightbox.hidden)closeImageZoom();else if(!evidencePanel.hidden)closeEvidence()})
let lastWindowScroll=window.scrollY
let positionSaveTimer
window.addEventListener('scroll',()=>{const current=window.scrollY;const scrollingUp=current<lastWindowScroll;backToTop.hidden=reader.hidden||current<500||!scrollingUp;lastWindowScroll=current;if(!reader.hidden){clearTimeout(positionSaveTimer);positionSaveTimer=setTimeout(saveGuidePosition,150)}},{passive:true})
window.addEventListener('beforeunload',saveGuidePosition)
let edgeSwipe=null
reader.addEventListener('pointerdown',event=>{if(event.pointerType==='touch'&&event.clientX<=40&&evidencePanel.hidden)edgeSwipe={x:event.clientX,y:event.clientY,id:event.pointerId}})
reader.addEventListener('pointerup',event=>{if(!edgeSwipe||event.pointerId!==edgeSwipe.id)return;const dx=event.clientX-edgeSwipe.x,dy=Math.abs(event.clientY-edgeSwipe.y);edgeSwipe=null;if(dx>=90&&dy<=60)returnToLibrary()})
reader.addEventListener('pointercancel',()=>{edgeSwipe=null})
let horizontalGesture=0,horizontalGestureTimer
reader.addEventListener('wheel',event=>{if(!evidencePanel.hidden||Math.abs(event.deltaX)<=Math.abs(event.deltaY))return;horizontalGesture+=event.deltaX;clearTimeout(horizontalGestureTimer);horizontalGestureTimer=setTimeout(()=>{horizontalGesture=0},250);if(horizontalGesture<=-160){horizontalGesture=0;returnToLibrary()}},{passive:true})
guideContent.addEventListener('click',event=>{const link=event.target.closest('[data-legacy-page]');if(!link)return;const page=Number(link.dataset.legacyPage)||1;showEvidencePanel(`<span class="eyebrow">SOURCE REFERENCE</span><h2>${escapeHTML(currentGuide.title)}</h2><p class="evidence-location">PDF page ${page}</p><p>This older guide has a page reference but no stored source excerpt. Recompile the PDF to create exact source evidence.</p><button data-open-legacy="${page}">Open full PDF</button><p class="muted">If your system viewer does not jump automatically, navigate to PDF page ${page}.</p>`)})
evidenceContent.addEventListener('click',async event=>{const citation=event.target.closest('[data-open-citation]');const legacy=event.target.closest('[data-open-legacy]');if(!citation&&!legacy)return;const button=citation||legacy;button.disabled=true;try{if(citation)await backend().OpenCitationSource(currentGuide.id,citation.dataset.openCitation);else await backend().OpenGuideSource(currentGuide.id,Number(legacy.dataset.openLegacy)||1)}catch(err){showMessage(err,{dismissible:true});button.disabled=false}})
guideContent.addEventListener('click',event=>{const button=event.target.closest('[data-edit-step]');if(button)renderStepEditor(Number(button.dataset.editStep))})
guideContent.addEventListener('click',event=>{if(event.target.closest('[data-edit-title]'))renderTitleEditor()})
guideContent.addEventListener('click',async event=>{const button=event.target.closest('[data-generate-overview]');if(!button)return;button.disabled=true;const started=Date.now();const timer=setInterval(()=>{button.textContent=`Writing… ${elapsedLabel(Date.now()-started)}`},1000);showMessage('Writing a concise overview from the saved section summaries…');try{currentGuide=await backend().GenerateOverview(currentGuide.id);renderGuide(currentGuide);await loadGuides();showMessage('Guide overview saved.',{dismissible:true})}catch(err){showMessage(err,{dismissible:true});button.disabled=false;button.textContent='Retry overview'}finally{clearInterval(timer)}})
function setSectionExpanded(section,expanded){section.classList.toggle('collapsed',!expanded);const body=section.querySelector('.section-body');const toggle=section.querySelector('[data-toggle-section]');if(body)body.hidden=!expanded;if(toggle)toggle.setAttribute('aria-expanded',String(expanded))}
guideContent.addEventListener('click',event=>{const toggle=event.target.closest('[data-toggle-section]');const expandAll=event.target.closest('[data-expand-all]');const collapseAll=event.target.closest('[data-collapse-all]');if(toggle){const section=toggle.closest('.guide-section');setSectionExpanded(section,section.classList.contains('collapsed'));saveExpandedSections()}else if(expandAll||collapseAll){guideContent.querySelectorAll('.guide-section').forEach(section=>setSectionExpanded(section,Boolean(expandAll)));saveExpandedSections()}})
guideContent.addEventListener('toggle',event=>{if(event.target.matches('.reference-block'))saveOpenReferenceBlocks()},true)
guideContent.addEventListener('click',async event=>{const button=event.target.closest('[data-regenerate-section]');if(!button||activeRegeneration)return;const index=Number(button.dataset.regenerateSection),guideID=currentGuide.id,guideTitle=currentGuide.title,sectionTitle=button.closest('.guide-section')?.querySelector('.section-title')?.textContent?.trim()||`Section ${index+1}`;if(!await backend().ConfirmRegenerateSection(guideID,index)||activeRegeneration)return;activeRegeneration={guideID,index,guideTitle,sectionTitle,startedAt:Date.now()};progress.hidden=false;progress.classList.add('indeterminate');updateActiveRegenerationFeedback();try{const regenerated=await backend().RegenerateSection(guideID,index),sections=await backend().ListGuideSections(guideID);activeRegeneration=null;if(currentGuide?.id===guideID){currentGuide=regenerated;currentSections=sections;if(!reader.hidden)renderGuide(currentGuide)}updateActiveRegenerationFeedback();showMessage(`Section ${index+1} regenerated.`,{dismissible:true});await loadGuides()}catch(err){activeRegeneration=null;showMessage(err,{dismissible:true});progress.hidden=true;progress.classList.remove('indeterminate');updateActiveRegenerationFeedback()}})
guideContent.addEventListener('click',async event=>{const button=event.target.closest('[data-delve-section]');if(!button)return;const index=Number(button.dataset.delveSection);button.disabled=true;const started=Date.now();const original=button.textContent;const timer=setInterval(()=>{button.textContent=`Delving… ${elapsedLabel(Date.now()-started)}`},1000);showMessage(`Using the saved source transcript to delve into section ${index+1}…`);try{currentGuide=await backend().DelveSection(currentGuide.id,index);renderGuide(currentGuide);showMessage(`Section ${index+1} deep dive saved.`,{dismissible:true})}catch(err){showMessage(err,{dismissible:true});button.disabled=false;button.textContent=original}finally{clearInterval(timer)}})
let jobRefreshTimer
window.runtime?.EventsOn?.('pipeline:progress', update => {
  const sectionRegeneration=update.operation==='section_regeneration'
  if(sectionRegeneration&&activeRegeneration&&activeRegeneration.guideID===update.guide_id&&activeRegeneration.index===Number(update.section_index))updateActiveRegenerationFeedback();else if(!sectionRegeneration)showMessage(update.message,{dismissible:['failed','cancelled'].includes(update.stage),autoClear:update.stage==='complete'?5000:0})
  const complete=update.stage==='complete'
  const determinate=complete||(!sectionRegeneration&&update.total>1)
  const percent=complete?100:determinate?Math.max(4,Math.round(update.current/update.total*100)):12
  progress.hidden = false
  progress.classList.toggle('indeterminate',!determinate)
  progress.querySelector('div').style.width = `${percent}%`
	if (complete) setTimeout(() => { if(!activeRegeneration)progress.hidden = true }, 1200)
	if (update.stage === 'failed' || update.stage === 'cancelled') {progress.hidden=true;progress.classList.remove('indeterminate')}
	if(!sectionRegeneration){clearTimeout(jobRefreshTimer);jobRefreshTimer=setTimeout(()=>{loadJobs();if(complete)loadGuides()},250)}
})
loadGuides()
setInterval(updateJobTimers,1000)
setInterval(updateActiveRegenerationFeedback,1000)
setInterval(()=>{if(!setupStatus.hidden&&!dirtyToolPaths.size)loadSystemStatus()},10000)
