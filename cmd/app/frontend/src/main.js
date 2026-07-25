import './style.css'
import './reader.css'
import './library.css'

const backend = () => window.go?.ui?.App
const app = document.querySelector('#app')

app.innerHTML = `
  <header><div><span class="eyebrow">LOCAL LEARNING WORKBENCH</span><h1>tutorio</h1></div><div class="status">Local only</div></header>
  <main>
    <section class="create">
      <h2>Compile a tutorial</h2>
      <p>Turn a long-form tutorial into a structured, reusable guide.</p>
      <form id="compile-form"><label for="url">YouTube URL</label><div class="row"><input id="url" type="url" placeholder="https://youtube.com/watch?v=…" required><button>Compile guide</button></div></form><div class="import-row"><span>Or compile a PDF, TXT, SRT, or VTT file.</span><button class="quiet" id="import-file">Import file</button></div>
      <div id="message" role="status"></div><div id="progress" class="progress" hidden><div></div></div>
    </section>
    <section id="library"><div id="job-area" hidden><div class="section-head"><h2>Compilation queue</h2></div><div id="jobs" class="jobs"></div></div><div class="section-head"><h2>Library</h2><button class="quiet" id="refresh">Refresh</button></div><div id="guides" class="guides"><p class="muted">No guides loaded.</p></div></section>
    <section id="reader" hidden><div class="reader-actions"><button class="quiet back" id="back">← Library</button><button class="quiet back" id="export-guide">Export Markdown</button></div><div id="guide-content"></div></section>
    <aside id="evidence-panel" class="evidence-panel" hidden aria-label="Source evidence"><div class="evidence-card"><button class="quiet evidence-close" id="close-evidence" aria-label="Close evidence">Close</button><div id="evidence-content"></div></div></aside>
  </main>`

