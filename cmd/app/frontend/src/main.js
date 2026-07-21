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
      <form id="compile-form"><label for="url">YouTube URL</label><div class="row"><input id="url" type="url" placeholder="https://youtube.com/watch?v=…" required><button>Compile guide</button></div></form>
      <div id="message" role="status"></div><div id="progress" class="progress" hidden><div></div></div>
    </section>
    <section id="library"><div id="job-area" hidden><div class="section-head"><h2>Compilation queue</h2></div><div id="jobs" class="jobs"></div></div><div class="section-head"><h2>Library</h2><button class="quiet" id="refresh">Refresh</button></div><div id="guides" class="guides"><p class="muted">No guides loaded.</p></div></section>
    <section id="reader" hidden><div class="reader-actions"><button class="quiet back" id="back">← Library</button><button class="quiet back" id="export-guide">Export Markdown</button></div><div id="guide-content"></div></section>
  </main>`

const message = document.querySelector('#message')
const progress = document.querySelector('#progress')
const guides = document.querySelector('#guides')
const library = document.querySelector('#library')
const reader = document.querySelector('#reader')
const guideContent = document.querySelector('#guide-content')
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

const meaningfulValues = (values = []) => values.filter(value => value != null && !['', '{}', '[]', 'null'].includes(String(value).trim()))
const renderList = (title, values = []) => { const filtered = meaningfulValues(values); return filtered.length ? `<section><h2>${title}</h2><ul>${filtered.map(value => `<li>${escapeHTML(value)}</li>`).join('')}</ul></section>` : '' }
const formatTime = seconds => { const value = Math.max(0, Math.round(Number(seconds) || 0)); return `${Math.floor(value / 60)}:${String(value % 60).padStart(2, '0')}` }
const timestampURL = (uri, seconds) => { try { const url = new URL(uri); url.searchParams.set('t', `${Math.max(0, Math.round(seconds))}s`); return url.toString() } catch { return '' } }
const renderTimestamps = (values = [], sourceURI = '') => values.length ? `<div class="timestamps">${values.map(value => { const url = timestampURL(sourceURI, value.start_seconds); const label = `${formatTime(value.start_seconds)}${value.label ? ` · ${escapeHTML(value.label)}` : ''}`; return url ? `<button data-source-url="${escapeHTML(url)}" title="Open source at ${formatTime(value.start_seconds)}">${label} ↗</button>` : `<span>${label}</span>` }).join('')}</div>` : ''

function renderGuide(guide) {
  const cheatSheet = meaningfulValues([...new Set([...(guide.cheat_sheet || []), ...(guide.keyboard_shortcuts || []).map(item => `${item.keys}${item.action ? ` — ${item.action}` : ''}`), ...(guide.commands || []).map(item => `${item.value}${item.description ? ` — ${item.description}` : ''}`), ...(guide.steps || []).flatMap(step => step.commands || [])])])
  const steps = (guide.steps || []).map((step, index) => `<article class="step" data-step-index="${index}"><div class="step-number">${step.number}</div><div><div class="step-heading"><h3>${escapeHTML(step.title)}</h3><button class="quiet step-edit" data-edit-step="${index}" title="Edit this step">✎ Edit</button></div><p>${escapeHTML(step.explanation)}</p>${renderTimestamps(step.timestamps, guide.source_uri)}${step.source_excerpt ? `<blockquote><span>Supporting transcript</span>${escapeHTML(step.source_excerpt)}</blockquote>` : ''}${renderList('Actions', step.actions)}${step.commands?.length ? `<h4>Commands</h4>${step.commands.map(command => `<pre><code>${escapeHTML(command)}</code></pre>`).join('')}` : ''}${renderList('Warnings', step.warnings)}</div></article>`).join('')
  const commands = guide.commands?.length ? `<section><h2>Commands</h2>${guide.commands.map(command => `<div class="command"><pre><code>${escapeHTML(command.value)}</code></pre><p>${escapeHTML(command.description)}</p></div>`).join('')}</section>` : ''
  const shortcuts = guide.keyboard_shortcuts?.length ? `<section><h2>Keyboard shortcuts</h2><div class="shortcut-grid">${guide.keyboard_shortcuts.map(item => `<div><kbd>${escapeHTML(item.keys)}</kbd><span>${escapeHTML(item.action)}${item.context ? ` · ${escapeHTML(item.context)}` : ''}</span></div>`).join('')}</div></section>` : ''
  const sectionActions = currentSections.length ? `<div class="section-actions">${currentSections.map(section => `<button class="quiet" data-regenerate-section="${section.index}">Regenerate section ${section.index + 1}</button>`).join('')}</div>` : ''
  const generation = guide.generation?.model ? `<section class="generation"><h2>Generation details</h2><dl><div><dt>Model</dt><dd>${escapeHTML(guide.generation.model)}</dd></div><div><dt>Sections</dt><dd>${guide.generation.segment_count}</dd></div><div><dt>Duration</dt><dd>${Math.round(guide.generation.duration_milliseconds / 1000)}s</dd></div><div><dt>Tokens</dt><dd>${guide.generation.prompt_tokens} in · ${guide.generation.output_tokens} out</dd></div></dl>${sectionActions}</section>` : ''
  guideContent.innerHTML = `<article class="guide"><span class="eyebrow">${escapeHTML(guide.source_type)} GUIDE</span><h1>${escapeHTML(guide.title)}</h1><p class="lead">${escapeHTML(guide.overview)}</p>${generation}<section class="outcome"><h2>Final outcome</h2><p>${escapeHTML(guide.final_outcome)}</p></section>${renderList('Prerequisites', guide.prerequisites)}<section><h2>Step-by-step guide</h2><div class="steps">${steps}</div></section>${renderList('Important concepts', guide.important_concepts)}${commands}${shortcuts}${renderList('Warnings', guide.warnings)}${renderList('Common mistakes', guide.common_mistakes)}${renderList('Cheat sheet', cheatSheet)}${renderList('Appendix', guide.appendix)}<section><h2>Source timestamps</h2>${renderTimestamps(guide.source_timestamps, guide.source_uri)}</section></article>`
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
guides.addEventListener('click',async event=>{const remove=event.target.closest('[data-delete-guide]');if(remove){remove.disabled=true;try{const deleted=await backend().DeleteGuide(remove.dataset.deleteGuide);if(deleted){message.textContent='Guide deleted.';await loadGuides()}else{remove.disabled=false}}catch(err){message.textContent=String(err);remove.disabled=false}return}const open=event.target.closest('[data-guide-id]');if(open)openGuide(open.dataset.guideId)})
jobs.addEventListener('click',async event=>{const retry=event.target.closest('[data-retry-job]');const cancel=event.target.closest('[data-cancel-job]');const diagnostic=event.target.closest('[data-show-job]');const button=retry||cancel||diagnostic;if(!button)return;button.disabled=true;try{if(diagnostic){const sections=await backend().GetJobSections(diagnostic.dataset.showJob);const failed=[...sections].reverse().find(section=>section.error||section.raw_response);const card=diagnostic.closest('[data-job]');card.querySelector('.job-diagnostic')?.remove();card.insertAdjacentHTML('beforeend',`<details class="job-diagnostic" open><summary>Section ${(failed?.index??0)+1} model response</summary><p>${escapeHTML(failed?.error||'No section error was recorded.')}</p>${failed?.raw_response?`<pre><code>${escapeHTML(failed.raw_response)}</code></pre>`:'<p>No raw response was returned by the model.</p>'}</details>`);button.disabled=false;return}if(retry){await backend().RetryJob(retry.dataset.retryJob);message.textContent='Failed section requeued; completed sections will be reused.'}else{await backend().CancelJob(cancel.dataset.cancelJob);message.textContent='Compilation cancelled.'}await loadJobs()}catch(err){message.textContent=String(err);button.disabled=false}})
document.querySelector('#back').addEventListener('click', () => { reader.hidden = true; library.hidden = false })
document.querySelector('#export-guide').addEventListener('click',async()=>{if(!currentGuide)return;try{const path=await backend().ExportMarkdown(currentGuide.id);if(path)message.textContent=`Exported to ${path}`}catch(err){message.textContent=String(err)}})
guideContent.addEventListener('click', event => { const link = event.target.closest('[data-source-url]'); if (link) window.runtime?.BrowserOpenURL?.(link.dataset.sourceUrl) })
guideContent.addEventListener('click',event=>{const button=event.target.closest('[data-edit-step]');if(button)renderStepEditor(Number(button.dataset.editStep))})
guideContent.addEventListener('click',async event=>{const button=event.target.closest('[data-regenerate-section]');if(!button)return;const index=Number(button.dataset.regenerateSection);const hasEdits=(currentGuide.steps||[]).some(step=>step.source_segment===index&&step.user_edited);const warning=hasEdits?' This section contains manual edits, which will be replaced. Edits in every other section will be preserved.':'';if(!window.confirm(`Regenerate section ${index+1}? This will call Ollama and replace that section's generated content.${warning}`))return;button.disabled=true;try{currentGuide=await backend().RegenerateSection(currentGuide.id,index);currentSections=await backend().ListGuideSections(currentGuide.id);renderGuide(currentGuide);message.textContent=`Section ${index+1} regenerated.`}catch(err){message.textContent=String(err);button.disabled=false}})
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