const message = document.querySelector('#message')
const progress = document.querySelector('#progress')
const guides = document.querySelector('#guides')
const library = document.querySelector('#library')
const reader = document.querySelector('#reader')
const guideContent = document.querySelector('#guide-content')
const evidencePanel = document.querySelector('#evidence-panel')
const evidenceContent = document.querySelector('#evidence-content')
const jobs = document.querySelector('#jobs')
const jobArea = document.querySelector('#job-area')
let currentGuide = null
let currentSections = []
const escapeHTML = value => String(value ?? '').replace(/[&<>'"]/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[c]))

async function loadGuides() {
  if (!backend()) { guides.innerHTML = '<p class="muted">Run with <code>wails dev</code> to connect the library.</p>'; return }
  try {
    const items = await backend().ListGuides()
    guides.innerHTML = items.length ? items.map(g => `<article class="guide-card"><span>${escapeHTML(g.source_type)}</span><h3>${escapeHTML(g.title)}</h3><p>${escapeHTML(g.overview)}</p><small>${new Date(g.created_at).toLocaleString()}</small><div class="card-actions"><button data-guide-id="${escapeHTML(g.id)}">Open guide →</button><button class="quiet danger" data-delete-guide="${escapeHTML(g.id)}">Delete</button></div></article>`).join('') : '<p class="muted">Your generated guides will appear here.</p>'
    await loadJobs()
  } catch (err) { guides.innerHTML = `<p class="error">${escapeHTML(err)}</p>` }
}

const elapsedLabel=milliseconds=>{const seconds=Math.max(0,Math.floor(milliseconds/1000));return seconds<60?`${seconds}s`:`${Math.floor(seconds/60)}m ${seconds%60}s`}
async function loadJobs(){const items=(await backend().ListJobs()).filter(job=>job.status!=='completed'&&job.status!=='cancelled');jobArea.hidden=items.length===0;jobs.innerHTML=items.map(job=>{const active=job.status==='running'&&job.stage==='generating'&&job.total;const position=active?`section ${Math.min(job.current+1,job.total)} of ${job.total}`:(job.total?`${job.current}/${job.total}`:'');const timer=active?` · <span data-job-elapsed data-started="${escapeHTML(job.updated_at)}"></span>`:'';const action=job.status==='failed'?`<div class="job-actions"><button class="quiet" data-show-job="${escapeHTML(job.id)}">Diagnostics</button><button class="quiet" data-retry-job="${escapeHTML(job.id)}">Retry failed section</button></div>`:`<button class="quiet" data-cancel-job="${escapeHTML(job.id)}">Cancel</button>`;return `<div class="job" data-job="${escapeHTML(job.id)}"><div><strong>${escapeHTML(job.source_uri)}</strong><span>${escapeHTML(job.status)} · ${escapeHTML(job.stage)}${position?` · ${position}`:''}${timer}</span>${job.error?`<small>${escapeHTML(job.error)}</small>`:''}</div>${action}</div>`}).join('');updateJobTimers()}
function updateJobTimers(){document.querySelectorAll('[data-job-elapsed]').forEach(element=>{const elapsed=Date.now()-new Date(element.dataset.started).getTime();element.textContent=`${elapsedLabel(elapsed)} elapsed${elapsed>90000?' · slower than usual':''}`;element.classList.toggle('slow',elapsed>90000)})}

const asArray = values => Array.isArray(values) ? values : values == null ? [] : [values]
const meaningfulValues = values => asArray(values).filter(value => value != null && !['', '{}', '[]', 'null'].includes(String(value).trim()))
const renderList = (title, values = []) => { const filtered = meaningfulValues(values); return filtered.length ? `<section><h2>${title}</h2><ul>${filtered.map(value => `<li>${escapeHTML(value)}</li>`).join('')}</ul></section>` : '' }
const formatTime = seconds => { const value = Math.max(0, Math.round(Number(seconds) || 0)); return `${Math.floor(value / 60)}:${String(value % 60).padStart(2, '0')}` }
const timestampURL = (uri, seconds) => { try { const url = new URL(uri); url.searchParams.set('t', `${Math.max(0, Math.round(seconds))}s`); return url.toString() } catch { return '' } }
const renderTimestamps = (values = [], sourceURI = '') => asArray(values).length ? `<div class="timestamps">${asArray(values).map(value => { const url = timestampURL(sourceURI, value.start_seconds); const label = `${formatTime(value.start_seconds)}${value.label ? ` · ${escapeHTML(value.label)}` : ''}`; return url ? `<button data-source-url="${escapeHTML(url)}" title="Open source at ${formatTime(value.start_seconds)}">${label} ↗</button>` : `<span>${label}</span>` }).join('')}</div>` : ''
const renderSourceReferences=(references,timestamps,sourceURI)=>{const values=asArray(references);if(!values.length)return renderTimestamps(timestamps,sourceURI);return `<div class="source-links"><h4>Sources</h4><div class="timestamps">${values.map((value,index)=>{if(value.kind==='page'){const start=value.page_start||1;const end=value.page_end||start;const location=start===end?`PDF page ${start}`:`PDF pages ${start}–${end}`;const label=values.length>1?`Source ${index+1} · ${location}`:location;return `<button data-legacy-page="${start}">${label}${value.label?` · ${escapeHTML(value.label)}`:''}</button>`}const url=timestampURL(sourceURI,value.start_seconds),location=`${formatTime(value.start_seconds)}${value.label?` · ${escapeHTML(value.label)}`:''}`,label=values.length>1?`Source ${index+1} · ${location}`:location;return url?`<button data-source-url="${escapeHTML(url)}">${label} ↗</button>`:`<span>${label}</span>`}).join('')}</div></div>`}
const renderCitations=citations=>{const values=asArray(citations);return values.length?`<div class="source-links evidence-links"><h4>Sources</h4><div class="timestamps">${values.map((citation,index)=>{const location=citation.label||'View source evidence';const label=values.length>1?`Source ${index+1} · ${location}`:location;return `<button data-citation-id="${escapeHTML(citation.id)}">${escapeHTML(label)}</button>`}).join('')}</div></div>`:''}

function renderGuide(guide) {
  const cheatSheet = meaningfulValues([...new Set([...(guide.cheat_sheet || []), ...(guide.keyboard_shortcuts || []).map(item => `${item.keys}${item.action ? ` — ${item.action}` : ''}`), ...(guide.commands || []).map(item => `${item.value}${item.description ? ` — ${item.description}` : ''}`), ...(guide.steps || []).flatMap(step => step.commands || [])])])
  const renderStep=(step,index)=>`<article class="step" data-step-index="${index}"><div class="step-number">${step.number}</div><div><div class="step-heading"><h3>${escapeHTML(step.title)}</h3><button class="quiet step-edit" data-edit-step="${index}" title="Edit this step">✎ Edit</button></div><p>${escapeHTML(step.explanation)}</p>${renderCitations(step.citations)||renderSourceReferences(step.references,step.timestamps,guide.source_uri)}${step.source_excerpt&&!step.citations?.length?`<blockquote><span>Supporting transcript</span>${escapeHTML(step.source_excerpt)}</blockquote>`:''}${renderList('Actions',step.actions)}${step.commands?.length?`<h4>Commands</h4>${step.commands.map(command=>`<pre><code>${escapeHTML(command)}</code></pre>`).join('')}`:''}${renderList('Warnings',step.warnings)}</div></article>`
  const indices=currentSections.length?currentSections.map(section=>section.index):[...new Set((guide.steps||[]).map(step=>step.source_segment??0))]
  const guideSections=indices.map((sectionIndex,position)=>{
    const source=currentSections.find(section=>section.index===sectionIndex)
    const indexedSteps=(guide.steps||[]).map((step,index)=>({step,index})).filter(item=>(item.step.source_segment??0)===sectionIndex)
    const deepDive=(guide.deep_dives||[]).find(item=>item.source_segment===sectionIndex)
    const title=source?.guide?.title||`Section ${position+1}`
    const timestamps=indexedSteps.flatMap(item=>item.step.timestamps||[])
    const start=timestamps.length?Math.min(...timestamps.map(item=>Number(item.start_seconds)||0)):0
    const end=timestamps.length?Math.max(...timestamps.map(item=>Number(item.end_seconds)||0)):start
    const pageReference=source?.transcript?.reference?.kind==='page'?source.transcript.reference:null
    const range=pageReference?(pageReference.page_start===pageReference.page_end?`Page ${pageReference.page_start}`:`Pages ${pageReference.page_start}–${pageReference.page_end}`):`${formatTime(start)}–${formatTime(end)}`
    const actions=source?`<div class="section-actions"><button class="quiet" data-delve-section="${sectionIndex}">${deepDive?'Refresh deep dive':'Delve deeper'}</button><button class="quiet" data-regenerate-section="${sectionIndex}">Regenerate</button></div>`:''
    const dive=deepDive?`<article class="deep-dive"><span class="eyebrow">DEEP DIVE · ${escapeHTML(deepDive.model||'LOCAL MODEL')}</span><h3>${escapeHTML(deepDive.title)}</h3><p>${escapeHTML(deepDive.explanation)}</p>${renderList('Key points',deepDive.key_points)}${renderList('Examples',deepDive.examples)}${renderList('Caveats',deepDive.caveats)}${renderList('Source evidence',deepDive.evidence)}</article>`:''
    return `<section class="guide-section"><div class="guide-section-head"><div><span class="eyebrow">SOURCE SECTION ${position+1}</span><h2>${escapeHTML(title)}</h2><span class="section-range">${range}</span></div>${actions}</div><div class="steps">${indexedSteps.map(item=>renderStep(item.step,item.index)).join('')}</div>${dive}</section>`
  }).join('')
  const commands = guide.commands?.length ? `<section><h2>Commands</h2>${guide.commands.map(command => `<div class="command"><pre><code>${escapeHTML(command.value)}</code></pre><p>${escapeHTML(command.description)}</p></div>`).join('')}</section>` : ''
  const shortcuts = guide.keyboard_shortcuts?.length ? `<section><h2>Keyboard shortcuts</h2><div class="shortcut-grid">${guide.keyboard_shortcuts.map(item => `<div><kbd>${escapeHTML(item.keys)}</kbd><span>${escapeHTML(item.action)}${item.context ? ` · ${escapeHTML(item.context)}` : ''}</span></div>`).join('')}</div></section>` : ''
  const generation = guide.generation?.model ? `<section class="generation"><h2>Generation details</h2><dl><div><dt>Model</dt><dd>${escapeHTML(guide.generation.model)}</dd></div><div><dt>Sections</dt><dd>${guide.generation.segment_count}</dd></div><div><dt>Duration</dt><dd>${Math.round(guide.generation.duration_milliseconds / 1000)}s</dd></div><div><dt>Tokens</dt><dd>${guide.generation.prompt_tokens} in · ${guide.generation.output_tokens} out</dd></div></dl></section>` : ''
  guideContent.innerHTML = `<article class="guide"><span class="eyebrow">${escapeHTML(guide.source_type)} GUIDE</span><h1>${escapeHTML(guide.title)}</h1><p class="lead">${escapeHTML(guide.overview)}</p>${generation}<section class="outcome"><h2>Final outcome</h2><p>${escapeHTML(guide.final_outcome)}</p></section>${renderList('Prerequisites', guide.prerequisites)}<div class="guide-sections">${guideSections}</div>${renderList('Important concepts', guide.important_concepts)}${commands}${shortcuts}${renderList('Warnings', guide.warnings)}${renderList('Common mistakes', guide.common_mistakes)}${renderList('Cheat sheet', cheatSheet)}${renderList('Appendix', guide.appendix)}<section><h2>Source references</h2>${renderSourceReferences(guide.source_references,guide.source_timestamps,guide.source_uri)}</section></article>`
}

function renderStepEditor(index) {
  const step=currentGuide.steps[index];const article=guideContent.querySelector(`[data-step-index="${index}"]`);if(!step||!article)return
  article.innerHTML=`<div class="step-number">${step.number}</div><form class="inline-editor"><label>Title<input name="title" value="${escapeHTML(step.title)}" required></label><label>Explanation<textarea name="explanation" required>${escapeHTML(step.explanation)}</textarea></label><label>Actions, one per line<textarea name="actions">${escapeHTML((step.actions||[]).join('\n'))}</textarea></label><label>Commands, one per line<textarea name="commands">${escapeHTML((step.commands||[]).join('\n'))}</textarea></label><label>Warnings, one per line<textarea name="warnings">${escapeHTML((step.warnings||[]).join('\n'))}</textarea></label><div class="editor-actions"><button type="submit">Save step</button><button type="button" class="quiet" data-cancel-step>Cancel</button></div></form>`
  article.querySelector('[data-cancel-step]').addEventListener('click',()=>renderGuide(currentGuide))
  article.querySelector('form').addEventListener('submit',event=>saveStep(event,index))
  article.querySelector('input').focus()
}

async function saveStep(event,index) {
  event.preventDefault();const form=event.currentTarget;const updated=structuredClone(currentGuide);const step=updated.steps[index];step.title=form.elements.title.value.trim();step.explanation=form.elements.explanation.value.trim();for(const field of ['actions','commands','warnings'])step[field]=form.elements[field].value.split('\n').map(value=>value.trim()).filter(Boolean);const buttons=form.querySelectorAll('button');buttons.forEach(button=>button.disabled=true);try{currentGuide=await backend().SaveGuide(updated);renderGuide(currentGuide);message.textContent=`Step ${index+1} saved.`}catch(err){message.textContent=String(err);buttons.forEach(button=>button.disabled=false)}
}

async function openGuide(id) {
  try { currentGuide = await backend().GetGuide(id); currentSections = await backend().ListGuideSections(id); renderGuide(currentGuide); library.hidden = true; reader.hidden = false; window.scrollTo({ top: 0, behavior: 'smooth' }) }
  catch (err) { message.textContent = String(err) }
}

document.querySelector('#compile-form').addEventListener('submit', async event => {
  event.preventDefault(); const button = event.currentTarget.querySelector('button'); button.disabled = true; progress.hidden = false; progress.querySelector('div').style.width = '4%'; message.textContent = 'Adding compilation to the local queue…'
  try { await backend().QueueYouTube(document.querySelector('#url').value); message.textContent = 'Compilation queued. You can keep using the library.'; await loadJobs() }
	catch (err) { message.textContent = String(err) }
	finally { button.disabled = false; if (!message.textContent.includes('queued')) progress.hidden = true }
})
document.querySelector('#refresh').addEventListener('click', loadGuides)
document.querySelector('#import-file').addEventListener('click',async event=>{const button=event.currentTarget;button.disabled=true;try{const job=await backend().SelectAndQueueFile();if(job?.id){message.textContent='File compilation queued.';progress.hidden=false;progress.querySelector('div').style.width='4%';await loadJobs()}}catch(err){message.textContent=String(err)}finally{button.disabled=false}})
guides.addEventListener('click',async event=>{const remove=event.target.closest('[data-delete-guide]');if(remove){remove.disabled=true;try{const deleted=await backend().DeleteGuide(remove.dataset.deleteGuide);if(deleted){message.textContent='Guide deleted.';await loadGuides()}else{remove.disabled=false}}catch(err){message.textContent=String(err);remove.disabled=false}return}const open=event.target.closest('[data-guide-id]');if(open)openGuide(open.dataset.guideId)})
jobs.addEventListener('click',async event=>{const retry=event.target.closest('[data-retry-job]');const cancel=event.target.closest('[data-cancel-job]');const diagnostic=event.target.closest('[data-show-job]');const button=retry||cancel||diagnostic;if(!button)return;button.disabled=true;try{if(diagnostic){const sections=await backend().GetJobSections(diagnostic.dataset.showJob);const failed=[...sections].reverse().find(section=>section.error||section.raw_response);const card=diagnostic.closest('[data-job]');card.querySelector('.job-diagnostic')?.remove();card.insertAdjacentHTML('beforeend',`<details class="job-diagnostic" open><summary>Section ${(failed?.index??0)+1} model response</summary><p>${escapeHTML(failed?.error||'No section error was recorded.')}</p>${failed?.raw_response?`<pre><code>${escapeHTML(failed.raw_response)}</code></pre>`:'<p>No raw response was returned by the model.</p>'}</details>`);button.disabled=false;return}if(retry){await backend().RetryJob(retry.dataset.retryJob);message.textContent='Failed section requeued; completed sections will be reused.'}else{await backend().CancelJob(cancel.dataset.cancelJob);message.textContent='Compilation cancelled.'}await loadJobs()}catch(err){message.textContent=String(err);button.disabled=false}})
document.querySelector('#back').addEventListener('click', () => { closeEvidence(); reader.hidden = true; library.hidden = false })
document.querySelector('#export-guide').addEventListener('click',async()=>{if(!currentGuide)return;try{const path=await backend().ExportMarkdown(currentGuide.id);if(path)message.textContent=`Exported to ${path}`}catch(err){message.textContent=String(err)}})
guideContent.addEventListener('click', event => { const link = event.target.closest('[data-source-url]'); if (link) window.runtime?.BrowserOpenURL?.(link.dataset.sourceUrl) })
function closeEvidence(){evidencePanel.hidden=true;evidenceContent.innerHTML=''}
function showEvidencePanel(content){evidenceContent.innerHTML=content;evidencePanel.hidden=false}
document.querySelector('#close-evidence').addEventListener('click',closeEvidence)
evidencePanel.addEventListener('click',event=>{if(event.target===evidencePanel)closeEvidence()})
guideContent.addEventListener('click',async event=>{const link=event.target.closest('[data-citation-id]');if(!link)return;showEvidencePanel('<p class="muted">Loading source evidence…</p>');try{const value=await backend().GetCitationEvidence(currentGuide.id,link.dataset.citationId);const page=value.chunk?.location?.physical_page;const before=value.previous?.text;const after=value.next?.text;showEvidencePanel(`<span class="eyebrow">SOURCE EVIDENCE</span><h2>${escapeHTML(value.source?.title||currentGuide.title)}</h2><p class="evidence-location">${page?`PDF page ${page}`:'Source location unavailable'}</p><div class="evidence-context">${before?`<p>${escapeHTML(before)}</p>`:''}<mark>${escapeHTML(value.chunk?.text||'No extracted text is available.')}</mark>${after?`<p>${escapeHTML(after)}</p>`:''}</div><button data-open-citation="${escapeHTML(link.dataset.citationId)}">Open full PDF</button>${page?`<p class="muted">If your system viewer does not jump automatically, navigate to PDF page ${page}.</p>`:''}`)}catch(err){showEvidencePanel(`<h2>Evidence unavailable</h2><p>${escapeHTML(err)}</p>`)}})
guideContent.addEventListener('click',event=>{const link=event.target.closest('[data-legacy-page]');if(!link)return;const page=Number(link.dataset.legacyPage)||1;showEvidencePanel(`<span class="eyebrow">SOURCE REFERENCE</span><h2>${escapeHTML(currentGuide.title)}</h2><p class="evidence-location">PDF page ${page}</p><p>This older guide has a page reference but no stored source excerpt. Recompile the PDF to create exact source evidence.</p><button data-open-legacy="${page}">Open full PDF</button><p class="muted">If your system viewer does not jump automatically, navigate to PDF page ${page}.</p>`)})
evidenceContent.addEventListener('click',async event=>{const citation=event.target.closest('[data-open-citation]');const legacy=event.target.closest('[data-open-legacy]');if(!citation&&!legacy)return;const button=citation||legacy;button.disabled=true;try{if(citation)await backend().OpenCitationSource(currentGuide.id,citation.dataset.openCitation);else await backend().OpenGuideSource(currentGuide.id,Number(legacy.dataset.openLegacy)||1)}catch(err){message.textContent=String(err);button.disabled=false}})
guideContent.addEventListener('click',event=>{const button=event.target.closest('[data-edit-step]');if(button)renderStepEditor(Number(button.dataset.editStep))})
guideContent.addEventListener('click',async event=>{const button=event.target.closest('[data-regenerate-section]');if(!button)return;const index=Number(button.dataset.regenerateSection);if(!await backend().ConfirmRegenerateSection(currentGuide.id,index))return;button.disabled=true;button.textContent='Regenerating…';try{currentGuide=await backend().RegenerateSection(currentGuide.id,index);currentSections=await backend().ListGuideSections(currentGuide.id);renderGuide(currentGuide);message.textContent=`Section ${index+1} regenerated.`}catch(err){message.textContent=String(err);button.disabled=false;button.textContent='Regenerate'}})
guideContent.addEventListener('click',async event=>{const button=event.target.closest('[data-delve-section]');if(!button)return;const index=Number(button.dataset.delveSection);button.disabled=true;const started=Date.now();const original=button.textContent;const timer=setInterval(()=>{button.textContent=`Delving… ${elapsedLabel(Date.now()-started)}`},1000);message.textContent=`Using the saved source transcript to delve into section ${index+1}…`;try{currentGuide=await backend().DelveSection(currentGuide.id,index);renderGuide(currentGuide);message.textContent=`Section ${index+1} deep dive saved.`}catch(err){message.textContent=String(err);button.disabled=false;button.textContent=original}finally{clearInterval(timer)}})
let jobRefreshTimer
window.runtime?.EventsOn?.('pipeline:progress', update => {
  message.textContent = update.message
  const percent = update.total > 0 ? Math.max(4, Math.round(update.current / update.total * 100)) : 12
  progress.hidden = false
  progress.querySelector('div').style.width = `${percent}%`
	if (update.stage === 'complete') setTimeout(() => { progress.hidden = true }, 1200)
	if (update.stage === 'failed' || update.stage === 'cancelled') progress.hidden = true
	clearTimeout(jobRefreshTimer);jobRefreshTimer=setTimeout(()=>{loadJobs();if(update.stage==='complete')loadGuides()},250)
})
loadGuides()
setInterval(updateJobTimers,1000)
